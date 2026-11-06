---
title: "Retry With Exponential Backoff and Jitter"
date: "2026-11-27"
description: "Retry with exponential backoff and jitter, from zero: timeouts, idempotency, which errors to retry, a 1,000-client experiment, and a gRPC bug that dropped spans."
tags: ["tech", "retries"]
draft: "false"
---

You press "Pay". The page spins. Ten seconds later it says "Something went wrong". Did the payment go through? You don't know. The app doesn't know either. Now it has to decide: try again, or give up.

That decision is a *retry*, and it is riskier than it looks. A good retry saves a request from a one-second network hiccup. A bad one charges you twice, or turns a small server problem into a full outage. This post starts from zero: what a retry is, when it is safe, which errors deserve one, how long to wait, and why the wait needs *randomness*. In the middle I run an experiment with 1,000 clients and show the real output. At the end is a bug I found in VictoriaTraces, where the client logic was fine and the server still lost data.

<img class="meme" src="/static/images/blog/memes/retry-exponential-backoff-jitter-1.webp" alt="Reaction image: Dread when the payment page hangs and you don't know if you were charged" width="640" height="480" loading="lazy">

## A request fails. Now what?

A request can fail in three ways, and the client can't tell them apart:

1. The request never reached the server (network down, DNS failed).
2. The server got it, did the work, and the response got lost on the way back.
3. The server got it and refused or crashed.

A *retry* is just sending the same request again. It fixes case 1, and case 3 if the server recovers. Case 2 is where it hurts: the work is already done and you ask for it again.

So two questions come before any code. How long do you wait before you decide a request failed? And is it safe to send it twice? Those are timeouts and idempotency.

## Timeouts: no timeout, no failure to retry

If a request has no timeout, a stuck server holds your client forever. You never get an error, so you never retry. Every retry design starts with a timeout.

```go
client := &http.Client{Timeout: 2 * time.Second}
resp, err := client.Get("https://api.example.com/orders/42")
```

To pick the number, look at the latency of the call when things are healthy and choose a high percentile. The AWS Builders' Library picks an acceptable false-timeout rate and then reads the downstream latency at that percentile (p99.9 in its example). With p99.9, about 0.1% of calls time out even though they were only slow.

You can get it wrong in two ways. If it is too long, your threads or goroutines pile up waiting and users stare at a spinner. If it is too short, healthy but slow requests time out, you retry them, and the server now has *more* work. That is how a timeout causes the overload it was meant to survive.

Connect timeout and read timeout differ. Connecting to a healthy server takes milliseconds, so that timeout can be tight. Reading the response to a query can take longer. Set them separately if your client allows it.

## Is it safe to send it twice? Idempotency

An operation is *idempotent* if doing it twice has the same effect as doing it once. `PUT /users/7 {name: "Ana"}` is idempotent: the second call sets the same name. `DELETE /users/7` is idempotent too. `POST /charge {amount: 50}` is not: the second call charges another 50.

This is the case 2 problem from earlier. The server charged the card, the response was lost, and the client retried. The customer paid twice.

<img class="meme" src="/static/images/blog/memes/retry-exponential-backoff-jitter-2.webp" alt="Reaction image: Calm while the double charge happens" width="680" height="687" loading="lazy">

The fix is an *idempotency key*. The client makes a unique ID per logical action (a UUID) and sends it with every attempt:

```http
POST /charge HTTP/1.1
Idempotency-Key: 6f1c2a9e-5b0e-4a63-9c1e-0d6f1a7b8c11
Content-Type: application/json

{"amount": 50, "currency": "usd"}
```

The server stores the key with the result of the first attempt. If the same key arrives again, it skips the charge and returns the stored result. Stripe does this with its `Idempotency-Key` header on POST requests. EC2's `RunInstances` takes a `ClientToken` for the same reason.

```go
func charge(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if saved, ok := store.Get(key); ok {
		w.Write(saved) // second attempt: replay the first result
		return
	}
	result := doCharge(r)
	store.Put(key, result)
	w.Write(result)
}
```

(A real version also has to handle two attempts with the same key arriving at the same moment. Store the key as "in progress" first, atomically.)

<!-- DIAGRAM: sequence diagram. Client sends POST with key K. Server charges, stores K->result, response lost (X). Client retries with key K. Server finds K, returns stored result, no second charge. -->

So: retry freely when the operation is idempotent (GET, PUT, DELETE, or anything with a key). When it is not, add a key or don't retry.

## Which failures deserve a retry?

