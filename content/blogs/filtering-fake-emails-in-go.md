---
title: "How I Stopped Fake Emails From Wrecking My Signup Flow"
date: "2025-05-14"
tags: ["tech", "golang"]
---

So you build a SaaS. You put up a nice little signup form. And then, day one, some legend signs up with `hello@mailinator.com`, never opens a single email, and sits in your database forever like a ghost. Multiply that by a few hundred and your "10,000 users" number is mostly vapor. Been there.

I got tired of it and wrote a tiny Go package called [emailguard](https://github.com/Vandit1604/emailguard). The whole job is one question: is this a real, usable business email, or is it throwaway garbage? Let me walk you through how it decides, because the logic is way simpler than you'd think.

## The one function you actually call

The entire library is basically this:

```go
if !emailguard.IsLegitEmail(email) {
    // politely tell them to use a real email
}
```

That is it. Everything else is just how `IsLegitEmail` makes up its mind. And it makes up its mind in three quick checks, cheapest first, so it bails early the moment something looks off.

## Check 1: does the domain even accept mail?

First thing, we do an MX record lookup. MX records are the DNS entries that say "here is the mail server for this domain." No MX records means no inbox exists. You literally cannot email this person. Reject, instantly.

This catches typos and made-up domains for free. Someone types `gmial.com`? No MX, gone. And we give the lookup a **1 second timeout**, because the whole point is to be fast enough to run inside a signup request without the user staring at a spinner.

## Check 2: is it a known throwaway domain?

There is a wonderful open source project called [disposable-email-domains](https://github.com/disposable-email-domains/disposable-email-domains) that maintains a giant curated list of temp-mail providers. Mailinator, 10minutemail, all their cousins. Instead of me hand-maintaining that list (no thanks), emailguard just clones the repo and refreshes it on a cooldown:

```go
repoURL       = "https://github.com/disposable-email-domains/disposable-email-domains.git"
pullCooldown  = 30 * time.Minute
```

So the blocklist stays current without me touching it. We also check the **eTLD+1**, which is a fancy way of saying "the real registrable domain." That way `foo.mailinator.com` gets caught the same as `mailinator.com`. Sneaky subdomains don't save you.

## Check 3: is the mail server itself sus?

This is the check I'm quietly proud of. Some services aren't on any blocklist but are obviously forwarders or masking services. The trick? Their **MX hostname gives them away.** If a domain's mail server is literally named something with `disposable` or a forwarding keyword in it, that's a tell. So we peek at the MX host, not just the domain, and match against a small set of keywords. Cheap, and it catches stuff the static list misses.

## The safety net

One thing you learn fast: be paranoid about false positives. Blocking Gmail because your DNS had a bad second is a great way to lose a real customer. So there are safe defaults for the big consumer providers, and verdicts plus DNS results get **cached in memory**, so the same domain doesn't get re-checked a hundred times and a flaky lookup doesn't flip-flop.

The nice part of the whole thing: **zero external services.** No paid email-verification API, no third party seeing your users' emails. Just Go and DNS. Your data stays yours.

## TL;DR

- No MX records? Not a real inbox. Reject.
- On the disposable-email blocklist (auto-synced from GitHub)? Reject.
- Mail server hostname screams "forwarder"? Reject.
- Everything cached, 1s timeouts, safe defaults for Gmail and friends.

Three dumb-simple checks, stacked, and suddenly your signup numbers mean something again. No AI, no vendor, no monthly bill.

---

*Fun fact: the "check the MX hostname, not the domain" trick came from staring at reject logs and noticing the same weird mail servers over and over. Best features usually come from being annoyed at your own data. Go read [the source](https://github.com/Vandit1604/emailguard), it's one file.*
