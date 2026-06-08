---
title: "How Prometheus Finds Matching Series in Milliseconds"
date: "2026-06-09"
tags: ["tech", "prometheus"]
---

Okay so picture this. You ask Prometheus for one specific label combo. Behind the scenes it is sitting on ten million time series. And somehow the answer shows up in a few milliseconds. The wild part? It never actually looked at all ten million to find your answer.

That gap between how much data there is and how cheap the query is, that is the whole story. Let me break down how it pulls this off, starting from the very bottom.

## First, what even is a series

Let us not assume anything. In Prometheus, a single time series is just a bag of labels with an ID number stuck to it. That is it. Labels describe the thing, the ID is its name tag.

```
{job="api", pod="web-1"}  →  #1
{job="api", code="500"}   →  #2
{job="worker"}            →  #3
{job="api", code="500"}   →  #4
```

So when you run a query, the database has exactly one job: take your label conditions, hand back the set of IDs that match. Everything after that (grabbing the actual numbers, doing the math) only happens once you already have that set of IDs. So the real question is just, how do we get from labels to IDs fast.

## The obvious way, and why it flops

The first idea anyone has is to store it forward. Series points to its labels. To answer `{job="api"}` you walk through every single series and check, does this one have job equals api? Yes? Keep it. No? Skip it.

This works. It is also dead on arrival at scale. You are checking every series on every query, so the cost grows with the total number of series. Ten million series means ten million checks for one tiny question. Nobody got time for that. So Prometheus flips the whole thing around.

## The inverted index, aka the glow up

Think about the index at the back of a textbook. You do not read all 600 pages to find where "mitochondria" shows up. You flip to the back, find the word, and it tells you the exact pages. Prometheus does the same move. Instead of storing series to labels, it stores label to "which series have this label".

![Forward index flipped into an inverted index](/static/images/blog/01-inverted-index-flip.svg)

Each label and value pair points to a sorted list of series IDs. That little list has a name, it is called a postings list. And here is the clean part. The index is basically a hashmap with `label="value"` as the key. So going from `job="api"` to `[1, 2, 4]` is a single instant lookup. You scan zero series. You just look up a key and the answer is already sitting there. Big difference from checking ten million things one by one.

## Queries are just set math now

Once every label gives you a sorted list of IDs, a query with multiple labels is literally just combining those lists. No magic.

![Resolving a two-label query by intersecting postings lists](/static/images/blog/02-intersection.svg)

Say you want `job="api"` AND `code="500"`. You grab both lists and find the IDs that show up in both. Because the lists are already sorted, you do not need to do anything fancy. You walk both at the same time with two fingers, like comparing two sorted decks of cards. If both fingers point at the same number, that is a match. If not, move the finger that is behind. One clean pass, no nested loops, no re-sorting. Easy.

And it turns out every PromQL matcher is just one of three set operations on these same sorted lists:

![PromQL matchers mapped to intersect, union, and subtract](/static/images/blog/03-matchers-as-set-ops.svg)

AND is "in both lists" (intersect). OR is "in either list" (union). NOT is "in the first but not the second" (subtract). That is the entire vibe. Sorted lists make all three a single linear walk.

## Regex is where it gets sus

Everything so far is fast because the key already exists in the index. You ask for `job="api"`, the index literally has a key called `job="api"`, boom, instant.

Regex breaks that. If you search `pod=~"web.*"`, there is no key called `web.*` anywhere. That is not a real value, it is a pattern. The index only knows the actual values that genuinely exist. So now Prometheus has no shortcut. It has to go through every single value of that label and test each one against your pattern.

![A regex matcher scanning every value of a label](/static/images/blog/04-regex-scan.svg)

For every value that matches, it pulls that value's postings list, then unions them all together at the end. Here is the catch that bites people. The cost depends on how many different values that label has, not on how picky your regex is. If a label has 100,000 distinct values, that is 100,000 pattern tests on every query, even if only two of them match. This is exactly why high cardinality labels (labels with a ton of unique values) plus regex is the classic way to nuke your Prometheus performance. It is a trap and people fall in it constantly.

## The one trick that saves you sometimes

Not every regex pays this tax though. Prometheus looks at your pattern first and goes, wait, is this actually a regex or is it secretly just a list of options? Patterns like `foo|bar` or `web-1|web-2` are just "one of these exact strings". So Prometheus quietly turns them back into normal exact lookups (this happens in a function called `FindSetMatches`). No scanning every value, just a union of instant lookups. W move.

But a real open ended pattern like `web.*x.*y` gets no shortcut and still has to walk everything. So `pod=~"web-1|web-2"` is cheap, while `pod=~"web.*"` is not, even though they look like cousins. Lowkey good to keep in mind when you write queries.

## TL;DR

- Store labels flipped (inverted): each `label="value"` points to the series that have it.
- Keep every list sorted, so combining lists is one quick walk.
- Turn queries into set math: AND, OR, NOT become intersect, union, subtract.

Exact match is a lookup, instant. Regex is a scan, potentially expensive. Three simple ideas stacked together is what lets you query ten million series in milliseconds. And the same three ideas straight up tell you ahead of time which queries are gonna be slow. Once it clicks, it clicks.

---

*Fun fact, this same structure runs most search engines. Lucene (and therefore Elasticsearch) works the exact same way. If you wanna see the real code, Prometheus keeps it in the TSDB `index` package. Go read it, it is not as scary as it sounds.*