A retry only helps when the failure is *temporary*. A malformed request is still malformed the second time, so retrying it just wastes work.

For HTTP:

| Status | Meaning | Retry? |
|---|---|---|
| 400, 401, 403, 404, 422 | Your request is wrong or not allowed | Never |
| 408 | Server timed out waiting for your request | Yes |
| 429 | You are sending too much | Yes, after waiting |
| 500 | Server bug | Usually no (see below) |
| 502, 504 | Gateway got a bad answer or timed out | Yes |
| 503 | Server is overloaded or restarting | Yes |

`Retry-After` says how long to wait, in seconds or as a date. If a server sends it, use it before you invent your own delay. The OTLP spec says clients SHOULD honour it. MDN describes 429 as a limit on *you* (per-client throttling) and 503 as the server itself being unable to take more. Both mean "come back later".

I say "usually no" for 500 because it means "something broke inside and I don't know what", and it may break the same way again. Some clients retry it anyway. What matters more is that a server should not hide an *overload* behind a 500, which leads to gRPC.

### gRPC: UNAVAILABLE versus INTERNAL

gRPC has its own status codes. Every call ends with a `grpc-status`, and (this surprised me) the HTTP status of the response is still 200 even for errors. The codes that matter for retries:

| gRPC code | Meaning | Retry? |
|---|---|---|
| `UNAVAILABLE` (14) | The service can't take this right now | Yes. This is the "try again" code |
| `DEADLINE_EXCEEDED` (4) | Your deadline passed | Only if the call is idempotent |
| `RESOURCE_EXHAUSTED` (8) | A quota or limit was hit | Only if the server says when to come back |
| `INVALID_ARGUMENT` (3) | Your request is bad | Never |
| `INTERNAL` (13) | A broken invariant on the server | Not retried by most clients |

`INTERNAL` is the dangerous one. It is gRPC's "this should never happen" code, so clients treat it as a bug, not as a busy server. The OpenTelemetry Go SDK shows this in `retryableGRPCStatus`:

```go
switch s.Code() {
case codes.Canceled, codes.DeadlineExceeded, codes.Aborted,
	codes.OutOfRange, codes.Unavailable, codes.DataLoss:
	// Additionally handle RetryInfo.
	_, d := throttleDelay(s)
	return true, d
case codes.ResourceExhausted:
	// Retry only if the server signals that the recovery from resource exhaustion is possible.
	return throttleDelay(s)
}
// Not a retry-able error.
return false, 0
```

<a class="src-link" href="https://github.com/open-telemetry/opentelemetry-go/blob/fbc0aea1b3535ea2514de5a51eaef65641a5960c/exporters/otlp/otlptrace/otlptracegrpc/client.go#L299-L317" target="_blank" rel="noopener noreferrer">↗ otlptracegrpc/client.go · retryableGRPCStatus</a>

`Unavailable` is in. `Internal` is not. `ResourceExhausted` counts only if the server also sends a `RetryInfo` detail, gRPC's version of `Retry-After`. I read this list from the SDK's main branch on 2026-11-06 (latest release then: v1.47.0). It comes back later.

## What happens with no delay at all

Say you know what to retry. The simplest code is a loop:

```go
for attempt := 0; attempt < 10; attempt++ {
	if err := call(); err == nil {
		return nil
	}
}
```

There is no waiting. On an overloaded server this loop hits it ten times in microseconds. With 1,000 clients doing that at once, a server that struggled with 1,000 requests now gets 10,000. This is a *retry storm*: failures make more load, and more load makes more failures.

<img class="meme" src="/static/images/blog/memes/retry-exponential-backoff-jitter-3.webp" alt="Reaction image: Chaos, 1,000 clients all pushing through the door at once" width="640" height="480" loading="lazy">

I measure this below. First, the usual fix.

## Exponential backoff

Wait between attempts, and wait longer each time:

```
delay = min(cap, base * 2^attempt)
```

With base 100 ms and a 10 s cap: 100 ms, 200 ms, 400 ms, 800 ms, 1.6 s, 3.2 s, 6.4 s, then 10 s forever. The first retry is quick, in case the failure was a blip. If the problem persists, the client backs off and gives the server room.

Without the cap, delays grow to hours. Limit the attempts too: 3 to 5, plus a total deadline, so a request does not retry forever. Google's SRE book caps a request at 3 attempts.

This looks finished, but think about 1,000 clients that all failed at the same moment. They all wait exactly 100 ms, come back together, fail together, then all wait 200 ms. The load is still a synchronized wave, only slower. The server sees spikes at 100 ms, 300 ms, 700 ms, and so on.

