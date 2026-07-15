---
title: "How Prometheus Finds Matching Series in Milliseconds"
date: "2026-06-09"
description: "How Prometheus finds the series matching your query in milliseconds: inverted indexes, posting lists, Seek-based set intersection, and why negative matchers and regex are the expensive part."
tags: ["tech", "prometheus"]
---

Picture the situation. You ask Prometheus for one specific label combination. Behind the scenes it is sitting on ten million active time series. The answer comes back in a few milliseconds. The interesting part is that it never looked at all ten million series to find it, and it decided *which* series to skip without reading them either.

That gap, between how much data exists and how little of it a query actually touches, is the whole story. I went digging through the TSDB source to understand how it pulls this off, and it turns out to be three ideas stacked on top of each other, plus a couple of details in the real code that are more clever than the textbook version. Let me build it up from the bottom, and I'll link the actual Prometheus code as we go so none of this is hand-waving.

## First, what even is a series

Let's not assume anything. In Prometheus, a single time series is just a set of labels with an integer ID attached. The labels describe the thing, the ID is its internal name tag.

```
{job="api", pod="web-1"}  →  #1
{job="api", code="500"}   →  #2
{job="worker"}            →  #3
{job="api", code="500"}   →  #4
```

So when you run a query, the index has exactly one job: take your label conditions and hand back the set of IDs that match. Everything after that, grabbing the actual samples and doing the PromQL math, only happens once you already hold that set of IDs. The whole performance question reduces to: how do you get from labels to IDs fast?

## The obvious way, and why it falls over

The first design anyone reaches for is a forward mapping: each series points to its labels. To answer `{job="api"}`, you walk every series and check whether it has `job="api"`. Keep the matches, skip the rest.

This works, and it dies immediately at scale. You are checking every series on every query, so the cost grows with the *total* number of series, not with how many actually match. Ten million series means ten million checks to answer one small question. It's the database equivalent of `grep` with no index: fine on a small file, hopeless on a huge one. This is also why relational databases build B-tree indexes instead of scanning tables, and Prometheus makes the same move, just with a structure tuned for its specific shape of data.

## The inverted index

Think about the index at the back of a textbook. You don't read all 600 pages to find where "mitochondria" appears. You flip to the back, find the word, and it lists the exact pages. Prometheus does precisely this. Instead of mapping series to labels, it maps each `label="value"` to the list of series that have it.

![Forward index flipped into an inverted index](/static/images/blog/01-inverted-index-flip.svg)

That list of series IDs has a name: a **postings list**. In the in-memory head block, the whole structure is a `MemPostings`, which is essentially a `map[labelName]map[labelValue][]SeriesRef`, a nested map from label name to value to a sorted slice of series references. Going from `job="api"` to `[1, 2, 4]` is a couple of map lookups and you're holding the answer. You scan zero series. This is the exact same data structure that powers Lucene, and therefore Elasticsearch and every search box built on it. Inverted indexes are one of those ideas that quietly runs half the software you touch.

There's a detail that matters later: those slices are kept **sorted** by series ID, and Prometheus works to keep them that way (there's a whole `EnsureOrder` step that sorts postings across worker goroutines). Sorted is not an accident. Sorted is the thing that makes the next part cheap.

## Queries become set math

Once every label gives you a sorted list of IDs, a query with multiple conditions is just combining those lists.

![Resolving a two-label query by intersecting postings lists](/static/images/blog/02-intersection.svg)

Say you want `job="api"` AND `code="500"`. Grab both postings lists and find the IDs present in both. The naive mental model is two fingers walking two sorted lists: if both fingers point at the same ID, it's a match; otherwise advance whichever finger is behind. One linear pass, no nested loops, no re-sorting.

That intuition is right, but the real code is smarter than a plain two-finger walk, and the difference is the whole reason a rare label stays cheap even against a common one. Here's what `intersectPostings.Next` actually does: it advances every list once, takes the *maximum* current ID as a target, and if the lists don't already agree, it calls `Seek(target)` on all of them, which makes each list **jump forward** to the first ID at or past the target rather than stepping one at a time.

```go
func (it *intersectPostings) Next() bool {
    if !it.postings[0].Next() {
        return false
    }
    target := it.postings[0].At()
    // ... pick the max At() across all lists as the new target ...
    return it.Seek(target)  // every list jumps ahead to >= target
}
```

<a class="src-link" href="https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/tsdb/index/postings.go#L643-L667" target="_blank" rel="noopener noreferrer">↗ tsdb/index/postings.go · intersectPostings</a>

And on disk, that `Seek` is a **binary search**, not a linear scan. Persistent postings are stored as fixed-width big-endian `uint32` series refs, so `bigEndianPostings.Seek` can do a `sort.Search` (binary search) straight to the target offset:

```go
func (it *bigEndianPostings) Seek(x storage.SeriesRef) bool {
    // ... binary search over 4-byte big-endian refs ...
    i := sort.Search(num, func(i int) bool {
        return binary.BigEndian.Uint32(it.list[i*4:]) >= uint32(x)
    })
    // ...
}
```

<a class="src-link" href="https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/tsdb/index/postings.go#L911-L930" target="_blank" rel="noopener noreferrer">↗ tsdb/index/postings.go · bigEndianPostings.Seek</a>

Why does this matter? Because if you intersect a tiny list (say `code="500"`, ten series) with a giant one (`job="api"`, a million series), the intersection is driven by the tiny list, and for each of its ten IDs it binary-searches the million-entry list instead of walking it. Ten binary searches over a million entries is nothing. The fixed-width big-endian encoding exists specifically to make that random-access binary search possible. Sorted lists plus `Seek` is the mechanism; the two-finger picture is just the friendly version of it.

## The three set operations

Every PromQL matcher, once you're down at this layer, is one of three set operations on these sorted lists.

![PromQL matchers mapped to intersect, union, and subtract](/static/images/blog/03-matchers-as-set-ops.svg)

`AND` is "in both lists" (intersect). `OR` is "in either list" (union, `Merge` in the code). `NOT` is "in the first but not the second" (subtract, `Without`). All three are a single linear-ish walk over sorted inputs. But `NOT` hides a question the other two don't: subtract from *what*? "Series that don't have `code="500"`" only makes sense relative to some base set of everything.

Prometheus answers this with a special postings list keyed by the **empty label** `{}`. Every single series, when it's added to the index, registers itself under that empty key as well as under each of its real labels:

```go
func (p *MemPostings) Add(id storage.SeriesRef, lset labels.Labels) {
    // ... register id under each real label ...
    p.addFor(id, allPostingsKey) // and under the empty "all postings" key
}
```

<a class="src-link" href="https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/tsdb/index/postings.go#L403-L421" target="_blank" rel="noopener noreferrer">↗ tsdb/index/postings.go · Add</a>

So there is always a postings list of *every* series ID, called the all-postings list, and `NOT` is just that base minus the matching list.

## The part that's trickier than AND: negative matchers

This is where the real code earns its keep, and it's the bit most explanations skip. Consider `{job="api", instance!="host-1"}`. You can't just intersect two lists, because `instance!="host-1"` also has to include series that have *no `instance` label at all*. Absence matches a negation. So a negative matcher can't be an intersection; it has to be a subtraction from a larger base.

`PostingsForMatchers` handles this by splitting your matchers into two groups: **intersecting** ones (the base you build up) and **subtracting** ones (the negations and anything that matches the empty string). It computes the intersection of the positive matchers first, then applies `Without` for each negative one. Two nice details fall out of the source:

- It **sorts the intersecting matchers to run first**, deliberately, so the base you subtract from is as small as possible before you start removing from it. Subtracting from a small set is cheaper and avoids consistency hazards with series being added mid-query.
- If a query has *only* negative matchers (say `{job!="api"}` with nothing positive), there's nothing to intersect, so it starts from the all-postings list and subtracts. That's the empty-label list from the last section doing its job.

<a class="src-link" href="https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/tsdb/querier.go#L266-L412" target="_blank" rel="noopener noreferrer">↗ tsdb/querier.go · PostingsForMatchers</a>

<aside class="callout" data-label="The subtle one">
A label being <em>absent</em> counts as matching <code>!=</code> and <code>!~</code>. That's why <code>instance!="host-1"</code> returns series with no <code>instance</code> label, and why Prometheus can't treat a negative matcher as a normal intersection. It resolves the positives into a base set and subtracts the negatives. Keep this in mind: adding a <code>!=</code> can quietly force the query onto the all-postings list.
</aside>

## Regex is where it gets expensive

Everything so far is fast because the key already exists in the index. Ask for `job="api"` and the index literally has a key `job="api"`; it's a lookup. Regex breaks that. Search `pod=~"web.*"` and there is no key called `web.*`, because that's a pattern, not a value. The index only knows the concrete values that actually exist.

![A regex matcher scanning every value of a label](/static/images/blog/04-regex-scan.svg)

So Prometheus falls back to `PostingsForLabelMatching`, which walks **every distinct value** of that label and tests each one against your pattern, then unions the postings lists of the values that matched. Here's the trap that catches people: the cost scales with the number of distinct values the label has, not with how selective your regex is. If a label has 100,000 unique values, that's 100,000 pattern tests on every query even if only two values match. This is exactly why **high-cardinality labels plus regex** is the classic way to melt a Prometheus instance. The regex isn't the problem; the cardinality it has to scan is.

