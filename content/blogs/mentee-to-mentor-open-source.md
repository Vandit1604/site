---
title: "From Scared of the Repo to Mentoring People Into It"
date: "2025-02-20"
description: "Going from scared of a big repo to mentoring people into open source in two years, told through the real merged PRs that built the path."
tags: ["opensource", "ideology"]
---

Everyone's first open source contribution starts the same way: you find a huge, famous project, open the code, and immediately feel like you wandered into the cockpit of a plane mid-flight. Thousands of files. Contributors who clearly do this in their sleep. A `CONTRIBUTING.md` that assumes you already know eleven things you don't. And a little voice going "you do not belong here, close the tab."

This post is about ignoring that voice, because ignoring it is basically the whole game. I went from "too scared to open a PR" to being a [GSoC mentor](https://www.jenkins.io/projects/gsoc/2024/) two years later, and the path was way less mysterious than it looked from the bottom. Here's what actually worked, with the real PRs so you can see it wasn't magic.

## Nobody starts by writing the cool part

Here's the thing they don't tell you. Your first contribution to a massive project is almost never the exciting distributed-systems wizardry. Mine, with Jenkins, was documentation and tooling. I helped rebuild the Jenkins docs, read by **11 million+ users**, onto a proper versioned stack (GatsbyJS + Antora). That included unglamorous but real work like [porting the whole site over](https://github.com/jenkins-infra/docs.jenkins.io/pull/106) and dragging a pile of plugins up to date.

And I remember thinking, at the time, "is docs even a real contribution?" Reader, it is the realest one. To fix the docs you have to understand the thing, understand its build, understand where people actually get confused. You end up learning the project *sideways*, through its edges, which is honestly a cheat code. By the time you're comfortable in the docs and the CI, the scary code isn't so scary anymore. You've been living next door to it for months.

That was [GSoC 2023](https://www.jenkins.io/projects/gsoc/2023/). I went in as a **mentee**, fully expecting to be found out as a fraud any day. Never happened. Turns out everyone's just figuring it out at their own layer.

<aside class="callout" data-label="Note">
"Edges" means docs, tests, CI, tooling, build scripts, error messages. The stuff that surrounds the core logic. It's lower stakes, easier to get reviewed, and it teaches you the codebase's shape faster than staring at the hardest file in the repo ever will.
</aside>

## The trick is picking the boring door

If you're staring at [`kubernetes/kubernetes`](https://github.com/kubernetes/kubernetes) or [`prometheus/prometheus`](https://github.com/prometheus/prometheus) wondering how anyone contributes to *that*, here's my actual advice: **don't look for the impressive change. Look for the annoying one.**

- A flaky test everyone hates but nobody's fixed.
- A regression that needs a test to lock it down so it never comes back. That's literally what my [first merge into core Kubernetes](https://github.com/kubernetes/kubernetes/pull/122625) was: a regression test for a negative-index bug in json-patch.
- Some CI orchestration held together with tape. A big chunk of my [Kueue](https://github.com/kubernetes-sigs/kueue/pull/1552) and [Prometheus test-infra](https://github.com/prometheus/test-infra/pull/777) work was exactly this.

These are unglamorous, which is precisely why they're wide open, and why maintainers *love* the person who takes them. Do enough of them and you start getting handed the real stuff. Eventually I was in the actual query layer of Prometheus, [adding a limit parameter to `/query` and `/query_range`](https://github.com/prometheus/prometheus/pull/15552), and [exposing a new metric in memcached_exporter](https://github.com/prometheus/memcached_exporter/pull/227). None of that was the *first* thing I touched. It was the tenth.

<aside class="callout callout--tip" data-label="The move">
"Contributing to Kubernetes" sounds like scaling a cliff. Up close it's a hundred small handholds. You never need to make the whole climb at once. You only need to reach the next hold: one flaky test, one missing regression test, one confusing doc.
</aside>

## Then one day you're on the other side

In 2024 I came back to GSoC. Not as a mentee. As a [**mentor**](https://www.jenkins.io/projects/gsoc/2024/).

Being on the other side of the table rewired how I saw the whole thing. Because now I was watching new contributors do the *exact* thing I did: freeze up at the size of the repo, apologize for "dumb questions," assume their small PR wasn't worth submitting. And I got to be the person going "no, submit it, it's good, this is literally how it works."

The gap between mentee and mentor felt like a canyon when I was staring up at it. Turns out it's one year and a pile of small, finished contributions. Nobody hands you a badge that says you belong. You just keep showing up until, one day, you're the one telling someone else they belong.

## TL;DR

- The scared feeling on your first big-repo PR is normal and semi-permanent. Everyone feels it. Open the PR anyway.
- Start at the **edges**: docs, tests, tooling, CI. You learn the project sideways, and it's the fastest way in.
- Chase the **annoying** tasks, not the impressive ones. Flaky tests and regressions are how you get merged into projects like [Kubernetes](https://github.com/kubernetes/kubernetes/pull/122625) and [Prometheus](https://github.com/prometheus/prometheus/pull/15552).
- Do enough small finished things and "mentee to mentor" stops looking like a canyon and starts looking like a staircase.

If you've been circling some giant repo waiting to feel ready, this is your sign. You're not going to feel ready. Pick the boring door and walk in.

## Go deeper

- My [merged PRs across these projects](https://github.com/pulls?q=is%3Apr+author%3AVandit1604+is%3Amerged) if you want to see the actual trail
- [Kubernetes contributor guide](https://www.kubernetes.dev/docs/guide/) and [Prometheus contributing docs](https://github.com/prometheus/prometheus/blob/main/CONTRIBUTING.md)
- [How Google Summer of Code works](https://summerofcode.withgoogle.com/), if you want to get paid to do your first contributions

---

*Fun fact: I still get the "do I belong here" flicker opening an unfamiliar codebase. The difference now is I know it's a liar. Go find one flaky test in a project you admire this week. That's the whole first step.*