## Jitter: add randomness

*Jitter* means picking a random delay instead of the exact one. AWS compared three styles in their "Exponential Backoff and Jitter" post:

```
full:         sleep = random(0, min(cap, base * 2^n))
equal:        t = min(cap, base * 2^n);  sleep = t/2 + random(0, t/2)
decorrelated: sleep = min(cap, random(base, previous_sleep * 3))
```

Full jitter spreads clients across the whole window: a client that would wait 800 ms now waits anywhere from 0 to 800 ms. After a few rounds the clients are out of step, and arrivals look like a stream, not a heartbeat. In AWS's simulation full jitter did the least work, so I use it by default.

<!-- DIAGRAM: three clients on a timeline. Top lane: no jitter, three retry marks aligned at 100, 300, 700 ms. Bottom lane: full jitter, marks scattered. -->

```go
func backoff(attempt int, base, cap time.Duration) time.Duration {
	d := base << attempt
	if d > cap || d <= 0 { // <= 0 catches shift overflow
		d = cap
	}
	return time.Duration(rand.Int63n(int64(d) + 1))
}
```

## The experiment: 1,000 clients, one small server

I didn't want to just repeat the AWS result, so I ran my own on a limiter I already had. The server is the limiter from my `queuelimit` kata in [go-drills](https://github.com/Vandit1604/go-drills): 20 slots, and a request that finds no free slot is rejected at once with a 503 (max wait of zero, like a `-maxConcurrentInserts` limit). A request that gets a slot holds it for 50 ms. So the server can finish 20 / 0.05 s = 400 requests per second.

1,000 clients start at the same moment. Each needs one success and gets at most 10 attempts. Three policies: A retries with no delay. B uses exponential backoff (base 100 ms, cap 10 s) without jitter. C uses the same backoff with full jitter. I count the requests that arrive in each 100 ms bucket. The server is called as a function (no network), so this measures the policy, not TCP.

```go
type limiter struct{ slots chan struct{} }

func (l *limiter) acquire() error {
	select {
	case l.slots <- struct{}{}:
		return nil
	default:
		return errBusy // the "503"
	}
}
func (l *limiter) release() { <-l.slots }

func run(delay func(attempt int, rng *rand.Rand) time.Duration, seed int64) result {
	lim := &limiter{slots: make(chan struct{}, 20)}
	// ...
	for i := 0; i < 1000; i++ {
		go func(i int) {
			rng := rand.New(rand.NewSource(seed + int64(i)))
			for a := 0; a < 10; a++ {
				count(time.Since(start) / bucket) // requests per 100 ms bucket
				if lim.acquire() == nil {
					time.Sleep(50 * time.Millisecond) // hold the slot
					lim.release()
					return // success
				}
				if a < 9 {
					time.Sleep(delay(a, rng))
				}
			}
		}(i)
	}
	// ...
}
```

Real output (Go 1.25.4, arm64 laptop; three runs per policy, buckets printed for the first run). The "ok" column counts the successes of the requests that arrived in that bucket.

```text
== A: no delay
run 1: success 20/1000, requests sent 9820, last success at 50ms
bucket(100ms)  requests  ok
    0 ms      9820    20
run 2: success 20/1000, requests sent 9820, last success at 50ms
run 3: success 20/1000, requests sent 9820, last success at 50ms
== B: backoff
run 1: success 200/1000, requests sent 9100, last success at 32.76s
bucket(100ms)  requests  ok
    0 ms      1000    20
  100 ms       980    20
  300 ms       960    20
  700 ms       940    20
 1500 ms       920    20
 3100 ms       900    20
run 2: success 200/1000, requests sent 9100, last success at 32.77s
run 3: success 200/1000, requests sent 9100, last success at 32.76s
== C: full jitter
run 1: success 1000/1000, requests sent 5361, last success at 6.7s
bucket(100ms)  requests  ok
    0 ms      2176    40
  100 ms       646    40
  200 ms       461    40
  300 ms       299    40
  400 ms       285    40
  500 ms       235    40
  600 ms       108    40
  700 ms        96    40
  ...
 2000 ms        31    28
  ...
 3900 ms         4     4
run 2: success 1000/1000, requests sent 5388, last success at 6.19s
run 3: success 1000/1000, requests sent 5402, last success at 7.13s
```

(The full run prints every non-empty bucket. I cut the middle of C; the program is above and the numbers are unedited.)

