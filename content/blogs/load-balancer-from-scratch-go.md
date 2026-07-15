---
title: "A Load Balancer Is 200 Lines of Go, and Five Bugs You Won't See Coming"
date: "2024-07-16"
description: "Building a round-robin HTTP load balancer in ~200 lines of Go, and the five quiet bugs (one of which go vet screams about) that reveal what real load balancing actually is."
tags: ["tech", "golang"]
---

"Load balancer" is one of those phrases that sounds like it should live in a data center behind a locked door. You have used a dozen of them without thinking: every time nginx sits in front of your app, every time a Kubernetes `Service` fans traffic across pods, every time an AWS ALB spreads requests, something in the middle is quietly picking a backend for you. I built a tiny one, [go-lb](https://github.com/Vandit1604/go-lb), because I wanted to stop hand-waving the one question underneath all of that: *how does a single incoming request find its way to one specific server out of many?* You hit one address, but behind it are five identical backends. Something has to sit in the middle and decide. I wanted to be that something, in code, so the routing stopped being magic.

The answer, it turns out, fits in a couple hundred lines of Go. A load balancer takes incoming HTTP requests and spreads them across a pool of backend servers. That's the job. That's the whole job.

But this post has a twist, and it's the honest kind. Writing the happy path was easy. The interesting part is the bugs sitting quietly in my own code, the kind that pass every casual test and would absolutely bite you in production. There are five of them. Four you'd never notice by running the thing. One is loud enough that Go's own tooling flags it, and I shipped it anyway. Let me build the thing and then show you where it lies to you, because *that* is the actual lesson about load balancers.

## What actually sits in the middle

Before the code, the shape. Strip away the marketing and every layer-7 load balancer is two pieces: a **picker** that chooses which backend handles the next request, and a **forwarder** that relays the request to it and streams the response back. nginx calls the pool an `upstream` block. HAProxy calls it a `backend`. Envoy calls it a `cluster`. Kubernetes hides it inside `kube-proxy` and calls the front door a `Service`. Different words, same two pieces.

The picker is where the named strategies live: round-robin, least-connections, IP-hash, weighted. The forwarder is mostly plumbing: rewrite the request headers, open a connection, copy bytes both ways. What surprised me building go-lb is how lopsided the difficulty is. The forwarder is nearly free in Go. The picker is trivial to write and *very* easy to write subtly wrong. Almost every bug below lives in the picker or in the state it leans on.

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

The `% len(s.backends)` wraps the index back to zero, so it cycles forever. The `sync.Mutex` matters more than it looks: a load balancer is hammered by many requests at once, so two goroutines could try to bump `current` at the same instant and both grab the same backend, or worse, index out of bounds. The lock makes the rotation atomic. Miss that and you've got a race condition that only shows up under load, which is the worst kind. (nginx sidesteps this entirely by keeping its round-robin counter per-worker-process rather than shared, so there's nothing to lock. Different concurrency model, same problem to solve.)

Then the actual forwarding is almost free, because Go's standard library already has a reverse proxy:

```go
rp := httputil.NewSingleHostReverseProxy(url)
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/main.go#L11" target="_blank" rel="noopener noreferrer">↗ main.go</a>

That one line is doing a lot. `NewSingleHostReverseProxy` returns a `*httputil.ReverseProxy` whose `Director` function rewrites each incoming request: it swaps in the backend's scheme and host, joins the paths, and leaves the rest alone. When a request arrives, `ServeHTTP` runs the Director, dials the backend (reusing pooled connections through the default `http.Transport`), and then *streams* the response back to the client as it arrives rather than buffering the whole thing in memory. That streaming detail is why a reverse proxy can relay a 2GB download without using 2GB of RAM. Writing that correctly, with trailers, flushing, and hop-by-hop header stripping handled to spec, is the genuinely hard part of a proxy, and Go hands it to you for free.

So the "balancer" is really just: pick a backend, hand the request to its proxy. Pick, forward, repeat.

<aside class="callout callout--tip" data-label="The reframe">
A load balancer isn't a mysterious appliance. It's a loop that picks a backend and a reverse proxy that forwards to it. Go ships the hard part (the proxy) in the standard library. What's left, the picking, is the part you actually design, and it's where all the interesting decisions and bugs live.
</aside>

## Bug one: it skips the first backend

Look at that rotation again. `s.current` starts at `0`, and the function does `(s.current + 1) % len` *before* returning the backend. So the very first request doesn't go to `backends[0]`. It goes to `backends[1]`. Backend zero doesn't get a request until the counter wraps all the way around.

With three backends it's a small unfairness. But it's a real off-by-one, and it's the perfect example of a bug that passes every test you'd casually write. You fire ten requests, see them spread across all three servers, go "yep, round-robin works," and ship it. The skipped-first-backend detail only matters at the edges: a single request, a pool of two, a health check that always hits the "wrong" one, a canary you just added at index 0 that quietly gets no traffic. Increment-then-use versus use-then-increment is a one-character difference that changes behavior, and you will not see it unless you're looking. The fix is to read `backends[s.current]` first and increment afterward, so the sequence starts at 0.

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

Looks great. There's a `SetAlive` setter, an `IsAlive` getter, and the routing code even walks the pool with `GetNextValidServer`, checking `IsAlive()` on each candidate and correctly returning a 503 if every backend is down. The whole liveness system is wired up... except **nothing ever calls `IsBackendAlive` or `SetAlive`.** No background loop pings the servers. So every backend is marked alive when it's created, in `NewBackend`, and stays "alive" forever, even after it's been on fire for an hour. The safety net is fully built and never hung up.

<aside class="callout callout--warn" data-label="This is the real gotcha">
This is the single most important thing a real load balancer does, and it's the easiest to leave half-finished, because a demo with three healthy backends works perfectly without it. The failure only appears when a backend actually dies, which never happens on your laptop and always happens at 3am in prod. "The code exists" and "the code runs" are different claims. Grep for the callers.
</aside>

The grown-up version is a whole subsystem. HAProxy runs **active** checks on a timer (a TCP connect or an HTTP `GET /health` every few seconds) and, separately, **passive** checks that watch real traffic and eject a backend after N consecutive failures. Envoy calls the passive version *outlier detection* and will pull a host out of rotation the moment it starts returning 5xx, then slowly let it back in. Kubernetes splits the concept in two: a `readinessProbe` decides whether a pod gets traffic at all, and a `livenessProbe` decides whether to restart it. go-lb has exactly the dialing primitive those systems are built on, and never schedules it. That gap *is* the difference between a demo and a load balancer.

## Bug three: the counter that's lying about its own name

go-lb has a field called `aliveConnections` and a getter `GetActiveConnections`. Sounds like it tracks how many requests a backend is currently handling, which is exactly what you'd need for a smarter "least-connections" strategy (send the next request to whoever's least busy, the thing nginx spells `least_conn` and Envoy calls `LEAST_REQUEST`). Here's how it's updated:

```go
func (b *Backend) Serve(rw http.ResponseWriter, req *http.Request) {
    b.reverseProxy.ServeHTTP(rw, req)
    b.mux.Lock()
    b.aliveConnections++
    b.mux.Unlock()
}
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/backend.go#L59-L64" target="_blank" rel="noopener noreferrer">↗ backend.go</a>

Spot it? The counter goes up *after* the request finishes, and it never comes back down. There's no matching `aliveConnections--`. So it's not "active connections" at all, it's a lifetime total of requests ever served. A number that only grows. To actually track active connections you'd increment *before* `ServeHTTP` and decrement *after*, in a `defer` so it survives a panic:

```go
func (b *Backend) Serve(rw http.ResponseWriter, req *http.Request) {
    b.mux.Lock(); b.aliveConnections++; b.mux.Unlock()
    defer func() { b.mux.Lock(); b.aliveConnections--; b.mux.Unlock() }()
    b.reverseProxy.ServeHTTP(rw, req)
}
```

As written, the field's name is a promise the code doesn't keep, and any least-connections logic built on it would be quietly, confidently wrong: it would always route to the backend that has served the *fewest requests since startup*, which after a restart means dumping everything on whichever server booted last. The name lies, and the lie is the kind that survives code review because everyone reads the name instead of the arithmetic.

## Bug four: the config file is a decoration

go-lb reads a `config.yaml` on startup and validates it. Here's the file:

```yaml
port: 3332
strategy: round-robin
backends:
  - "http://localhost:8080"
  - "http://localhost:8081"
  - "http://localhost:8082"
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/config.yaml#L1-L6" target="_blank" rel="noopener noreferrer">↗ config.yaml</a>

The config loader even *validates* that a port was provided, erroring out if `config.Port == 0`. So it insists you give it a port. Then `main.go` throws it away and hardcodes a different one:

```go
log.Println("Starting load balancer on port :9000")
if err := http.ListenAndServe(":9000", mux); err != nil {
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/main.go#L38-L39" target="_blank" rel="noopener noreferrer">↗ main.go</a>

You set `port: 3332`, the validator demands it, and the server binds `:9000` anyway. The `strategy: round-robin` line is worse: nothing in the codebase ever reads `config.Strategy`. The struct field exists, YAML happily unmarshals into it, and it is referenced exactly zero times. So the one knob that would make this a *configurable* load balancer, the choice of routing algorithm, is decorative. Round-robin isn't selected; it's the only thing there is.

This is the most human bug of the five, and the most common one in real codebases. The config file reads like a contract. Half of it isn't wired to anything. Nobody notices because the defaults happen to match what the ignored values say, so the demo works and the file looks authoritative. The tell is always the same: grep for where a config field is *read*, not where it's declared. A field that's declared and never read is a comment wearing a costume.

## Bug five: the one go vet catches, that I shipped anyway

This is the loud one. Run `go vet ./...` on go-lb and it reports three copies of the same complaint:

```
loadbalancer.go:12: InitLoadBalancer passes lock by value:
    ServerPool contains sync.Mutex
main.go:33: call of InitLoadBalancer copies lock value:
    ServerPool contains sync.Mutex
```

Here's the crime. `ServerPool` embeds a `sync.Mutex` by value, and then I pass the whole `ServerPool` *by value* into the constructor and copy it again into a struct literal:

```go
func InitLoadBalancer(serverPool ServerPool) *LoadBalancer {
    return &LoadBalancer{
        serverPool: serverPool,
    }
}
```

<a class="src-link" href="https://github.com/Vandit1604/go-lb/blob/70ec783994ca4c967ecb3b3161f9ac16b1a393ac/loadbalancer.go#L12-L16" target="_blank" rel="noopener noreferrer">↗ loadbalancer.go</a>

Copying a `sync.Mutex` is a genuine sin in Go, because a mutex's whole job is to be a single shared piece of state that every goroutine agrees on. Copy it and you now have two mutexes that don't know about each other; a goroutine locking one provides no protection against a goroutine reading through the other. go-lb gets away with it *only* by luck: the copy happens once, at startup, before any request goroutines exist, and from then on every request goes through the same `lb.serverPool` copy, so there's effectively one mutex in play. It works. It's still wrong, and it's exactly the kind of "works today" that becomes a heisenbug the day someone adds a second code path that touches the original `ServerPool`. The fix is to hold `*ServerPool` (a pointer) everywhere and never copy the struct. `go vet` told me this before I ever pushed. I shipped it anyway, which is its own small lesson about how easy it is to tune out the tooling that's trying to help you.

## Why I'm showing you my own bugs

Because this is the real education. Anyone can write the round-robin. What building go-lb actually taught me is that a load balancer is defined by its failure handling and its honesty about its own state, and both are exactly the stuff that's invisible when everything's healthy. The five bugs aren't embarrassing accidents, they're a map of where the real difficulty lives: **concurrency** (the mutex, the off-by-one, the copied lock), **liveness** (the check nobody calls), and **honest state** (the counter that grows forever, the config that's ignored).

Notice that not one of them is a crash. They're all cases where the code runs, the demo passes, and the behavior is quietly not what the names claim. That's the texture of infrastructure bugs generally. The system stays up and does the wrong thing calmly, and you find out from a graph three weeks later.

Production load balancers, HAProxy, Envoy, nginx, Traefik, are mostly *that hard part*: active and passive health checks, connection draining, timeouts, retries with budgets, circuit breaking, outlier ejection, weighted and least-request routing, sticky sessions. The routing loop is the 10%. The other 90% is what happens when a backend misbehaves, and every one of my five bugs is a tiny hole in that 90%. Writing the toy version is what made the shape of the real thing finally obvious to me.

## TL;DR

- A load balancer's core is a **picker plus a forwarder**. In Go the forwarder is `httputil.ReverseProxy`, which streams responses and handles the hard proxy spec for free. The picker is the part you design.
- Lock the rotation with a **mutex**, or you get a race under load that never shows up in testing.
- **Bug 1:** increment-then-index skips `backends[0]` on the first request. A one-character off-by-one.
- **Bug 2:** the health-check code exists but **nothing calls it**, so dead backends keep getting traffic. This is the whole point of a load balancer, and the easiest thing to leave unwired.
- **Bug 3:** `aliveConnections` counts up after each request and never down, so it's a lifetime total, not active connections. The name lies.
- **Bug 4:** `config.yaml` sets a port and a strategy that the code **never reads**. It binds a hardcoded `:9000` and only ever does round-robin. Config that isn't wired up is a comment in a costume.
- **Bug 5:** `go vet` flags a **copied mutex** (`ServerPool` passed by value). It works by luck because the copy happens once at startup. Still wrong. Hold a pointer.
- Real load balancers are 90% failure handling. The routing loop is the easy 10%.

## Go deeper

- The whole thing, ~230 lines: [github.com/Vandit1604/go-lb](https://github.com/Vandit1604/go-lb)
- Go's [`httputil.ReverseProxy`](https://pkg.go.dev/net/http/httputil#ReverseProxy), the standard-library workhorse, and its `Director`/`Rewrite` internals
- [How HAProxy does health checks](https://www.haproxy.com/documentation/haproxy-configuration-tutorials/reliability/health-checks/), active and passive, for what the grown-up version of bug two looks like
- [Envoy outlier detection](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier), the passive "eject a misbehaving host" pattern
- [`go vet`'s copylocks check](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/copylock), the analyzer that caught bug five

---

*Fun fact: I found most of these bugs by reading my own code back to write this post, not while writing the code. The `go vet` one I'd seen and ignored. Nothing teaches you a system like having to explain it out loud to a stranger.*
