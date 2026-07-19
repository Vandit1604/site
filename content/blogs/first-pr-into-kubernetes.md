---
title: "My First Merged PR Into Kubernetes Was 25 Lines of Tests. Here's Why That's the Move"
date: "2024-01-24"
description: "My first merged PR into Kubernetes was 25 lines of tests. Why a small, test-only change is the smartest way into a five-million-line repo."
tags: ["opensource", "golang"]
---

Kubernetes is roughly five million lines of Go. The first time I opened it with any intention of contributing, I did the thing everyone does: scrolled for a while, felt my soul leave my body, and closed the tab. How do you even *start* on something that big? Where's the door?

Here's the answer nobody tells you, and it's almost annoyingly boring: **you start with tests.** My [first merged PR into `kubernetes/kubernetes`](https://github.com/kubernetes/kubernetes/pull/122625) added exactly 25 lines, all of them tests, zero lines of production code. It's titled "Negative index regression test for json-patch." It is not glamorous. It is exactly how you get in.

## Why tests are the perfect first PR

Think about it from the maintainer's side. A stranger shows up and wants to change the core logic of the thing that runs half the internet's infrastructure. That's scary. That's a lot of review, a lot of risk, a lot of "who are you and why should I trust this."

Now a stranger shows up and says "I noticed this behavior isn't locked down by a test. Here's a test that pins it." That's not scary at all. You're not changing behavior, you're *protecting* it. The risk to the maintainer is near zero, the value is real, and you've just proven you can navigate the codebase, run its test suite, and follow its contribution rules. That's the entire audition.

<aside class="callout callout--tip" data-label="The reframe">
A test-only PR says three things about you without you saying them: I can find my way around this codebase, I understand this specific behavior well enough to assert on it, and I follow your process. That's most of what a maintainer wants to know about a new contributor. You demonstrate it instead of claiming it.
</aside>

## What the PR actually did

The area was JSON Patch, the [RFC 6902](https://datatracker.ietf.org/doc/html/rfc6902) format Kubernetes uses to apply partial updates to objects. JSON Patch lets you point at an array element by index, including the special "last element" style path. The behavior existed and worked, but it wasn't nailed down by a regression test, which means a future refactor could silently break it and nobody would notice until production did.

So the whole PR is just: here are the cases that must keep working. One in the apiserver's patch handler:

```go
{
    name:  "valid-negative-index-patch",
    patch: `[{"op": "test", "value": "foo", "path": "/metadata/finalizers/-1"}]`,
},
```

...with the object set up so `finalizers` actually has an element to point at:

```go
pod.ObjectMeta.Finalizers = []string{"foo"}
```

And one in kubectl's helpers, checking that a `replace` at the last-index path swaps the right element:

```go
{
    obj: &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:       "foo",
            Finalizers: []string{"foo", "bar", "test"},
        },
    },
    fragment: `[ {"op": "replace", "path": "/metadata/finalizers/-1", "value": "baz"} ]`,
    // expected: finalizers become ["foo", "bar", "baz"]  ← last one replaced
},
```

<a class="src-link" href="https://github.com/kubernetes/kubernetes/pull/122625/files" target="_blank" rel="noopener noreferrer">↗ kubernetes/kubernetes#122625 (files changed)</a>

That's the whole change. A `test` op that asserts the last finalizer is `"foo"`, and a `replace` op that turns `["foo", "bar", "test"]` into `["foo", "bar", "baz"]`. Twenty-five lines across two test files. Merged into Kubernetes.

<aside class="callout" data-label="Why finalizers">
Finalizers are a nice target for this because they're a plain string slice on basically every Kubernetes object, so a test using them is simple and representative. You don't need an exotic resource to prove that last-index array patching works. The most ordinary field in the API does the job.
</aside>

## The part that's actually hard (and it's not the code)

The 25 lines were the easy bit. The real work of a first contribution to a giant repo is everything *around* the code:

- **Finding the seam.** You're not looking for a big feature. You're looking for a small, true gap: a behavior that works but isn't tested, a flaky test, an error message that lies. Those are everywhere in big repos, and they're the perfect size for a first PR.
- **Getting the build and tests to run at all.** Kubernetes has a whole developer setup. Just getting to "I can run this one test file locally" is a genuine milestone, and it's the thing that actually teaches you the repo.
- **The process.** CLA, commit conventions, the CI robots (`/ok-to-test`, the label bots), the review cadence. Big projects are as much bureaucracy as code, and learning to move through it politely is a real skill.

None of that is intellectually hard. It's just unfamiliar, and unfamiliar feels like hard until you've done it once. After the first one, the second PR into the same repo is ten times easier, because you've paid the setup cost already.

<aside class="callout callout--warn" data-label="Don't do this">
The classic first-PR mistake is going big to look impressive: rewriting a subsystem, "fixing" an architecture you don't fully understand yet. It reads as risky, it's a nightmare to review, and it usually gets closed. Small and correct beats big and speculative every single time when nobody knows you yet. Earn trust in cheap increments.
</aside>

## The staircase

The thing that first PR unlocked wasn't the 25 lines. It was proof, to the maintainers and to me, that I could do this. The next contributions got bigger. Eventually I was touching real logic in [Prometheus' query layer](https://github.com/prometheus/prometheus/pull/15552) and orchestration in [Kueue and Kubernetes test-infra](https://github.com/search?q=author%3AVandit1604+is%3Apr+is%3Amerged&type=pullrequests). But none of that would have happened if I'd waited until I felt "ready" to make a big splash.

You don't walk into the biggest repo in the world and make a grand entrance. You slip in through the tests, prove you belong one small correct change at a time, and climb from there.

## TL;DR

- Kubernetes is ~5M lines. The way in is not a big feature, it's a **small, test-only PR**.
- A test-only change is **near-zero risk** for the maintainer and proves you can navigate the repo, understand a behavior, and follow the process. That's the whole audition.
- My first merge: [25 lines of regression tests](https://github.com/kubernetes/kubernetes/pull/122625) locking in last-index JSON Patch behavior on finalizers. Zero production code.
- The code is easy. The **setup, the seam-finding, and the process** are the real first-timer work, and you only pay that cost once.
- Small and correct beats big and speculative when nobody knows you yet.

## Go deeper

- The PR itself: [kubernetes/kubernetes#122625](https://github.com/kubernetes/kubernetes/pull/122625)
- The [Kubernetes contributor guide](https://www.kubernetes.dev/docs/guide/), which is genuinely good
- [RFC 6902 (JSON Patch)](https://datatracker.ietf.org/doc/html/rfc6902), if you want to understand what was being tested
- More on the mindset in [going from scared of the repo to mentoring people into it](/blogs/mentee-to-mentor-open-source)

---

*Fun fact: my most-merged contribution to "big" open source is, by line count, mostly tests and CI plumbing. The unglamorous work is not the consolation prize. It's the actual door.*
