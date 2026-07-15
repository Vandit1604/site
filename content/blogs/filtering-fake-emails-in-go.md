---
title: "How I Stopped Fake Emails From Wrecking My Signup Flow"
date: "2025-05-14"
description: "A tiny Go package that blocks disposable and fake signups with four cheap checks cheapest-first, an auto-syncing blocklist, and the honest false-positive traps nobody warns you about."
tags: ["tech", "golang"]
---

So you build a SaaS. You put up a nice little signup form. And then, day one, some legend signs up with `hello@mailinator.com`, never opens a single email, and sits in your database forever like a ghost. Multiply that by a few hundred and your "10,000 users" number is mostly vapor. Been there.

I got tired of it and wrote a tiny Go package called [emailguard](https://github.com/Vandit1604/emailguard). One job: decide whether an email is a real, usable business address or throwaway garbage, fast enough to run *inside* a signup request. The paid version of this is a whole industry, ZeroBounce, Kickbox, NeverBounce, all charging per lookup and all seeing every email your users type. emailguard does the 90% case locally, in one file, with nothing leaving your box. In this post I'll show you the exact decision logic, the real Go that implements it, how the blocklist keeps itself current, and the false-positive traps that will bite you if you build this yourself. It's less code than you'd think, and the interesting parts are the ones the happy path hides.

## The whole thing is one function

Everything below is really just how one function makes up its mind:

```go
if !emailguard.IsLegitEmail(email) {
    // politely tell them to use a real email
}
```

And it makes up its mind in four checks, **cheapest first**, so it bails the instant something looks off instead of doing expensive DNS work it doesn't need to. This ordering is the whole performance story: a Gmail signup never touches the network, and a made-up domain gets rejected without ever scanning a keyword list. You do the free checks first and only pay for DNS when the cheap ones can't decide.

<figure>
  <img src="/static/images/blog/07-email-funnel.svg" alt="An email flows through an allowlist shortcut, a disposable-domain blocklist, an MX-records check, and an MX-hostname keyword scan; any failing check rejects, passing all accepts.">
  <figcaption>Four checks, ordered cheapest to priciest. Any one failing rejects early. Only surviving all four gets you in.</figcaption>
</figure>

Here's the real decision core, lightly trimmed:

```go
func IsLegitEmail(email string) bool {
    // ... parse out the domain, bail on anything malformed ...

    // verdict cache hit? return it, do zero work
    if ok, hit := getVerdictCached(domain); hit {
        return ok
    }

    // 1) allow common consumer providers you explicitly permit
    if inSet(allowlist, domain) {
        setVerdictCached(domain, true)
        return true
    }

    // 2) block if the domain or its eTLD+1 is disposable
    if inSet(tempMails, domain) {
        setVerdictCached(domain, false)
        return false
    }

    // 3) require MX records (cached, 1s timeout)
    mxHosts := checkForMXCached(domain)
    if len(mxHosts) == 0 {
        setVerdictCached(domain, false)
        return false
    }

    // 4) MX intelligence: does the mail server itself look sus?
    for _, h := range mxHosts {
        for _, kw := range mxBadKeywords {
            if strings.Contains(normDomain(h), kw) {
                setVerdictCached(domain, false)
                return false
            }
        }
    }

    setVerdictCached(domain, true)
    return true
}
```

<a class="src-link" href="https://github.com/Vandit1604/emailguard/blob/c562d849758cfbe3f506f1ac3d4aedb06a14bb82/emailguard.go#L115-L176" target="_blank" rel="noopener noreferrer">↗ emailguard.go</a>

Let me unpack why each check is there, and then the machinery underneath that makes it fast and, occasionally, wrong.

## Check 1: the shortcut for the obvious good ones

Before doing any network work, we check an allowlist of big consumer providers. If someone signs up with a plain `@gmail.com`, we don't need DNS gymnastics to know it's real. Return `true`, cache it, move on. This is pure "don't do expensive work you don't have to." The list is small and deliberate:

```go
var allowlist = map[string]struct{}{
    "gmail.com": {}, "googlemail.com": {}, "outlook.com": {},
    "hotmail.com": {}, "live.com": {}, "yahoo.com": {},
    "icloud.com": {}, "proton.me": {}, "protonmail.com": {}, "fastmail.com": {},
}
```

A `map[string]struct{}` is Go's idiomatic set: the empty struct takes zero bytes, so you get O(1) membership with no wasted memory on values. One honest wrinkle: the package doc comment advertises a "configurable consumer-domain allowlist," but `allowlist` is an unexported package variable, so you can't actually configure it without editing the source. The doc promises a knob the code doesn't expose yet. Worth knowing before you build a signup flow assuming you can add your own trusted domains at runtime.

## Check 2: is it a known throwaway domain?

There's a wonderful open source project, [disposable-email-domains](https://github.com/disposable-email-domains/disposable-email-domains), that maintains a giant curated list of temp-mail providers. Mailinator, 10minutemail, all their cousins, tens of thousands of entries. Instead of me hand-maintaining that list (no thanks), emailguard treats the GitHub repo itself as the data source and keeps a local copy fresh.

Here's how that actually works, because "it auto-syncs" hides some nice engineering. On first load it does a **shallow clone** (`Depth: 1`, so you fetch the current snapshot and none of the history) into `/tmp/disposable-email-domains` using [go-git](https://github.com/go-git/go-git), a pure-Go git implementation with no dependency on the `git` binary being installed. It reads `disposable_email_blocklist.conf` line by line, skipping blanks and comment lines (`#` and `;`), and drops each domain into a set pre-sized for 40,000 entries so the map doesn't spend the load rehashing:

```go
set := make(map[string]struct{}, 40000)
sc := bufio.NewScanner(f)
for sc.Scan() {
    line := strings.TrimSpace(sc.Text())
    if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
        continue
    }
    set[normDomain(line)] = struct{}{}
}
```

<a class="src-link" href="https://github.com/Vandit1604/emailguard/blob/c562d849758cfbe3f506f1ac3d4aedb06a14bb82/emailguard.go#L222-L260" target="_blank" rel="noopener noreferrer">↗ emailguard.go · LoadTempMails</a>

Refreshes are throttled and self-healing. A `.lastpull` stamp file records when the repo was last updated; if it's newer than the 30-minute `pullCooldown`, the pull is skipped entirely. When a pull does run and fails for any reason other than "already up to date," the code doesn't limp along on a half-broken checkout, it blows the directory away and reclones from scratch:

```go
pullErr := wt.Pull(&git.PullOptions{RemoteName: "origin", Depth: 1, Force: true})
if pullErr != nil && !errors.Is(pullErr, git.NoErrAlreadyUpToDate) {
    _ = os.RemoveAll(dir)
    _, cloneErr := git.PlainClone(dir, false, &git.CloneOptions{URL: url, Depth: 1})
    // ...
}
```

<a class="src-link" href="https://github.com/Vandit1604/emailguard/blob/c562d849758cfbe3f506f1ac3d4aedb06a14bb82/emailguard.go#L262-L311" target="_blank" rel="noopener noreferrer">↗ emailguard.go · ensureRepo</a>

So the blocklist stays current without me touching it, and a corrupted local checkout heals itself on the next refresh. We also check the **eTLD+1**, which is a fancy way of saying "the real registrable domain," so `foo.mailinator.com` gets caught the same as `mailinator.com`. Sneaky subdomains don't save you.

<aside class="callout" data-label="eTLD+1">
"Effective top-level domain plus one" is what most people mean by "the domain." For <code>foo.bar.co.uk</code> it's <code>bar.co.uk</code>, not <code>co.uk</code>, because <code>co.uk</code> is effectively a TLD. Go's <a href="https://pkg.go.dev/golang.org/x/net/publicsuffix" target="_blank" rel="noopener noreferrer">publicsuffix</a> package reads Mozilla's Public Suffix List, the same list every browser uses to decide cookie scope, so you don't hardcode a list of every country's quirks.
</aside>

## Check 3: does the domain even accept mail?

Now we spend a network call: an MX record lookup. MX records are the DNS entries that say "here is the mail server for this domain." No MX records means no inbox exists. You literally cannot email this person. Reject.

The lookup uses Go's `net.DefaultResolver.LookupMX` with a `context.WithTimeout` of **one second**, because the whole point is to not make the user stare at a spinner during signup. Passing a context is the part people skip, and it matters: the default resolver will happily wait the OS DNS timeout (often 5 to 30 seconds) if a nameserver black-holes your query, and a signup form cannot hang that long. One second is the deal: answer fast or don't answer.

```go
ctx, cancel := context.WithTimeout(context.Background(), mxTimeout)
defer cancel()
recs, err := net.DefaultResolver.LookupMX(ctx, domain)
```

This catches typos and made-up domains for free. Someone types `gmial.com`? No MX, gone.

## Check 4: is the mail server itself sus?

This is the check I'm quietly proud of, and also the one with the sharpest edges. Some services aren't on any blocklist but are obviously forwarders or masking services. The tell? Their **MX hostname gives them away.** If a domain's mail server is literally named something with a forwarding or disposable keyword in it, that's a signal. So we peek at the MX host, not just the domain, and scan it against a small keyword set:

```go
var mxBadKeywords = []string{
    "mask", "alias", "relay", "forward",
    "tempmail", "mailinator", "disposable", "burner",
}
```

<a class="src-link" href="https://github.com/Vandit1604/emailguard/blob/c562d849758cfbe3f506f1ac3d4aedb06a14bb82/emailguard.go#L72-L81" target="_blank" rel="noopener noreferrer">↗ emailguard.go</a>

There's a second, smarter half most people would miss: for each MX host, emailguard also computes *its* registrable domain and checks that against the disposable set. So a domain that isn't itself blocklisted but routes its mail through a known temp-mail provider still gets caught. The masking service can hide the front door; it can't hide where the mail actually lands.

Now the honest part. That keyword scan is a plain `strings.Contains` substring match, and substrings are blunt. "relay" and "forward" are extremely common in *legitimate* corporate mail infrastructure. Plenty of real companies route through hosts with "relay" in the name. So this check has real false-positive risk baked in, and it's the first knob I'd loosen if I saw good users getting bounced. It's a heuristic that earns its keep on the junk it catches, but it is not free, and pretending it is would be the exact kind of dishonesty this whole post is trying to avoid.

## The caches, and the honest cost of caching a "no"

Two caches make repeated checks nearly free, and they're where the concurrency lives. Every verdict and every MX result is stored in a TTL map guarded by a single `sync.RWMutex`. Reads take the read lock (so a hundred concurrent signups can all hit the cache at once), writes take the full lock. The MX cache does one thing worth copying: it never hands a caller the slice it stored. It returns a defensive copy under the lock.

```go
if e, ok := mxCache[domain]; ok && time.Now().Before(e.exp) {
    hostsCopy := append([]string(nil), e.hosts...)
    cacheMu.RUnlock()
    return hostsCopy
}
```

<a class="src-link" href="https://github.com/Vandit1604/emailguard/blob/c562d849758cfbe3f506f1ac3d4aedb06a14bb82/emailguard.go#L180-L195" target="_blank" rel="noopener noreferrer">↗ emailguard.go · checkForMXCached</a>

If it returned `e.hosts` directly, the caller and the cache would share the same underlying array, and any code that later mutated the returned slice would silently corrupt the cache for everyone. `append([]string(nil), ...)` is the idiomatic Go way to say "give me my own copy." The blocklist load is guarded by a `sync.Once`, so even if fifty goroutines call `IsLegitEmail` at the same instant on a cold process, the 40,000-domain set is built exactly once and the rest wait.

Here's the cost nobody mentions: **negative verdicts get cached too, for the full five minutes.** If a domain's DNS hiccuped for that one-second window and returned no MX, that domain is now marked dead for five minutes. A real user on a real company domain who happened to sign up during a transient DNS blip gets rejected, retries thirty seconds later, and gets rejected *again* from the cache. Caching a "yes" saves work. Caching a "no" turns a momentary network glitch into a five-minute outage for one specific user, and they will never tell you, they'll just leave. If I were hardening this for real money, I'd cache positive verdicts long and negative verdicts short, or not at all.

## What it deliberately doesn't do

The next tier up is SMTP probing: actually connecting to the MX server and issuing `RCPT TO` to see whether the specific mailbox exists, not just the domain. Paid verifiers lean on this. emailguard skips it on purpose, and the reasons are worth stating because "just check harder" sounds obviously better until you try it. SMTP probing is slow (a full connection handshake per check, far too slow for an inline signup), unreliable (most serious mail servers accept every `RCPT TO` and reject later to avoid leaking which addresses exist, a technique called catch-all), and it gets your IP onto spam blocklists fast, because dialing strangers' mail servers to interrogate them for addresses is exactly what spammers do. So emailguard stops at "can this domain receive mail, and does it look legitimate," which is the line where the checks stay cheap, local, and safe.

## The part everyone gets wrong: false positives

One thing you learn fast building this: be paranoid about blocking real people. Rejecting a paying customer because your DNS had a bad second is a fantastic way to lose revenue you'll never even know you lost. The defenses:

- **The allowlist runs first**, so the big providers never depend on a flaky lookup.
- **Positive verdicts are cached**, so a domain that passed once isn't re-litigated a hundred times.
- **The keyword scan is the thing to tune down**, because substring matching on "relay"/"forward" is where legitimate infra gets caught.
- **Short-TTL your negatives**, per the caching cost above.

<aside class="callout callout--warn" data-label="Gotcha">
Heuristics reject some real addresses and let some junk through. That's the deal with all spam-ish filtering. Tune the aggressiveness to the cost of a wrong call: a fraud signup flow can be strict, a newsletter box should lean permissive. There is no setting that's 100% both ways.
</aside>

The nicest property of the whole thing: **zero external services.** No paid email-verification API, no third party seeing your users' emails. Just Go, DNS, and a public blocklist. Your data stays yours.

## Run it

```bash
go get github.com/Vandit1604/emailguard
```

```go
import "github.com/Vandit1604/emailguard"

if emailguard.IsLegitEmail(input) {
    // let them in
}
```

One thing to know: the package does its first blocklist clone in an `init()` function, so *importing* emailguard triggers a network git clone at process startup, not on the first request. That keeps the first real signup fast, but it means your binary reaches out to GitHub the moment the package loads. Good to know if you run in a locked-down network or a cold-start-sensitive serverless environment.

## TL;DR

- Four checks, cheapest first: **allowlist → disposable blocklist → MX exists → MX hostname scan.** Bail early, only pay for DNS when the free checks can't decide.
- The blocklist **auto-syncs from GitHub** via a pure-Go shallow clone, throttled by a `.lastpull` stamp, and reclones itself if a pull breaks.
- MX lookups run through a **1-second context timeout** so a black-holed nameserver can't hang your signup form.
- Concurrency is handled with a **`sync.RWMutex` + `sync.Once`**, and the MX cache returns **defensive slice copies** so callers can't corrupt it.
- Honest traps: the keyword scan is a **blunt substring match** ("relay"/"forward" hit real infra), and **caching a negative verdict for 5 minutes** turns one DNS blip into a 5-minute rejection for that user.
- No paid API, no vendor seeing your users. Pure Go and DNS.

## Go deeper

- The repo (it's one file): [github.com/Vandit1604/emailguard](https://github.com/Vandit1604/emailguard/blob/main/emailguard.go)
- The blocklist it syncs from: [disposable-email-domains](https://github.com/disposable-email-domains/disposable-email-domains)
- [go-git](https://github.com/go-git/go-git), the pure-Go git implementation that does the cloning
- [Go's publicsuffix package](https://pkg.go.dev/golang.org/x/net/publicsuffix) and [Mozilla's Public Suffix List](https://publicsuffix.org/) for eTLD+1
- [Why SMTP `RCPT TO` verification is unreliable](https://en.wikipedia.org/wiki/Callback_verification), the check emailguard deliberately skips

---

*Fun fact: the "check the MX hostname, not the domain" trick came from staring at reject logs and noticing the same weird mail servers over and over. Best features usually come from being annoyed at your own data.*