Reading it:

- **A.** All 9,820 requests land in the first 100 ms bucket. The server serves 20 and the other 980 clients burn their 10 attempts before the first slot frees. Success: 2%. The clients retried and got nothing, in a few microseconds.
- **B.** You can see the waves. Buckets exist only at 0, 100, 300, 700, 1500 and 3100 ms: the sums of the backoff delays. Every wave has about 1,000 requests, and the server finishes 20 of them. After 10 waves, 200 clients have succeeded. Backoff *without* jitter gave the server breathing room between waves but never spread the clients. Each wave still hit with nearly the full crowd.
- **C.** All 1,000 succeed, with about 5,400 requests, roughly 45% fewer than B and A. The first bucket still has a spike (2,176 requests, everyone starts at t = 0), but each later bucket is much smaller, and by 2 seconds the arrival rate matches the rate the server can finish. Last success after 6.2 to 7.1 s, against 32.8 s for B, which still has 800 clients unserved.

One honest caveat. B is not broken forever: with more than 10 attempts the same waves would eventually serve everyone, at the cost of many more requests. The 10-attempt cap is what turns "slow" into "lost". Real systems have this cap too.

<img class="meme" src="/static/images/blog/memes/retry-exponential-backoff-jitter-4.webp" alt="Reaction image: Relief, everyone leaves through the gates in an orderly stream" width="640" height="486" loading="lazy">

## Retries multiply: budgets and amplification

One more trap, and it takes down real systems. Say a request passes through five layers: a web server, an API, a service, a cache layer, and a database. Each layer retries 3 times. When the database gets slow, one user request becomes 3 calls at layer 2, each of those makes 3 calls at layer 3, and so on. At the database that is 3^5 = 243 times the load, just when it is struggling most. The AWS Builders' Library uses this same example.

Two defences:

1. Retry in one layer only. The others pass the error up without retrying. (AWS suggests this for low-cost operations; pick the layer yourself.)
2. Use a retry budget: allow retries only as a fraction of normal traffic. Google's SRE book retries a request only while retries stay under 10% of that client's requests. The gRPC retry design uses a token bucket for this. With `maxTokens: 10` and `tokenRatio: 0.1`, the bucket starts at 10, each failed call removes 1, each success adds 0.1, and retries stop while the count is at or below half of `maxTokens`. In a long outage that keeps the extra load near 1.1x, not 3x or 243x.

<!-- DIAGRAM: five boxes in a chain labelled web, api, service, cache, db. Arrows labelled x3 between each. Last box labelled 243x. -->

## Circuit breakers, briefly

A retry budget limits the *amount* of retrying. A *circuit breaker* stops it completely when a dependency stays down. It has three states:

- **Closed:** requests flow normally. Failures are counted.
- **Open:** too many failures, so the breaker rejects calls immediately without touching the dependency.
- **Half-open:** after a pause, let a few calls through. If they work, close. If they fail, open again.

Use one when a dependency can be down for minutes and you want to fail fast the whole time. The cost is a second mode in your system, which the AWS authors say can be difficult to test. So, budget or breaker? The budget is a rate limit on retries, always on. The breaker is an on/off switch for a dependency that looks dead. Many systems need only the budget.

## My case: when the server returns INTERNAL under overload

