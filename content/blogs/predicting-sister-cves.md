---
title: "Sister CVEs: Predicting the Vulnerabilities You Haven't Found Yet"
date: "2025-06-24"
description: "A scanner tells you which CVEs you have. We built a service that predicts the ones you probably also have but haven't found: ExploitDB co-occurrence mapping, Gemini for analysis, shipped as a Kubernetes microservice."
tags: ["security", "tech"]
---

A vulnerability scanner answers one question: which known CVEs are in this thing right now? That's useful and it's also reactive. It only ever tells you about what's already been detected. The question a security team actually loses sleep over is the next one: *what am I likely to have that hasn't shown up yet?* What's the vulnerability an attacker finds next, in the same component, that my current scan didn't flag?

At RapidFort I built a service to take a swing at that question, an AI-assisted **sister-CVE predictor**. The idea: given the CVEs a product already has, predict the *related* ones it's likely to have too. It used [ExploitDB](https://www.exploit-db.com/) as the ground truth, Google's Gemini API as the analysis layer, and shipped as a Kubernetes microservice inside our AI CVE-prediction feature. It landed well with the team and, more importantly, with customers, because it turned a flat list of findings into a head start. This post is how it works, the analogy that makes it click, and the honest guardrails, because "AI predicts your vulnerabilities" is exactly the kind of sentence that should make an engineer suspicious.

## The insight: CVEs travel in packs

The whole thing rests on one observation: vulnerabilities are not independent, uniformly-distributed events. They cluster. If a product has a particular CVE, there is often a specific set of *other* CVEs that tend to show up alongside it, because they share a component, a version lineage, a class of bug, or an exploit family. A vulnerable version of some library rarely has exactly one problem; it has a neighborhood of them. The CVE you detected and the three you didn't are frequently siblings from the same broken code.

If you've ever seen "customers who bought this also bought that," you already understand the core algorithm. This is **market-basket analysis** applied to vulnerabilities: products that have CVE A also tend to have CVEs B and C. The retail version mines purchase baskets to find products that co-occur. The security version mines vulnerability data to find CVEs that co-occur. Same shape of problem, wildly higher stakes on getting the recommendation right.

<aside class="callout" data-label="The reframe">
A scanner is a detector: it reports what it can confirm is present. A sister-CVE predictor is a recommender: it reports what is <em>statistically likely</em> to be present given what was confirmed. Those are different jobs with different guarantees, and keeping them clearly separated is the whole ethical spine of the feature. One states facts; the other states informed bets, and it must never dress a bet up as a fact.
</aside>

## The data: ExploitDB as ground truth

You can't predict co-occurrence without real data about which vulnerabilities actually appear together, and that data has to be grounded in something authoritative, not vibes. [ExploitDB](https://www.exploit-db.com/) is a large public archive of exploits, each mapped to the CVEs and the affected products it targets. It's maintained, it's real, and crucially it ties exploits to specific products and versions.

That mapping is the raw material. By working through ExploitDB, you can build the association: for a given product, which CVEs are documented against it, and therefore which CVEs tend to travel together in the same product. From "this product has CVE A" you can derive "products like this one also carried CVEs B, C, D." The prediction isn't conjured from nothing; it's a pattern read out of a real corpus of exploits and the products they hit.

## The AI layer: Gemini for analysis, on a leash

Here's where it gets modern and where it gets dangerous. The raw co-occurrence data is a starting point, but the interesting work is analysis: reading the exploit descriptions, understanding *why* certain CVEs relate, clustering them by the component or bug class they share, and ranking which predicted siblings are actually plausible for a given target versus which are coincidental noise. That's language-heavy, judgment-heavy work, and it's what we used the **Gemini API** for. The LLM reasons over the exploit text and the relationships in a way a plain frequency count can't.

But an LLM analyzing vulnerabilities has one failure mode that would sink the entire feature: **hallucinating CVEs that don't exist.** If Gemini confidently invents `CVE-2023-99999` as a "likely sibling," you've shipped a security tool that fabricates threats, which is worse than useless because it burns the customer's time and, once caught, their trust. So the model was kept on a short leash. It's an analysis and ranking layer over real ExploitDB and CVE data, not a source of truth. Every CVE it surfaces has to reconcile against actual CVE records; a predicted identifier that doesn't correspond to a real, published CVE gets dropped, not shown. The LLM is allowed to reason about relationships between real vulnerabilities. It is not allowed to make up vulnerabilities.

<aside class="callout callout--warn" data-label="The guardrail">
The moment you put an LLM near security findings, its confident fluency becomes a liability. A model will happily produce a well-formatted, authoritative-sounding CVE ID that is completely fictional. The only safe posture is to treat the model's output as claims to be verified against ground truth, never as the ground truth itself. Ground it, verify every identifier against real CVE data, and drop anything that doesn't check out. Fluency is not accuracy, and in security that gap is the whole ballgame.
</aside>

