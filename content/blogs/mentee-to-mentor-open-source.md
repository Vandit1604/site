---
title: "From Scared of the Repo to Mentoring People Into It"
date: "2025-02-20"
tags: ["opensource", "ideology"]
---

Everyone's first open source contribution starts the same way: you find a huge, famous project, open the code, and immediately feel like you wandered into the cockpit of a plane mid-flight. Thousands of files. Contributors who clearly do this in their sleep. A `CONTRIBUTING.md` that assumes you already know eleven things you don't. And a little voice going "you do not belong here, close the tab."

I want to talk about ignoring that voice, because ignoring it is basically the whole game. This is the story of going from "too scared to open a PR" to being a **GSoC mentor** two years later, and what I actually learned in between.

## Nobody starts by writing the cool part

Here's the thing they don't tell you. Your first contribution to a massive project is almost never the exciting distributed-systems wizardry. Mine, with Jenkins, was documentation and tooling. I helped rebuild the Jenkins docs, the ones read by **11 million+ users**, onto a proper versioned stack (GatsbyJS + Antora).

And I remember thinking, at the time, "is docs even a real contribution?" Reader, it is the realest one. Because to fix the docs you have to understand the thing, understand the build, understand how people actually get confused. You end up learning the project *sideways*, through its edges, which is honestly a cheat code. By the time you're comfortable in the docs and the CI, the scary code isn't so scary anymore. You've been living next door to it for months.

That was GSoC 2023. I went in as a **mentee**, fully expecting to be found out as a fraud any day. Never happened. Turns out everyone's just figuring it out at their own layer.

## The trick is picking the boring door

If you're staring at `kubernetes/kubernetes` or `prometheus/prometheus` wondering how anyone contributes to *that*, here's my actual advice: **don't look for the impressive change. Look for the annoying one.**

- A flaky test that everyone hates but nobody's fixed.
- A regression that needs a test to lock it down so it never comes back.
- Some CI orchestration that's held together with tape.

These are unglamorous, which is exactly why they're wide open, and why maintainers *love* the person who takes them. That's how my code ended up merged into Kubernetes and Prometheus, not by parachuting in with a genius feature, but by picking up things that were just sitting there being irritating. Small, real, reviewed, merged. Repeat.

The dirty secret is that "contributing to Kubernetes" sounds like scaling a cliff, but up close it's a hundred small handholds. You only need to reach the next one.

## Then one day you're on the other side

In 2024 I came back to GSoC. Not as a mentee. As a **mentor**.

And being on the other side of the table rewired how I saw the whole thing. Because now I was watching new contributors do the *exact* thing I did, freeze up at the size of it, apologize for "dumb questions," assume their small PR wasn't worth submitting. And I got to be the person going "no, submit it, it's good, this is literally how it works."

The gap between mentee and mentor felt like a canyon when I was staring up at it. Turns out it's one year and a pile of small, finished contributions. That's it. Nobody hands you a badge that says you belong. You just keep showing up until, one day, you're the one telling someone else they belong.

## TL;DR

- The scary feeling on your first big-repo PR is normal and permanent-ish. Everyone feels it. Open the PR anyway.
- Start at the **edges**, docs, tests, tooling, CI. You learn the project sideways and it's the fastest way in.
- Chase the **annoying** tasks, not the impressive ones. Flaky tests and regressions are how you get merged into projects like Kubernetes and Prometheus.
- Do enough small finished things and the "mentee to mentor" jump stops looking like a canyon and starts looking like a staircase.

If you've been circling some giant repo waiting to feel ready, this is your sign. You're not going to feel ready. Pick the boring door and walk in.

---

*Fun fact: I still get the "do I belong here" flicker opening an unfamiliar codebase. The difference now is I know it's a liar. Go find one flaky test in a project you admire this week. That's the whole first step.*