## The shortcut that saves you, when it applies

Not every regex pays that tax, and the reason is worth knowing precisely because it changes how you write queries. Before scanning, Prometheus inspects the pattern to see whether it's *secretly a fixed set of strings*. A function called `findSetMatches` walks the parsed regex syntax tree and, for patterns like `foo|bar` or `web-1|web-2`, extracts them back into a plain list of exact strings.

```go
func findSetMatches(re *syntax.Regexp) (matches []string, caseSensitive bool) { ... }
```

<a class="src-link" href="https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/model/labels/regexp.go#L166" target="_blank" rel="noopener noreferrer">↗ model/labels/regexp.go · findSetMatches</a>

When that succeeds, the matcher exposes the strings through `SetMatches()`, and the querier's fast path skips the scan entirely, doing a union of exact lookups instead:

```go
if m.Type == labels.MatchRegexp {
    setMatches := m.SetMatches()
    if len(setMatches) > 0 {
        return ix.Postings(ctx, m.Name, setMatches...) // union of instant lookups
    }
}
```

<a class="src-link" href="https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/tsdb/querier.go#L414-L430" target="_blank" rel="noopener noreferrer">↗ tsdb/querier.go · postingsForMatcher</a>

So `pod=~"web-1|web-2"` is cheap: it becomes two exact lookups unioned together. But `pod=~"web.*"` is an open-ended pattern with no finite set behind it, so it falls all the way back to scanning every value. The two look like cousins and perform nothing alike.

<aside class="callout callout--tip" data-label="The move">
When you can, write anchored alternations of exact values (<code>=~"a|b|c"</code>) instead of open-ended wildcards (<code>=~"a.*"</code>). The first hits the set-match fast path and turns into instant lookups. The second scans every distinct value of the label. Same-looking query, wildly different cost, and now you know why before you ever run it.
</aside>

## Why sorted, and why big-endian

Two representation choices quietly hold the whole thing up. First, postings are always sorted by series ID, which is what makes intersect, union, and subtract single-pass merges and what makes `Seek` a binary search rather than a scan. Second, the on-disk index stores refs as fixed-width big-endian integers, which is what allows random access by index, which is what allows that binary search. The on-disk index also interns every label string into a **symbol table** and refers to strings by offset, so a value like `job="api"` that appears on a million series is stored once, not a million times. None of these are exotic. They're the boring, correct choices that turn a nice idea into something that answers queries over ten million series without breaking a sweat.

## TL;DR

- Store labels **inverted**: each `label="value"` maps to a sorted postings list of the series that have it. Same structure as Lucene/Elasticsearch.
- Queries become **set math**: AND is intersect, OR is union (`Merge`), NOT is subtract (`Without`).
- Intersection isn't a naive two-finger walk. It uses **`Seek` to skip ahead**, and on disk that `Seek` is a **binary search** over fixed-width big-endian refs, so a rare label stays cheap against a common one.
- **NOT needs a base to subtract from**: the all-postings list keyed by the empty label, which every series registers itself into.
- **Negative matchers are subtractions, not intersections**, because an absent label matches `!=`. Prometheus intersects the positives first (smallest base) then subtracts.
- **Exact match is a lookup; regex is a scan** whose cost scales with the label's cardinality. `findSetMatches` rescues finite alternations (`a|b|c`) into exact lookups; open-ended patterns (`a.*`) scan everything.

Three ideas, stacked: invert the index, keep it sorted, turn queries into set operations. That's what lets you query ten million series in milliseconds, and the same three ideas tell you in advance which queries will be slow.

## Go deeper

- The real postings code (intersect, union, subtract, Seek): [`tsdb/index/postings.go`](https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/tsdb/index/postings.go)
- The matcher planner (positive/negative split, ordering): [`tsdb/querier.go`](https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/tsdb/querier.go#L266-L412)
- The regex set-match optimizer: [`model/labels/regexp.go`](https://github.com/prometheus/prometheus/blob/dfb21d0cd7c4987b571adc9137f9f00eb91ead34/model/labels/regexp.go#L166)
- [The TSDB format spec](https://github.com/prometheus/prometheus/blob/main/tsdb/docs/format/index.md) if you want to see the on-disk index layout, symbol table and all

---

*Fun fact: this same structure runs most search engines. Lucene, and therefore Elasticsearch, is built on inverted indexes and postings intersection, the exact moves above. If you want to read the real thing, Prometheus keeps it in the `tsdb/index` and `tsdb` packages. It reads more clearly than you'd expect.*