## Shipping it: a microservice, not a notebook

A prediction that lives in a data scientist's notebook helps nobody. This shipped as a real **Kubernetes microservice** (Python), part of the platform's AI CVE-prediction feature, so it was a live endpoint other services and the product could call, not an offline experiment. Making it a deployed microservice forced the engineering questions that a notebook lets you dodge:

- **The LLM is an external dependency with latency and cost.** Every Gemini call takes time and money, so you cache aggressively. CVE relationships don't change second to second; a product's likely siblings are stable enough that you compute and store them rather than re-asking the model on every request.
- **It has to degrade gracefully.** If the Gemini API is slow or rate-limited, the service can't hang the whole feature. The prediction is an enhancement, so a timeout should return the confirmed findings without the predicted ones, not fail the request.
- **The output is an API contract.** Predictions had to come back in a stable, structured shape that the rest of the product could render and reason about, with the confirmed-versus-predicted distinction preserved in the data, not just in the UI copy.

None of that is glamorous, and all of it is the difference between a clever idea and a feature customers actually use.

## Why it landed

The reception was the validating part: the team liked it, and customers responded to it, because it changed the emotional shape of the output. A raw CVE list says "here is your problem, good luck." A sister-CVE prediction says "here is your problem, *and here is where to look next before someone else does.*" It's the shift from reactive to proactive, from "what did we find" to "what should we check." Security teams are drowning in findings; a tool that helps them *prioritize where to look* is worth more than one that just lengthens the list.

It's worth being honest about what earns that trust: the feature is only valuable because it's careful. If it fabricated CVEs or cried wolf, it would have been switched off in a week. The applause came from it being a grounded, verified, well-scoped bet, presented honestly as a prediction. The care is not separate from the value; the care *is* the value.

## The honest part: it's a head start, not a verdict

The line I'd never let anyone blur: a predicted sister CVE is not a confirmed finding. It's a probabilistic hint that says "given what you have, this is statistically worth checking." Treating it as a detection would be dishonest and would eventually be wrong in a way that costs someone. So the framing everywhere, in the API, in the UI, in my own head, was prioritization: this is the industry's direction of travel generally, the same instinct behind [EPSS](https://www.first.org/epss/) (the Exploit Prediction Scoring System), which predicts the *probability a vulnerability will be exploited* rather than pretending to certainty. Prediction earns its place as a way to point attention, not to replace verification. You still confirm. The predictor just tells you where to point the scanner next.

## TL;DR

- A scanner is **reactive**: it reports the CVEs you already have. The higher-value question is which ones you're **likely to have but haven't found**, the "sister" CVEs.
- CVEs **co-occur**: same component, version, or bug class means they travel in packs. Predicting them is **market-basket analysis** ("products with CVE A also had B and C") applied to vulnerabilities.
- Ground truth was **ExploitDB**, which maps exploits to CVEs and products, so the co-occurrence patterns come from real data, not guesswork.
- **Gemini** did the analysis and ranking, kept strictly on a leash: it reasons over real CVEs, and every identifier is **verified against real CVE data** so the model can't hallucinate fictional threats.
- Shipped as a **Kubernetes microservice** (Python) in the AI CVE-prediction feature, with caching (LLM calls are slow and cost money) and graceful degradation (predictions enhance, never block).
- The framing that keeps it honest: a prediction is a **head start, not a verdict.** It tells you where to look next, in the spirit of EPSS. You still confirm.

The thing I took away: "AI for security" is only as good as its discipline about ground truth. The interesting engineering wasn't the model, it was the leash, the verification, and the honesty of calling a prediction a prediction. Get those right and a well-scoped bet becomes something a security team is genuinely glad to have.

## Go deeper

- [ExploitDB](https://www.exploit-db.com/), the exploit-to-CVE-to-product archive that grounds the whole thing
- [EPSS](https://www.first.org/epss/), the industry effort to predict exploitation probability, the honest cousin of this idea
- [Market-basket analysis / association rule mining](https://en.wikipedia.org/wiki/Association_rule_learning), the "customers who bought X also bought Y" technique underneath
- [The Gemini API](https://ai.google.dev/), the analysis layer, and why grounding + verification matter the moment an LLM touches security data

---

*Fun fact: the mental model that unlocked this for me was realizing a vulnerability scanner and a shopping recommender are the same program pointed at different data. "People who had this CVE also had these" is just "people who bought this also bought these" with much higher stakes and a much stricter requirement to never make anything up.*
