---
title: "How I Got Into GSoC (and Stopped Being Scared of Jenkins)"
date: "2023-05-22"
description: "How I got into Google Summer of Code with Jenkins by showing up early at the edges, and why you contribute to get good, not after."
tags: ["opensource", "ideology"]
---

A year ago I could not have told you what a "reverse proxy" was without sweating. My Java was rough, my CI/CD knowledge was basically zero, and "DevOps" was a word I nodded at in meetups without fully getting. This year I got selected for [Google Summer of Code with Jenkins](https://www.jenkins.io/blog/2023/05/17/vandit1604-introduction-blog-post/). This post is the honest story of the gap between those two sentences, because the way in is not what most people think.

If you're a student staring at GSoC thinking "those people are geniuses, I could never," I was you eleven months ago. Here's what actually worked.

## The lie I believed

I thought the path was: get good, *then* contribute. Grind LeetCode, learn everything, build up to a level where I'd be "allowed" to touch a real open source project. So I kept waiting to feel ready.

That's the lie. Nobody feels ready. The path is the other way around: **you contribute in order to get good, not after.** The project teaches you. You just have to show up while you're still bad at it.

<aside class="callout callout--tip" data-label="The reframe">
GSoC is not a reward for already being an expert. It's a paid apprenticeship. Orgs are looking for people who show up, ask decent questions, and stick around, not people who arrive already knowing everything. Being a beginner is the entire point.
</aside>

## What I actually did: show up early, at the edges

I started contributing to [Jenkins in July 2022](https://contributors.jenkins.io/pages/contributors/vandit-singh/), almost a year *before* GSoC applications. Not with anything clever. I picked up small, unglamorous work around the [Jenkins infrastructure](https://www.jenkins.io/projects/infrastructure/) and the docs, the kind of tasks more experienced people skip because they're not exciting.

And that boring work is exactly what taught me the project. To fix a doc you have to understand the thing. To touch the CI you have to understand how it builds. By the time applications opened, I wasn't a stranger sending a cold proposal. I was a name the maintainers already recognized, because I'd been around for months. That recognition is 80% of the game and almost nobody talks about it.

## The project I pitched

My [GSoC 2023 project](https://summerofcode.withgoogle.com/archive/2023/projects/5B6EgSAn) is "Building jenkins.io with alternative tools": rebuilding the Jenkins website's tooling using [Antora](https://antora.org/) and [Gatsby](https://www.gatsbyjs.com/) instead of the existing stack. The Jenkins docs serve a genuinely huge audience, so "modernize the thing millions of developers read" is a real, scoped, useful problem. Not a toy.

Here's the thing though: I could pitch that project *because I'd already been working next to it.* I knew where the pain was. My proposal wasn't a guess, it was "here is a problem I have personally hit, and here is how I'd fix it." Proposals written from real familiarity read completely differently from proposals written from the outside. Mentors can tell instantly.

## If you want to do this next year

The concrete playbook, minus the mystique:

- **Start now, not in application season.** Pick an org months early. Being a familiar face beats a polished cold proposal every time.
- **Go for the boring tasks.** Docs, tests, infra, tooling. They're open, they're low-stakes, and they teach you the codebase's shape faster than anything else.
- **Be visible and be kind.** Ask questions in the public channels. Answer easier ones when you can. Mentors are choosing a *person to work with for a summer*, not a resume.
- **Pitch a problem you've actually felt.** The best proposal is "I keep hitting this, here's my fix," not "here's a feature I googled."

<aside class="callout" data-label="Note">
I came in with weak Java, no real CI/CD, and a shaky grasp of DevOps. None of that disqualified me, because I picked an org where I could learn in the open and contribute at my level while I leveled up. Pick the org that fits where you are now, not where you wish you were.
</aside>

## TL;DR

- You contribute to *get* good, not after you're good. Nobody feels ready. Start anyway.
- Show up **months before** applications. Familiarity with the maintainers beats a slick cold proposal.
- Work the **edges** (docs, infra, tooling). It's the fastest way to learn the project and the easiest place to get merged.
- Pitch a **problem you've personally hit.** It reads a hundred times more credible than an outside idea.

I was underqualified on paper and it worked out, because GSoC rewards showing up over showing off. If you've been waiting to feel ready: stop. Go open one small PR this week.

## Go deeper

- The [Jenkins.io post introducing me as a GSoC 2023 contributor](https://www.jenkins.io/blog/2023/05/17/vandit1604-introduction-blog-post/), and the [GSoC midterm recap](https://www.jenkins.io/blog/2023/07/22/gsoc-2023-midterm/)
- My [Jenkins contributor spotlight](https://contributors.jenkins.io/pages/contributors/vandit-singh/) and [author page](https://www.jenkins.io/blog/authors/vandit1604/)
- The official [Google Summer of Code site](https://summerofcode.withgoogle.com/) and [Jenkins GSoC program](https://www.jenkins.io/projects/gsoc/)

---

*Fun fact: the year after this, I came back to Jenkins GSoC as a [mentor](https://www.jenkins.io/projects/gsoc/2024/projects/implementing-ui-for-jenkins-infra-statistics/). The gap between "too scared to open a PR" and "the person reviewing yours" was about two years and a pile of small, finished contributions. That's it.*
