---
title: "Service-to-Service Auth From Scratch, and the Five Things That Bite You"
date: "2026-04-15"
description: "How internal microservices actually prove who they are to each other: OAuth2 client-credentials with Keycloak and OIDC, JWT validation via JWKS, and the clock-skew, key-rotation, and audience traps nobody warns you about."
tags: ["infra", "security"]
---

When a user logs in, auth is a solved-feeling problem: they type a password, you check it, you hand them a session. But inside a system of microservices, most traffic has no user at all. The billing service calls the accounts service. A scanner calls the reporting API. There's no human, no password to type, and yet the callee still needs to answer one question before it does anything: *is the thing calling me actually who it claims to be, and is it allowed to ask this?*

I built the internal service-to-service auth for a platform from scratch, using OAuth2 client-credentials with Keycloak and OIDC. The mechanism is genuinely simple once it clicks. What took the time, and what this post is really about, is the handful of validation details that are quietly load-bearing, the ones where getting it 90% right means you have no security at all. Let me build up the right shape, then walk the five traps that got me or nearly did.

## The two tempting wrong answers

Before the right tool, the two things people reach for first.

**Shared API keys.** Give every service a long-lived secret string, check it on the other end. It works on day one and rots forever after. The keys are long-lived, so a leak is permanent until someone notices and rotates by hand. They're usually coarse (one key, full access), they end up copy-pasted into env files and config maps and occasionally git history, and rotating them means a coordinated redeploy across everything that holds one. It's auth you'll be apologizing for later.

**Mutual TLS.** The heavyweight, genuinely-strong option: every service gets a certificate, and they authenticate each other at the TLS layer. mTLS is excellent, and if you run a service mesh like Istio or Linkerd you may already have it for free. But rolling it yourself means running a PKI: issuing certs, distributing them, rotating them before they expire, revoking them when a service is compromised. That's a real operational surface, and the identity frameworks built to tame it ([SPIFFE / SPIRE](https://spiffe.io/)) are their own projects to learn. It's the right answer at a certain scale. It was more machinery than the problem in front of me needed.

## The fit: OAuth2 client-credentials

There's a grant type designed for exactly this, no-user, machine-to-machine auth: the **client-credentials grant** from OAuth2. The shape is clean. Every service is a registered *client* with a `client_id` and a `client_secret`. When service A wants to call service B, it doesn't send its secret to B. Instead:

1. Service A POSTs its `client_id` + `client_secret` to the **token endpoint** of an authorization server (I used [Keycloak](https://www.keycloak.org/), the open-source identity server) and asks for a token.
2. Keycloak authenticates A and issues a short-lived **JWT access token**.
3. Service A calls service B with `Authorization: Bearer <token>`.
4. Service B **validates the token** and, if it's good, serves the request. B never sees A's secret, and B never has to call Keycloak on the hot path.

That last point is the elegant part, and it's what OIDC buys you. [OpenID Connect](https://openid.net/developers/how-connect-works/) sits on top of OAuth2 and gives the authorization server a **discovery document** (`/.well-known/openid-configuration`) and a **JWKS endpoint** that publishes the *public* keys Keycloak signs tokens with. So service B can verify a token's signature locally, using a public key it fetched once, without a network round-trip to Keycloak per request. The secret proves identity to the issuer; the signature proves the token's authenticity to everyone else.

<aside class="callout callout--tip" data-label="Why this shape">
The secret never travels to the service you're calling, only to the issuer. Every callee validates a signed token offline against a public key. That means a compromised service B can't turn around and impersonate A, because B only ever held A's public-key-verified token, never A's secret. Shared API keys fail exactly this test: whoever you present the key to can now replay it as you.
</aside>

## Provisioning the clients as code

The clients don't get clicked into a Keycloak admin UI by hand, which is how this kind of thing usually rots into tribal knowledge. Every service that needs an identity is an entry in a **map in Terraform**, and the Keycloak provider turns that map into a set of **service account clients** (Keycloak's term for a confidential client that authenticates as *itself* via the client-credentials grant, no user attached). One `terraform apply` and every client in the map exists, each with its `client_id`, its secret, and its assigned role. Need a new service? Add a line to the map, apply, done.

This matters more than it looks. Because the clients are declared as data, "which services are allowed to authenticate, and what role does each hold" is answerable by reading a file in a pull request, not by spelunking someone's browser history. Drift, a client that exists in Keycloak but not in the map, or the reverse, shows up as a Terraform diff. And each client carrying its **own role** is what turns authentication into authorization: the token proves not just "I'm a registered client" but "I'm *this* client, with *this* role," so the callee can check the request came from a client it actually expects, holding a role that's actually allowed to make this call.

## Cookies for humans, tokens for machines

There's a split in the system worth calling out, because it's where the token path stops being optional. User-facing traffic from the frontend authenticates with **cookies**, the ordinary browser session flow, and by default that's what everything used. Internal service-to-service calls can't lean on that, and the **cron and worker jobs** made the reason concrete: a scheduled job has no browser, no session, no cookie jar. There's no human in the loop and nothing to hold a cookie. So internal calls, and the background workers especially, use the **Bearer token** path instead: the client fetches a token from Keycloak with its own credentials and presents it on the call.

So the platform runs two authentication modes on purpose. Cookies for anything that originates from a logged-in human in a browser; client-credential tokens for anything a service or a cron job kicks off on its own behalf. The rule for which applies is just "is there a browser and a human behind this request." The moment the answer is no, an internal API hop, a nightly worker, you're on the token path, because a cookie needs a browser to live in and these callers don't have one.

## What the callee actually has to check

Here's where the real work lives. "Validate the token" is five checks, and skipping any one of them silently removes a security guarantee while the happy path keeps working perfectly. This is the part that took the care.

1. **Signature**, against the right public key. Fetch Keycloak's public keys from the JWKS endpoint and verify the JWT's signature. The token header carries a `kid` (key ID) telling you which key signed it; you look that key up in the JWKS.
2. **Expiry** (`exp`), so an old token can't be replayed forever. JWTs are stateless, so this is your main revocation mechanism, which is why tokens are short-lived.
3. **Issuer** (`iss`), so you only trust tokens minted by *your* Keycloak realm, not some other issuer that happens to produce valid-looking JWTs.
4. **Audience** (`aud`), so a token minted for one service can't be replayed against another. This is the one people skip, and it's the most dangerous omission. More below.
5. **Algorithm**, pinned. Verify with the algorithm *you* expect (RS256), never the one the token's own header claims. Trusting the header is a classic JWT break.

In code, the five checks live in one place and it's worth seeing them together, because the security is entirely in this function being complete:

```go
token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
    // 5) pin the algorithm, never trust t.Header["alg"]
    if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
        return nil, fmt.Errorf("unexpected signing method")
    }
    // 1) find the right public key by kid, from the cached JWKS
    kid, _ := t.Header["kid"].(string)
    return jwks.keyFor(kid) // refetches JWKS on an unknown kid
},
    jwt.WithIssuer(expectedIssuer),        // 3) iss
    jwt.WithAudience(thisServiceName),     // 4) aud
    jwt.WithValidMethods([]string{"RS256"}),
    jwt.WithLeeway(5*time.Second),         // 1 (trap) clock skew
)
// exp is checked by default (2) once the signature verifies
if err != nil || !token.Valid {
    http.Error(w, "unauthorized", http.StatusUnauthorized)
    return
}
```

Every clause maps to a guarantee. Drop `WithAudience` and any valid realm token gets in. Drop `WithValidMethods` and the algorithm-confusion door opens. Drop `WithLeeway` and clock skew hands you flaky 401s. The happy path looks identical with or without each of these, which is exactly why they're easy to leave out and dangerous to.

## The five things that bite

Everything above is the textbook. Here's what the textbook doesn't stress, in the order they hurt.

**1. Clock skew.** `exp` and `nbf` (not-before) are timestamps, and they're compared against the *callee's* clock. If two machines' clocks drift a few seconds apart, a freshly-issued token can look expired, or not-yet-valid, and you get intermittent 401s that vanish when you retry, which is the worst kind of bug to reproduce. Allow a small leeway (a few seconds) on the time checks, and actually run NTP on your hosts. This one cost me real debugging time before the pattern clicked.

**2. JWKS caching and key rotation.** You must cache the JWKS public keys, or every single inbound request triggers a fetch to Keycloak, adding latency and making Keycloak a hot-path dependency. But if you cache them *forever*, the day Keycloak rotates its signing key, every token signed by the new key fails validation because you're checking against the old one. The correct behavior is to cache by `kid` and, on seeing a token with an unknown `kid`, refetch the JWKS once before rejecting. Cache, but be ready to refresh on a cache miss.

<aside class="callout callout--warn" data-label="The subtle one">
Audience validation is the check that feels optional and isn't. If service B doesn't verify that <code>aud</code> names service B, then any valid token from your realm is accepted, including one that service C legitimately obtained for a totally different purpose. Now a low-privilege service holding a valid token can call a high-privilege one. That's the confused-deputy problem, and skipping <code>aud</code> is how you build it by accident. Scope each token to its intended audience and check it on arrival.
</aside>

**3. Skipping audience** (the callout above). Worth its own number because it's the difference between "services are authenticated" and "services are authenticated *and authorized to talk to this specific callee*." Without it, authentication becomes a network-wide skeleton key.

**4. Making the issuer a single point of failure.** If every request needs a fresh token and Keycloak is down, everything stops. The fix falls out of doing token caching *on the client side* correctly: a service caches its access token and reuses it until shortly before expiry, so a brief Keycloak outage doesn't halt traffic, existing tokens keep working for their lifetime. Fetch a new token proactively before the old one expires, not reactively after the first 401, and add jitter so all your services don't stampede the token endpoint at the same second when their tokens expire together.

**5. Token lifetime is a real tradeoff, not a default.** Short tokens mean a leaked token is useless fast and revocation is effectively automatic, but you fetch tokens more often. Long tokens mean fewer fetches but a stolen token is valid longer, and because JWTs are stateless you can't easily revoke one before it expires. Keep them short (minutes, not hours), lean on caching to absorb the fetch cost, and accept that "instant revocation" isn't something stateless JWTs give you. If you truly need instant kill, you're back to introspection (asking Keycloak per request) and its latency cost, which is the thing the whole design was avoiding.

## TL;DR

- Internal services have **no user and no password**, so user-auth patterns don't apply. The question is machine identity.
- **Shared API keys** rot and leak; **mTLS** is strong but means running a PKI (or a service mesh / SPIFFE). Fine at scale, heavy to hand-roll.
- **OAuth2 client-credentials + Keycloak + OIDC** fits: each service is a client with a secret, gets a **short-lived JWT** from the token endpoint, and callees validate it **offline via JWKS** with no per-request call to the issuer.
- **Provision the clients as code**: a map in Terraform, and the Keycloak provider creates every **service account client** on apply, each with its own role. The auth topology is reviewable in a PR and drift shows up as a diff.
- **Two auth modes on purpose**: cookies for browser/human traffic, **Bearer tokens for internal service-to-service calls and cron/worker jobs**, which have no browser and so can't use a cookie.
- Validation is five checks: **signature (right `kid`), `exp`, `iss`, `aud`, and a pinned algorithm.** Skipping any one silently deletes a guarantee.
- The five traps: **clock skew** (allow leeway, run NTP), **JWKS rotation** (cache by `kid`, refetch on miss), **skipping `aud`** (confused deputy), **the issuer as a SPOF** (client-side token caching + jitter), and **token lifetime** (short, because stateless JWTs can't be revoked early).

The satisfying thing about this design is that the security lives in math and short-lived signed tokens, not in a shared secret you're praying nobody leaked. But it only holds if the callee does all five checks. Auth that validates the signature and calls it a day is a screen door: it looks like a barrier and stops nothing that's trying.

## Go deeper

- [Keycloak](https://www.keycloak.org/), the open-source identity server that plays the authorization-server role here
- [The client-credentials grant](https://datatracker.ietf.org/doc/html/rfc6749#section-4.4) in RFC 6749, the OAuth2 spec, and [how OIDC works](https://openid.net/developers/how-connect-works/) on top of it
- [JWT](https://datatracker.ietf.org/doc/html/rfc7519) and [JWKS](https://datatracker.ietf.org/doc/html/rfc7517), the token and the public-key-set formats
- [SPIFFE / SPIRE](https://spiffe.io/), the identity framework if you go the mTLS route instead
- [Auth0's client-credentials explainer](https://auth0.com/docs/get-started/authentication-and-authorization-flow/client-credentials-flow), a clean walkthrough of the same flow with a managed provider

---

*Fun fact: the bug that taught me the most here was intermittent 401s in staging that no one could reproduce. It was clock skew of about four seconds between two nodes. Half of "distributed systems security" turns out to be "your clocks disagree and now nobody trusts anybody."*
