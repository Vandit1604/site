---
title: "Observability That Actually Helps at 3am"
date: "2026-05-27"
description: "Wiring OpenTelemetry traces, metrics, and logs into Prometheus and Grafana across microservices. The one move that matters (trace IDs in every log line), and the cardinality, sampling, and clock-skew traps that waste your time."
tags: ["infra", "observability"]
---

Here's the moment observability is actually for. It's 3am, one request out of thousands was slow or errored, and it touched eight services on its way through. The user is angry, the dashboard says "p99 latency up," and you have no idea *which* of the eight did the damage. If your answer to that question is "SSH into each service and grep the logs by timestamp and hope," you don't have observability. You have logs, which is not the same thing.

I spent a stretch wiring real observability across a set of microservices: OpenTelemetry for instrumentation, feeding metrics into Prometheus and traces and logs into Grafana. The tooling is well-trodden and I'll cover it. But the thing that actually changed how fast we could debug wasn't a tool, it was one correlation trick that ties the three signals together. Let me build the picture, land on that trick, and then the traps that cost real time, because half of them are the kind you only learn by hitting.

## Three signals, and the thing that makes them useful

Observability is usually described as three pillars, and the description is accurate but misses the point.

- **Metrics** are cheap aggregate numbers over time: request rate, error rate, p99 latency. Great for "is something wrong and roughly where," useless for "why is *this* request slow."
- **Traces** follow a single request across every service it touches, as a waterfall of timed spans. Great for "where did this specific request spend its time."
- **Logs** are the detailed events a service emits: the actual error message, the actual value that was wrong.

Each pillar answers a different question, and on their own they're three separate haystacks. The insight that makes them worth the trouble is **correlation**: being able to jump from a metric spike to an example trace, and from a span in that trace to the exact log lines it produced. The connective tissue for all of it is a single **trace ID** that follows a request everywhere it goes. Get that right and the three haystacks become one searchable story. Skip it and you've bought three dashboards you still can't cross-reference at 3am.

## OpenTelemetry: instrument once

