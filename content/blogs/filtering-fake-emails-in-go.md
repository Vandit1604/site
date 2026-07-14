---
title: "How I Stopped Fake Emails From Wrecking My Signup Flow"
date: "2025-05-14"
tags: ["tech", "golang"]
---

So you build a SaaS. You put up a nice little signup form. And then, day one, some legend signs up with `hello@mailinator.com`, never opens a single email, and sits in your database forever like a ghost. Multiply that by a few hundred and your "10,000 users" number is mostly vapor. Been there.

I got tired of it and wrote a tiny Go package called [emailguard](https://github.com/Vandit1604/emailguard). One job: decide whether an email is a real, usable business address or throwaway garbage, fast enough to run *inside* a signup request. In this post I'll show you the exact decision logic, the real Go that implements it, and the false-positive traps that will bite you if you build this yourself. It's less code than you'd think.

## The whole thing is one function

Everything below is really just how one function makes up its mind:

```go
if !emailguard.IsLegitEmail(email) {
    // politely tell them to use a real email
}
```

And it makes up its mind in four checks, **cheapest first**, so it bails the instant something looks off instead of doing expensive DNS work it doesn't need to.

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

<a class="src-link" href="https://github.com/Vandit1604/emailguard/blob/main/emailguard.go" target="_blank" rel="noopener noreferrer">↗ emailguard.go</a>

Let me unpack why each check is there.

## Check 1: the shortcut for the obvious good ones

Before doing any network work, we check an allowlist of big consumer providers (Gmail, Outlook, etc.). If someone signs up with a plain `@gmail.com`, we don't need DNS gymnastics to know it's real. Return `true`, cache it, move on. This is pure "don't do expensive work you don't have to."

## Check 2: is it a known throwaway domain?

There's a wonderful open source project, [disposable-email-domains](https://github.com/disposable-email-domains/disposable-email-domains), that maintains a giant curated list of temp-mail providers. Mailinator, 10minutemail, all their cousins. Instead of me hand-maintaining that list (no thanks), emailguard clones the repo and refreshes it on a cooldown:

```go
repoURL      = "https://github.com/disposable-email-domains/disposable-email-domains.git"
pullCooldown = 30 * time.Minute
```

So the blocklist stays current without me touching it. We also check the **eTLD+1**, which is a fancy way of saying "the real registrable domain." That way `foo.mailinator.com` gets caught the same as `mailinator.com`. Sneaky subdomains don't save you.

<aside class="callout" data-label="eTLD+1">
"Effective top-level domain plus one" is what most people mean by "the domain." For <code>foo.bar.co.uk</code> it's <code>bar.co.uk</code>, not <code>co.uk</code>, because <code>co.uk</code> is effectively a TLD. Go's <a href="https://pkg.go.dev/golang.org/x/net/publicsuffix" target="_blank" rel="noopener noreferrer">publicsuffix</a> package handles this so you don't hardcode a list of every country's quirks.
</aside>

## Check 3: does the domain even accept mail?

Now we spend a network call: an MX record lookup. MX records are the DNS entries that say "here is the mail server for this domain." No MX records means no inbox exists. You literally cannot email this person. Reject.

This catches typos and made-up domains for free. Someone types `gmial.com`? No MX, gone. And the lookup gets a **1 second timeout**, because the whole point is to not make the user stare at a spinner during signup.

## Check 4: is the mail server itself sus?

This is the check I'm quietly proud of. Some services aren't on any blocklist but are obviously forwarders or masking services. The tell? Their **MX hostname gives them away.** If a domain's mail server is literally named something with a forwarding or disposable keyword in it, that's a signal. So we peek at the MX host, not just the domain, and scan it against a small keyword set. Cheap, and it catches stuff the static list misses.

## The part everyone gets wrong: false positives

One thing you learn fast building this: be paranoid about blocking real people. Rejecting Gmail because your DNS had a bad second is a fantastic way to lose a paying customer. Two defenses handle that:

- **The allowlist runs first** (check 1), so the big providers never depend on a flaky lookup.
- **Verdicts and DNS results are cached in memory**, so the same domain isn't re-checked a hundred times, and one hiccup can't flip-flop a domain's fate mid-session.

<aside class="callout callout--warn" data-label="Gotcha">
Heuristics reject some real addresses and let some junk through. That's the deal with all spam-ish filtering. Tune the aggressiveness to the cost of a wrong call: a fraud signup flow can be strict, a newsletter box should lean permissive. There is no setting that's 100% both ways.
</aside>

The nicest property of the whole thing: **zero external services.** No paid email-verification API, no third party seeing your users' emails. Just Go and DNS. Your data stays yours.

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

First call warms the blocklist and the caches. After that it's basically a map lookup plus, at most, one cached DNS call.

## TL;DR

- Four checks, cheapest first: **allowlist → disposable blocklist → MX exists → MX hostname scan.**
- Bail early, cache everything, 1s DNS timeouts, and let the big providers skip the gauntlet so you don't nuke real users.
- Blocklist auto-syncs from GitHub, so it stays current on its own.
- No paid API, no vendor seeing your users. Pure Go and DNS.

Three or four dumb-simple checks, stacked, and suddenly your signup numbers mean something again.

## Go deeper

- The repo (it's one file): [github.com/Vandit1604/emailguard](https://github.com/Vandit1604/emailguard/blob/main/emailguard.go)
- The blocklist it syncs from: [disposable-email-domains](https://github.com/disposable-email-domains/disposable-email-domains)
- [Go's publicsuffix package](https://pkg.go.dev/golang.org/x/net/publicsuffix) for eTLD+1
- [What MX records are](https://en.wikipedia.org/wiki/MX_record), if DNS mail routing is new to you

---

*Fun fact: the "check the MX hostname, not the domain" trick came from staring at reject logs and noticing the same weird mail servers over and over. Best features usually come from being annoyed at your own data.*
