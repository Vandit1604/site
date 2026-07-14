---
title: "A Load Balancer Is 200 Lines of Go and Three Bugs You Won't Notice"
date: "2024-07-16"
description: "Building a round-robin HTTP load balancer in ~200 lines of Go, and the three quiet bugs that reveal what real load balancing actually is."
tags: ["tech", "golang"]
---

"Load balancer" is one of those phrases that sounds like it should live in a data center behind a locked door. I built a tiny one, [go-lb](https://github.com/Vandit1604/go-lb), because I wanted to actually understand something I'd been hand-waving for years: *how does a single incoming request find its way to one specific server out of many?* You hit one address, but behind it are five identical backends. Something has to sit in the middle and decide. I wanted to be that something, in code, so the routing stopped being magic.

The answer, it turns out, fits in a couple hundred lines of Go. A load balancer takes incoming HTTP requests and spreads them across a pool of backend servers. That's the job. That's the whole job.

But this post has a twist, and it's the honest kind. Writing the happy path was easy. The interesting part is the three bugs sitting quietly in my own code, the kind that pass every casual test and would absolutely bite you in production. Let me build the thing and then show you where it lies to you, because *that* is the actual lesson about load balancers.

## The core: round-robin in one function

The simplest way to spread load is round-robin: hand request 1 to backend A, request 2 to backend B, request 3 to backend C, then back to A. Just rotate through the list. Here's the real rotation from go-lb:

```go
func (s *ServerPool) getNextBackend() *Backend {
    s.mux.Lock()
    defer s.mux.Unlock()
    if len(s.backends) == 0 {
        return nil
    }
    s.current = (s.current + 1) % len(s.backends)
    return s.backends[s.current]
}
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/serverpool.go#L30-L41" target="_blank" rel="noopener noreferrer">↗ serverpool.go</a>

The `% len(s.backends)` wraps the index back to zero, so it cycles forever. The `sync.Mutex` matters more than it looks: a load balancer is hammered by many requests at once, so two goroutines could try to bump `current` at the same instant and both grab the same backend, or worse, index out of bounds. The lock makes the rotation atomic. Miss that and you've got a race condition that only shows up under load, which is the worst kind.

Then the actual forwarding is almost free, because Go's standard library already has a reverse proxy:

```go
rp := httputil.NewSingleHostReverseProxy(url)
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/main.go#L11" target="_blank" rel="noopener noreferrer">↗ main.go</a>

`httputil.ReverseProxy` handles rewriting the request, streaming the response back, all of it. So the "balancer" is really just: pick a backend, hand the request to its proxy. Pick, forward, repeat.

<aside class="callout callout--tip" data-label="The reframe">
A load balancer isn't a mysterious appliance. It's a loop that picks a backend and a reverse proxy that forwards to it. Go ships the hard part (the proxy) in the standard library. What's left, the picking, is the part you actually design, and it's where all the interesting decisions and bugs live.
</aside>

## Bug one: it skips the first backend

Look at that rotation again. `s.current` starts at `0`, and the function does `(s.current + 1) % len` *before* returning the backend. So the very first request doesn't go to `backends[0]`. It goes to `backends[1]`. Backend zero doesn't get a request until the counter wraps all the way around.

With three backends it's a small unfairness. But it's a real off-by-one, and it's the perfect example of a bug that passes every test you'd casually write. You fire ten requests, see them spread across all three servers, go "yep, round-robin works," and ship it. The skipped-first-backend detail only matters at the edges (a single request, a pool of two, a health check that always hits the "wrong" one). Increment-then-use versus use-then-increment is a one-character difference that changes behavior, and you will not see it unless you're looking.

## Bug two: the health check that never runs

Here's the one that actually matters. A load balancer's real value isn't spreading load, it's *not sending traffic to a dead server*. If backend B falls over, requests routed to B should stop. go-lb even has the code for it. There's a function that TCP-dials a backend to see if it's alive:

```go
func IsBackendAlive(ctx context.Context, url *url.URL) bool {
    var d net.Dialer
    conn, err := d.DialContext(ctx, "tcp", url.Host)
    if err != nil {
        return false
    }
    _ = conn.Close()
    return true
}
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/serverpool.go#L54-L62" target="_blank" rel="noopener noreferrer">↗ serverpool.go</a>

Looks great. There's a `SetAlive` setter, an `IsAlive` getter, and the routing code even checks `IsAlive()` before forwarding, correctly returning a 503 if every backend is down. The whole liveness system is wired up... except **nothing ever calls `IsBackendAlive` or `SetAlive`.** No background loop pings the servers. So every backend is marked alive when it's created and stays "alive" forever, even after it's been on fire for an hour. The safety net is fully built and never hung up.

<aside class="callout callout--warn" data-label="This is the real gotcha">
This is the single most important thing a real load balancer does, and it's the easiest to leave half-finished, because a demo with three healthy backends works perfectly without it. The failure only appears when a backend actually dies, which never happens on your laptop and always happens at 3am in prod. "The code exists" and "the code runs" are different claims. Grep for the callers.
</aside>

## Bug three: the counter that's lying about its own name

go-lb has a field called `aliveConnections` and a getter `GetActiveConnections`. Sounds like it tracks how many requests a backend is currently handling, which is exactly what you'd need for a smarter "least-connections" strategy (send the next request to whoever's least busy). Here's how it's updated:

```go
func (b *Backend) Serve(rw http.ResponseWriter, req *http.Request) {
    b.reverseProxy.ServeHTTP(rw, req)
    b.mux.Lock()
    b.aliveConnections++
    b.mux.Unlock()
}
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/backend.go#L59-L64" target="_blank" rel="noopener noreferrer">↗ backend.go</a>

Spot it? The counter goes up *after* the request finishes, and it never comes back down. There's no matching `aliveConnections--`. So it's not "active connections" at all, it's a lifetime total of requests ever served. A number that only grows. To actually track active connections you'd increment *before* `ServeHTTP` and decrement *after*, in a `defer`. As written, the field's name is a promise the code doesn't keep, and any least-connections logic built on it would be quietly wrong.

## Why I'm showing you my own bugs

Because this is the real education. Anyone can write the round-robin. What building go-lb actually taught me is that a load balancer is defined by its failure handling, and failure handling is exactly the stuff that's invisible when everything's healthy. The three bugs aren't embarrassing accidents, they're a map of where the real difficulty lives: concurrency (the mutex, the off-by-one), liveness (the check nobody calls), and honest state (the counter that grows forever).

Production load balancers, HAProxy, Envoy, nginx, are mostly *that hard part*: health checks, connection draining, timeouts, retries, circuit breaking. The routing loop is the 10%. The other 90% is what happens when a backend misbehaves. Writing the toy version is what made that finally obvious to me.

## TL;DR

- A load balancer's core is a **rotation function plus `httputil.ReverseProxy`**. Go gives you the hard forwarding part for free.
- Lock the rotation with a **mutex**, or you get a race under load that never shows up in testing.
- **Bug 1:** increment-then-index skips `backends[0]` on the first request. A one-character off-by-one.
- **Bug 2:** the health-check code exists but **nothing calls it**, so dead backends keep getting traffic. This is the whole point of a load balancer, and the easiest thing to leave unwired.
- **Bug 3:** `aliveConnections` counts up after each request and never down, so it's a lifetime total, not active connections. The name lies.
- Real load balancers are 90% failure handling. The routing loop is the easy 10%.

## Go deeper

- The whole thing, ~233 lines: [github.com/Vandit1604/go-lb](https://github.com/Vandit1604/go-lb)
- Go's [`httputil.ReverseProxy`](https://pkg.go.dev/net/http/httputil#ReverseProxy), the standard-library workhorse
- [How HAProxy does health checks](https://www.haproxy.com/documentation/haproxy-configuration-tutorials/reliability/health-checks/), for what the grown-up version of bug two looks like

---

*Fun fact: I found all three of these bugs by reading my own code back to write this post, not while writing it. Nothing teaches you a system like having to explain it out loud to a stranger.*