Now the part I found in real code. [VictoriaTraces](https://github.com/VictoriaMetrics/VictoriaTraces) is a tracing database. Its ingest component, `vtinsert`, accepts spans over OTLP, both HTTP and gRPC. It has a flag, `-maxConcurrentInserts`, that limits how many inserts run at once. It works like the limiter in my experiment: when the limit is reached, new inserts are rejected.

Over OTLP/HTTP the rejection is a 503 or 429. Clients know those. Per the OTLP spec, clients should retry 429, 502, 503 and 504, so the spans get through once the server calms down.

Over OTLP/gRPC the same rejection came back as `grpc-status: 13`, INTERNAL. I filed this as [issue #306](https://github.com/VictoriaMetrics/VictoriaTraces/issues/306). The same happened when the disk was almost full and storage went read-only. Now go back to the OpenTelemetry Go SDK code above. `Internal` is in the "not retry-able" branch. So the SDK got an overload, saw a bug-class error, and dropped the batch.

You can reproduce the error side with the command from the issue:

```bash
./victoria-traces-prod -otlpGRPC.tls=false -storage.minFreeDiskSpaceBytes=1000000000000000
```

Send one span over gRPC and the answer is `grpc-status: 13`. Send the same span over HTTP and the answer is 429. The cause was one line: every error path in the gRPC handler passed the constant `grpc.StatusCodeInternal`:

<a class="src-link" href="https://github.com/VictoriaMetrics/VictoriaTraces/blob/cf59ff4822f53774ec8faa3ccb4501c8d0f9f220/app/vtinsert/opentelemetry/otlpgrpc.go#L94" target="_blank" rel="noopener noreferrer">↗ app/vtinsert/opentelemetry/otlpgrpc.go · otlpExportTracesHandler (line 94)</a>

The fix is in [PR #307](https://github.com/VictoriaMetrics/VictoriaTraces/pull/307), which is **open** at the time of writing. It makes the gRPC handlers take the status from the error, the same way the HTTP side's `httpserver.Errorf` does. If the error carries 429, 502, 503 or 504, the handler answers `UNAVAILABLE`. Everything else stays `INTERNAL`. The gRPC test helpers also had to change: gRPC errors arrive with HTTP 200, so a check for "status is 200" passed even when the call had failed. The helpers now read `grpc-status`.

<img class="meme" src="/static/images/blog/memes/retry-exponential-backoff-jitter-5.webp" alt="Reaction image: Betrayal, the client retried perfectly and the server still lost the data" width="680" height="597" loading="lazy">

To see the effect, I ran this test on my M-series MacBook. I started `vtinsert` with `-maxConcurrentInserts=1` and `-insert.maxQueueDuration=1ms`, so most parallel requests are rejected. Then 30 clients sent 3,000 spans over OTLP/HTTP and 3,000 over OTLP/gRPC, 10 spans per request, using the official OpenTelemetry Go SDK exporters (v1.47.0) with their default retry settings. I counted the stored spans per service name with LogsQL. I ran it on the current `master` and on the head of PR #307:

| transport | sent | stored on `master` | stored on PR #307 |
|---|---|---|---|
| OTLP/HTTP | 3,000 | 3,000 | 3,000 |
| OTLP/gRPC | 3,000 | 2,390 | 3,000 |

On `master`, 61 of the 300 gRPC requests failed in the SDK without a retry, so 610 spans were lost. The exact loss changes from run to run (2,280 to 2,420 spans stored in my runs), because it depends on which requests collide. The HTTP side lost nothing, because the exporter retried the 429 and 503 answers. On the PR branch, the gRPC side also lost nothing.

Most retry guides I read explain the *client*. This bug had a correct client. The server's status code is part of the retry contract: an overloaded server has to say so with a code the client knows is temporary (503, 429, `UNAVAILABLE`), not the code for "I am broken". Otherwise even perfect backoff only decides how quickly the client gives up.

I also checked whether the server sends `Retry-After` or `RetryInfo` on these paths. I did not find either in `otlpgrpc.go` and did not check elsewhere, so the client falls back to its own backoff. The OTel Go exporter defaults are 5 s initial interval, 30 s max interval and 1 minute total elapsed time (`retry.DefaultConfig`).

## Cheat sheet

**Client**

- Set a timeout first. Pick it from the healthy p99.9 latency.
- Retry only idempotent calls, or attach an idempotency key (one UUID per logical action, reused on every attempt).
- Retry: HTTP 408, 429, 502, 503, 504; gRPC `UNAVAILABLE`. `DEADLINE_EXCEEDED` only if idempotent. `RESOURCE_EXHAUSTED` only with a server hint.
- Never retry: HTTP 400, 401, 403, 404, 422; gRPC `INVALID_ARGUMENT`; `INTERNAL`.
- Honour `Retry-After` or `RetryInfo` first. Otherwise: `delay = random(0, min(cap, base * 2^n))`.
- Limit attempts (3 to 5) and set a total deadline.
- Keep retries under about 10% of requests with a budget. Retry in one layer only: 3 layers of 3 retries is 27x, 5 layers is 243x.
- Add a circuit breaker only for a dependency that can stay down for a long time.

**Server**

- Overload is 503 or 429 (HTTP), `UNAVAILABLE` (gRPC). Never 500 or `INTERNAL`.
- Send `Retry-After` when you know how long.
- Store results under idempotency keys for any non-idempotent endpoint.

**Numbers from my run** (1,000 clients, 20 slots, 50 ms hold, 10 attempts): no delay 2% success, backoff 20%, backoff with full jitter 100%.

My post on [building a load balancer in Go](/blogs/load-balancer-from-scratch-go) asks the same question, what the server says when it is full, from the other side.