The temptation is to instrument for whatever backend you happen to use. [OpenTelemetry](https://opentelemetry.io/) (OTel) is the standard that saves you from that: a vendor-neutral SDK and wire format for traces, metrics, and logs. You instrument your code once against OTel, and you can export to Prometheus, Jaeger, Tempo, Grafana, Datadog, whatever, without touching the instrumentation again. That neutrality matters more than it sounds, because the day you want to switch tracing backends, you really don't want it to be a code change in forty services.

The piece that makes tracing actually work across services is **context propagation**. A trace ID is created at the edge (the first service to see the request), and it has to travel across every service boundary so downstream spans attach to the same trace. OTel does this with the [W3C Trace Context](https://www.w3.org/TR/trace-context/) standard: a `traceparent` HTTP header carrying the trace ID and parent span ID. Service A makes a call, the header rides along, service B reads it and creates its spans as children of A's. Do this consistently and a request through eight services is one connected waterfall. Miss it in one service and the trace snaps in half there.

## The one move: put the trace ID in every log line

This is the trick, and it's almost embarrassingly simple for how much it changed. Every service already logs. The move is to log in **structured** form (JSON with fields, not printf strings) and inject the current **`trace_id` and `span_id` into every single log line**.

```json
{"level":"error","service":"billing","trace_id":"7f3a...c1","span_id":"a12b...","msg":"charge failed: card declined","order_id":"ord_991"}
```

Now watch what that unlocks. You find a slow trace in the tracing UI. It has a trace ID. You paste that trace ID into your log search and instantly get *every log line from that exact request, across all eight services, in order.* No timestamp guessing, no per-service grepping. And it runs the other way too: a scary error log has a trace ID, so from one log line you can pull up the full request waterfall and see what happened before and after. The trace ID is the join key between two systems that otherwise don't know about each other.

<aside class="callout callout--tip" data-label="The whole payoff">
Before: "one request was slow, go grep eight services by timestamp." After: "here's the trace, here's every log it produced, here's the span that took 900ms." The mean-time-to-diagnose difference is not incremental. It's the difference between debugging a distributed system and guessing about one. The trace ID in the log line is the cheapest high-leverage thing on this whole list.
</aside>

## Where it all lands

The signals flow to tools that each specialize: **metrics to Prometheus**, traces to a trace store (Jaeger or Grafana Tempo), logs to a log store (Loki or similar), and **Grafana** on top as the single pane that queries all three. The [OTel Collector](https://opentelemetry.io/docs/collector/) usually sits in the middle as a pipeline: services send to the Collector, it processes and fans out to each backend, so your services don't need to know where anything ultimately goes.

The feature that makes correlation feel magic in Grafana is **exemplars**: Prometheus can attach an example trace ID to a metric data point, so when you see a latency spike on a graph, you can click the spike and jump straight to a trace from that exact moment. Metric to trace to log, in three clicks, from a graph to the specific line of a specific request. That chain is the entire goal, and it only exists because a trace ID threads through all three.

## The traps that cost real time

The wiring is the easy 80%. Here's the 20% that bites.

**Cardinality will melt your metrics.** This is the big one, and it's the same trap I wrote about in [how Prometheus finds series](/blogs/how-prometheus-finds-series): every distinct combination of metric label values creates a separate time series. Put a high-cardinality field like `user_id`, `order_id`, or `request_id` on a *metric* label and you don't get one series, you get millions, and you take Prometheus down. The rule that keeps you safe: **high-cardinality data belongs on traces and logs, never on metric labels.** Metrics are for bounded dimensions (service, endpoint, status code). The per-request specifics go in the trace and the log, where cardinality is free. Getting this wrong is the single most common way teams break their own monitoring.

<aside class="callout callout--warn" data-label="The rule">
If a label can take thousands of distinct values, it is not a metric label. It's a trace attribute or a log field. "Which user hit this" is a log question. "How many 500s per endpoint" is a metric question. Cross those wires and Prometheus pays for it in memory until it falls over.
</aside>

**Sampling means the trace you want might not exist.** At real volume you cannot store every trace, so you sample. **Head sampling** decides at the start of the request (cheap, but you might drop the one that turned out to be interesting). **Tail sampling** decides after the whole trace completes, so you can keep all the errors and slow ones and drop the boring successes (smarter, but the Collector has to buffer spans and it's more machinery). Either way, internalize that a sampled-out request leaves no trace, literally, so "I can't find the trace" sometimes means "it wasn't kept," not "it didn't happen."

**Propagation drops at async boundaries.** Synchronous HTTP call chains propagate context easily. The places it silently breaks are the seams: a message pushed onto a queue, work handed to a background goroutine or thread pool, a batch job. If you don't explicitly carry the trace context across those boundaries, the downstream work starts a brand-new orphan trace and your beautiful connected waterfall has a hole exactly where the interesting async work happened. Async is where you have to be deliberate about propagation.

**Clock skew makes span timings lie.** Span start and end times come from each service's own clock. If two services' clocks disagree, you get traces where a child span appears to start before its parent, or a network hop shows a negative duration. It's the same lesson that bit me doing [service-to-service auth](/blogs/service-to-service-auth-from-scratch): distributed systems assume clocks agree, and they don't unless you run NTP and mean it. A trace that shows impossible timings is usually a skew problem, not a logic one.

**Logs are the expensive pillar.** Metrics are tiny, traces are sampled, but logs are voluminous and storing them adds up fast. The nice second-order effect of trace-ID correlation is that it lets you log *less*: you don't need to log the entire context on every line if you can always pull the full trace for a request. Log the event and the trace ID, not the whole world, and lean on correlation to reconstruct the rest.

## TL;DR

- Metrics, traces, and logs each answer a different question. They're only powerful when **correlated by a trace ID** that follows the request everywhere.
- **OpenTelemetry** instruments once, vendor-neutrally, and propagates context across services via the **`traceparent`** header. One un-propagating service breaks the trace.
- The highest-leverage move: **structured logs with `trace_id` in every line.** Now a trace jumps you to all its logs, and a log jumps you to its trace, across every service.
- Signals land in **Prometheus (metrics) + Tempo/Jaeger (traces) + Loki (logs) under Grafana**, with **exemplars** linking a metric spike straight to a trace.
- Traps: **cardinality** (high-cardinality fields go on traces/logs, never metric labels), **sampling** (the trace you want may not be stored), **async propagation gaps**, **clock skew** (impossible span timings), and **log cost** (correlation lets you log less).

Observability isn't about having dashboards. It's about being able to answer "why was *this* request slow" without guessing, and the whole thing hinges on one ID threading through three systems. Get the trace ID everywhere and the 3am page becomes a five-minute investigation instead of an hour of grep.

## Go deeper

- [OpenTelemetry](https://opentelemetry.io/) and the [OTel Collector](https://opentelemetry.io/docs/collector/), the vendor-neutral instrumentation layer and pipeline
- [W3C Trace Context](https://www.w3.org/TR/trace-context/), the `traceparent` standard that makes cross-service tracing work
- [Prometheus exemplars](https://prometheus.io/docs/prometheus/latest/feature_flags/#exemplars-storage) and the [Grafana LGTM stack](https://grafana.com/oss/) (Loki, Grafana, Tempo, Mimir) for tying the three signals together
- My post on [how Prometheus finds series](/blogs/how-prometheus-finds-series), for why cardinality is the thing that breaks your metrics

---

*Fun fact: the fastest observability win I ever shipped wasn't a new tool. It was adding one field, `trace_id`, to the structured logger's default context. A day of work, and suddenly every log line knew which request it belonged to. Some of the best infrastructure changes are boring one-liners with enormous blast radius.*
