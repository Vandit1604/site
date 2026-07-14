---
title: "Rebuilding a Site 11 Million People Read, With Antora and Gatsby"
date: "2023-08-18"
tags: ["tech", "opensource"]
---

My [Google Summer of Code 2023 project](https://summerofcode.withgoogle.com/archive/2023/projects/5B6EgSAn) had a title that sounds boring until you sit with it: "Building jenkins.io with alternative tools." Translation: take the documentation site that around 11 million people rely on, and rebuild the machinery underneath it without the readers ever noticing. No new features they'd see. Just a better engine, swapped in mid-flight.

That "mid-flight" part is the whole story. You can't take the Jenkins docs offline for a month while you re-architect them. So this post is about the two tools we reached for, [Antora](https://antora.org/) and [Gatsby](https://www.gatsbyjs.com/), why that specific pairing, and the thing nobody warns you about when you migrate a site this big: the content is the hard part, not the code.

## The problem with a giant docs site

Documentation for a project as old and sprawling as Jenkins isn't one book. It's advisories, guides, plugin docs, pipeline step references, event pages, author contributions, upgrade notes. Written by hundreds of people over many years. Living in different places, different formats, different assumptions.

The old tooling worked, but "works" and "pleasant to maintain and evolve" are different sentences. The goal was a stack where docs are **versioned properly**, **modular**, and rendered by something modern enough that contributors in 2023 actually enjoy touching it. That's where the two-tool split comes in.

<aside class="callout callout--tip" data-label="The split">
Antora's job is to be the librarian: it treats docs as versioned, structured content pulled from git and turns them into a clean, navigable model. Gatsby's job is to be the storefront: it takes content and renders a fast, modern, React-powered site. One owns the content pipeline, the other owns the presentation. Keeping those concerns apart is the entire design.
</aside>

## Why Antora for the docs

Antora is built for exactly one thing: multi-repo, multi-version technical documentation written in AsciiDoc. The reason it fits Jenkins is versioning. A project like this needs "here are the docs for the version you're actually running," and Antora treats versions as a first-class idea instead of a folder-naming convention you pray everyone respects.

The magic is that a whole documentation site is described by one small playbook file that points at content sources in git:

```yaml
content:
  sources:
    - url: https://github.com/some-org/some-docs
      branches: [main, v2.x, v1.x]
      start_path: docs
ui:
  bundle:
    url: ./ui-bundle.zip
```

Antora clones those sources, reads the structured content, resolves cross-references between pages *across versions and repos*, and hands you a coherent model. You stop hand-maintaining a navigation tree. The structure comes from the content itself. For a docs set the size of Jenkins', that's the difference between sustainable and not.

## Why Gatsby for the site

[Gatsby](https://www.gatsbyjs.com/) is a React-based static site generator with a GraphQL data layer. That GraphQL part is the reason it earns its place here. In Gatsby, every content source, the Antora-built docs, Markdown pages, JSON, gets pulled into a single unified data graph, and you query the exact slice each page needs:

```graphql
query {
  allMarkdownRemark(sort: { frontmatter: { date: DESC } }) {
    nodes {
      frontmatter { title date }
      fields { slug }
    }
  }
}
```

Then Gatsby renders everything to static HTML at build time, so the site is fast and cheap to serve (no server rendering per request), while contributors still get to work in React components instead of hand-written templates. The full rebuilt site lives in [Vandit1604/jenkins-io-docs](https://github.com/Vandit1604/jenkins-io-docs), and the big merge that brought it into the official infra is [docs.jenkins.io#106](https://github.com/jenkins-infra/docs.jenkins.io/pull/106).

<aside class="callout" data-label="Static, but modern">
"Static site" used to mean "boring and hand-written." Gatsby's pitch is: author in React, query with GraphQL, ship plain fast HTML. You get the developer experience of a modern app and the performance and hosting simplicity of static files. For docs, which are read far more than they change, that trade is close to ideal.
</aside>

## The part that actually eats your summer

Here's what I did not expect. The interesting engineering (wiring Antora into Gatsby, building the data pipeline, the components) was maybe 40% of the work. The other 60% was **the content itself.** Advisories, guides, author pages, event info, pipeline step docs, upgrade guides. When you move a site with this much history, every category of content has its own quirks, its own edge cases, its own "well, this one page does it differently for a reason nobody remembers."

You cannot rebuild the engine and quietly drop half the cargo. Every kind of page had to keep working, keep its URLs, keep making sense. That's the unglamorous truth of any real migration: the code is finite and knowable, but the content is a decade of accumulated decisions, and you have to honor all of them.

<aside class="callout callout--warn" data-label="Migration reality">
The scary metric on a site migration isn't lines of code, it's URLs. Break an old link and you've broken it for everyone who ever bookmarked it, every search result, every blog that linked to you. "Rebuild the tooling" quietly contains "and don't break a single one of the thousands of existing paths." Plan the content and the redirects before you fall in love with the new stack.
</aside>

## What I took from it

Rebuilding infrastructure that's actively in use, for an audience this size, taught me a kind of engineering humility you don't get from greenfield projects. Nobody claps when a migration goes well. Success looks like *nothing happening* from the reader's side. The site just quietly gets better to maintain, and the 11 million people never know there was surgery.

That's a good chunk of what real infrastructure work is. Do the hard thing carefully enough that no one notices you did it.

## TL;DR

- GSoC 2023: rebuild the tooling behind [jenkins.io docs](https://www.jenkins.io/) (11M+ users) without users noticing. New engine, same flight.
- **Antora** owns the content: versioned, multi-repo AsciiDoc turned into a structured model from one small playbook. Versioning is first-class.
- **Gatsby** owns the site: React + a GraphQL data layer, everything rendered to fast static HTML at build time.
- The code was the *easy* 40%. The **content** (advisories, guides, upgrade docs, a decade of edge cases) and not breaking a single URL was the real work.
- Good infra work is invisible. Success is the reader noticing nothing.

## Go deeper

- The rebuilt site: [Vandit1604/jenkins-io-docs](https://github.com/Vandit1604/jenkins-io-docs), merged via [docs.jenkins.io#106](https://github.com/jenkins-infra/docs.jenkins.io/pull/106)
- [Antora](https://antora.org/) and [Gatsby](https://www.gatsbyjs.com/) docs, if you want to try this pairing yourself
- The [Jenkins GSoC program](https://www.jenkins.io/projects/gsoc/), and how I ended up here in the first place, over in [my GSoC origin story](/blogs/how-i-got-into-gsoc-jenkins)

---

*Fun fact: the most satisfying moment wasn't shipping a feature. It was watching an old, deeply-linked docs URL resolve correctly on the new stack. Nobody will ever thank you for a redirect that works. You just have to enjoy it quietly.*
