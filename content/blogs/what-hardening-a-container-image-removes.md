---
title: "What Hardening a Container Image Actually Removes"
date: "2025-03-11"
description: "Most CVEs in your container image aren't in your code. Hardening 12+ OSS images (including IronBank/DoD) taught me the real move: cut CVEs ~80% by removing packages you never used, not by patching."
tags: ["infra", "security"]
---

Here's the fact that reframes container security once it lands: **most of the CVEs a scanner screams about in your image are not in your code.** They're in the operating system packages you inherited from your base image and never once use at runtime. Your Go binary doesn't need `bash`, `apt`, `perl`, `curl`, a full glibc userland, or the seven libraries those drag in, but if they're in the image, every vulnerability in them is now *your* vulnerability as far as the scanner is concerned.

I spent a chunk of time at RapidFort hardening open-source container images, more than a dozen of them, including images that had to meet IronBank / DoD standards, and the headline result was cutting CVE counts by roughly 80%. The thing nobody tells you up front is *how*. You don't patch your way to that number. You **remove** your way to it. This post is what hardening actually does, why the removal is legitimate and not scanner-gaming, and the real costs it comes with.

## Where the CVEs actually live

Pull a normal application image and scan it with [Trivy](https://trivy.dev/) or Grype. You'll get a wall of findings. Read them closely and a pattern jumps out: the overwhelming majority are attached to OS packages, not your application. A Node app on `node:latest` ships a complete Debian userland. A Python app on `python:3` ships the same. That's a shell, a package manager, coreutils, compression libraries, TLS libraries, sometimes a compiler toolchain, plus their transitive dependencies. Every one of those is code maintained by someone else, scanned against the same CVE feeds, and counted against your image.

So the image is mostly not your program. It's a small application sitting on top of a full Linux distribution that came along for the ride. And a full Linux distribution has a lot of surface for a CVE database to hit. Your 40MB of app is buried under a few hundred megabytes of things you're being held accountable for.

## The core move: the most secure package is the one that isn't there

Hardening is, at its heart, one idea applied relentlessly: **remove everything the application doesn't actually need to run.** Not patch it. Remove it. A CVE in `bash` is completely irrelevant to an image that has no `bash`. You didn't fix the vulnerability, you deleted the code that had it, which is strictly better because now it can't be exploited, can't be a foothold, and can't show up in next week's scan when a new CVE lands in that same package.

This flips the mental model from "keep everything updated" to "ship as little as possible." Every binary you don't include is a vulnerability you'll never have to patch, an attack-surface reduction, and one fewer thing in your [SBOM](https://www.cisa.gov/sbom). The question stops being "how do I fix these 200 CVEs" and becomes "why is any of this in my image at all."

<aside class="callout callout--tip" data-label="The reframe">
Patching is a treadmill: new CVEs land in your dependencies forever, and you chase them forever. Removal is permanent: a package that isn't in the image can never generate another finding. Hardening trades an endless patching task for a one-time subtraction. That's why the CVE count doesn't just drop, it stays down.
</aside>

## The toolkit, roughly in order of impact

None of these are exotic. The craft is in applying them without breaking the thing.

**Start from a smaller base.** The single biggest lever. The ladder runs `full` → `slim` → **distroless** → `scratch`. Google's [distroless](https://github.com/GoogleContainerTools/distroless) images contain your language runtime and nothing else: no shell, no package manager, no coreutils. `scratch` is literally empty, perfect for a static Go binary. The newer option worth knowing is [Chainguard's Wolfi](https://github.com/wolfi-dev) images, built from the ground up to be minimal and near-zero-CVE. Swapping a `node:latest` base for a distroless or Wolfi runtime can erase most of your findings before you've done anything else.

**Use multi-stage builds.** Build in a fat stage with all the compilers, headers, and package managers you need, then `COPY` only the finished artifact into a minimal runtime stage. The build toolchain is where a huge amount of CVE surface hides, and with multi-stage it never ships. Your users get the binary, not the `gcc` that made it.

**Kill the shell and the package manager.** Removing `bash`/`sh` and `apt`/`apk`/`yum` does double duty. It drops their CVEs, and it removes the attacker's toolkit: if someone gets code execution in your container, a distroless image gives them no shell to spawn and no package manager to pull down their tools. This is one of those rare moves that improves the scan number and the actual security posture at the same time.

**Drop root, drop setuid.** Run as a non-root `USER`, and strip setuid/setgid bits off any binaries that keep them, since those are classic local privilege-escalation vectors. Remove docs, man pages, package caches, and test fixtures while you're at it. They're pure dead weight and occasionally a finding.

## What that looks like concretely

Here's the transformation in the smallest honest example. The naive image, a full base with the whole toolchain baked into the shipped layer:

```dockerfile
# before: app + a complete Debian userland + the build toolchain, all shipped
FROM node:20
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
CMD ["node", "server.js"]
```

That final image carries `apt`, `bash`, `npm`, the compilers `node-gyp` pulled in, and a full glibc userland, none of which `server.js` touches at runtime. Now the hardened version, multi-stage so the build junk stays in the builder, and a distroless runtime with no shell and no package manager:

```dockerfile
# stage 1: build with everything you need
FROM node:20 AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev
COPY . .

# stage 2: ship only the runtime and the artifact
FROM gcr.io/distroless/nodejs20-debian12
WORKDIR /app
COPY --from=build /app /app
USER nonroot
CMD ["server.js"]
```

Same app, same behavior. The second image has no `apt`, no `bash`, no build toolchain, and runs as non-root. The scanner findings that were attached to all of that vanish, because all of that is gone. Nothing about `server.js` changed; everything about what surrounds it did.

## Why the number really does drop ~80%

Put those together and the arithmetic is not mysterious. You removed the packages the CVEs were counted against. If 80% of the findings were in the OS userland and the build toolchain, and you ship neither, 80% of the findings are gone. Not suppressed, not marked as false positives, *gone*, because the vulnerable code is no longer in the image. That's the honest mechanism behind the headline number, and it's why a hardened image tends to stay clean as new CVEs get disclosed: the disclosures land in packages you already don't have.

## The honest part: is this just gaming the scanner?

It's the first objection everyone raises, and it deserves a straight answer. Deleting packages to make Trivy show a smaller number *would* be gaming if the packages were actually used, because then you've shipped a broken image that happens to scan well. The line between legitimate hardening and cheating the scanner is exactly one thing: **is the removed package genuinely unused at runtime?**

That's where the real work is, and it's less glamorous than it sounds. You have to prove the image still does its job with the package gone. That means actually running it, exercising the real code paths, and watching for the thing that breaks because some library shelled out to a binary you removed, or loaded a locale file, or needed a CA bundle you cleaned up too aggressively. Hardening that isn't verified by running the workload isn't hardening, it's wishful deletion. The scan number is the easy part; the proof that the image still works is the job.

This is exactly the problem RapidFort's [runtime intelligence](https://www.rapidfort.com/use-case/software-attack-surface) approach automates: instead of guessing what's unused, it profiles what the workload actually loads and touches while it runs, then removes what it never reaches, which is how it reduces attack surface by 60 to 90% without breaking the app. The manual version of this, doing it by hand for one image, is what teaches you why the runtime signal matters. You cannot safely remove a package by reading the Dockerfile. You can only safely remove it by watching the container run and confirming it never needs it. (RapidFort's [resource center](https://www.rapidfort.com/resources/resource-center) has the deeper write-ups on the methodology if you want the vendor-grade version.)

And the final acceptance test is the least clever and most convincing one: **actually use the hardened image.** Not scan it and move on, run it as the real image, in the real workflow, and exercise the things it's supposed to do. If hardening stripped something the app quietly needed, using it is where you find out, immediately and unambiguously, instead of shipping a "secure" image that falls over the first time it hits a code path the scanner never made it touch. A hardened image that nobody ran is a hypothesis, not a result.

<aside class="callout callout--warn" data-label="The cost">
Removing the shell is great for security and annoying for you at 3am. <code>kubectl exec -it pod -- sh</code> to poke around a running container doesn't work when there's no <code>sh</code>. You debug hardened images differently: ephemeral debug containers (<code>kubectl debug</code>) that attach a temporary toolbox to the pod, or a separate, fatter debug image you deploy deliberately. The convenience you're giving up is real. Budget for it instead of being surprised by it.
</aside>

## IronBank, and hardening as a standard rather than a vibe

Some of the images had to meet IronBank requirements. IronBank is the US Department of Defense's hardened-container registry (part of Platform One), and it raises the bar from "make the scanner happy" to "satisfy a documented standard." That means an approved base image, a justification for the packages that remain, continuous scanning, and an accepted-findings process for the CVEs you genuinely can't remove yet (with a reason attached, not a shrug).

Working to that standard changed how I thought about the whole exercise. "Zero CVEs" is not really the goal, because you can't always hit zero and a number alone doesn't prove much. The goal is a **defensible image**: everything in it is there for a reason you can state, everything that's gone was removed because it was unused, and the handful of findings that remain are known, tracked, and justified. That's a stronger claim than a green checkmark, and it's the mindset that survives contact with an actual auditor.

## TL;DR

- **Most CVEs in your image are in OS packages you never use**, inherited from a fat base image, not in your application code.
- Hardening's core move is **remove, don't patch**. The most secure package is the one that isn't in the image. Removal is permanent; patching is a treadmill.
- The toolkit: **smaller base** (distroless / Wolfi / scratch), **multi-stage builds** so the toolchain never ships, **kill the shell + package manager** (drops CVEs *and* the attacker's tools), non-root, no setuid.
- The **~80% CVE cut** is just arithmetic: you deleted the packages the CVEs lived in. And it stays down because new disclosures land in packages you no longer carry.
- It's **not scanner-gaming if the removed package is truly unused**, and proving that (by running the workload) is the actual work.
- Costs are real: **no shell means debugging via `kubectl debug` or a separate debug image.** Standards like **IronBank** push you from "clean scan" to "defensible image," which is the better goal anyway.

## Go deeper

- [Google distroless](https://github.com/GoogleContainerTools/distroless) and [Chainguard / Wolfi](https://github.com/wolfi-dev) images, the two minimal-base families worth knowing
- [Multi-stage builds](https://docs.docker.com/build/building/multi-stage/), the cheapest way to keep the build toolchain out of your runtime
- [Trivy](https://trivy.dev/) and [Grype](https://github.com/anchore/grype), the scanners you're hardening against
- [IronBank / Platform One](https://p1.dso.mil/products/iron-bank), the DoD hardened-image standard
- [RapidFort's software attack surface use case](https://www.rapidfort.com/use-case/software-attack-surface) and [resource center](https://www.rapidfort.com/resources/resource-center), for the runtime-intelligence approach to automating all of the above
- [Reducing attack surface noise with runtime intelligence](https://www.rapidfort.com/blog/reducing-attack-surface-noise-with-runtime-intelligence-a-better-approach-to-cve-management), on separating the CVEs that matter from the ones in code you never run
- [SBOMs](https://www.cisa.gov/sbom), because the smaller the image, the smaller the thing you have to account for

---

*Fun fact: the most satisfying hardening result isn't the CVE number, it's opening a shell in the "before" image, then trying the same thing in the "after" image and getting "no such file or directory." That error message is the whole point. There's nothing there to attack.*
