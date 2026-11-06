---
title: "NULL vs Empty String vs Missing Field"
date: "2026-11-20"
description: "NULL vs empty string vs missing field: how SQL, JSON, Go, PromQL, LogsQL and TraceQL treat each one, and a real bug where != matched spans with no attribute."
tags: ["tech", "null"]
draft: "false"
---

A value can be in three different states, and most bugs in this area come from treating two of them as one.

I hit the same bug in three different systems in a month. A query said "give me everything that is not `x`", and it also returned the records that had no value at all. Each time the fix was one line, and each time it took me an hour to find, because I kept mixing up three ideas that look alike.

So this post starts from zero: the three states, then SQL (the strictest), then JSON and Go. After that come Prometheus, VictoriaLogs and Tempo, which merge some of these states on purpose. At the end I ran the same six questions on all of them and pasted the real output.

## The three states, with a paper form

Picture a paper form with a field called "Nickname".

1. You write nothing and tick a box that says "I have no nickname". That is **NULL**: a known fact that the value does not exist, or is unknown.
2. You write a pair of quotes, `""`, because you want to say "my nickname is the empty string". That is an **empty string**: a real value that happens to have zero characters.
3. The form you got has no Nickname line at all. That is a **missing field**: nobody ever asked the question.

In code:

| State | SQL | JSON | Go (`encoding/json`) | Prometheus |
|---|---|---|---|---|
| empty | `''` | `{"nick":""}` | `""` | label with value `""` |
| null | `NULL` | `{"nick":null}` | `nil` pointer | does not exist |
| missing | column not selected | `{}` | field not touched | label not set |

Some systems keep all three apart. Some merge two. Some merge all three. A bug appears when you write a query that assumes system A's rules and run it on system B.

> Tony Hoare called the null reference his "billion-dollar mistake" in a 2009 QCon London talk. He added it to ALGOL W in 1965 because it was easy to implement. The problem is older than most of the code that has it.

<img class="meme" src="/static/images/blog/memes/null-vs-empty-vs-missing-1.webp" alt="Reaction image: Disbelief, the numbers do not add up" width="347" height="260" loading="lazy">

## SQL: NULL is "unknown", not "nothing"

Start with the thing that surprises everyone once. Make a table:

```sql
CREATE TABLE users (id int, email text);
INSERT INTO users VALUES (1, 'a@x.com'), (2, ''), (3, NULL);
```

Now ask for the user with no email:

```sql
SELECT id FROM users WHERE email = NULL;
```

```
(0 rows)
```

Zero rows, even though row 3 has a NULL email. `NULL` is not a value, it means "unknown". Is one unknown equal to another unknown? Nobody can say, so `NULL = NULL` is neither true nor false. It is **unknown**, and SQL has a third truth value for that.

This is called three-valued logic. The rules are what you get if you read "unknown" as "I don't know":

| a | b | a AND b | a OR b |
|---|---|---|---|
| true | unknown | unknown | true |
| false | unknown | false | unknown |
| unknown | unknown | unknown | unknown |

`NOT unknown` is unknown. `WHERE` keeps a row only when the condition is **true**, so unknown rows get thrown away, same as false ones. That explains most NULL surprises.

The right test is `IS NULL`:

```sql
SELECT id FROM users WHERE email IS NULL;      -- 3
SELECT id FROM users WHERE email = '';         -- 2
```

If you want "equal, but treat two NULLs as equal", PostgreSQL has `IS NOT DISTINCT FROM`. The [docs](https://www.postgresql.org/docs/17/functions-comparison.html) define `IS DISTINCT FROM` as "Not equal, treating null as a comparable value". So `NULL IS DISTINCT FROM NULL` is false and `1 IS DISTINCT FROM NULL` is true, never unknown.

I ran the small things on PostgreSQL 17 to be sure:

```
SELECT '' IS NULL;                   -- f
SELECT NULL = NULL;                  -- (null)
SELECT NULL IS NOT DISTINCT FROM NULL;  -- t
SELECT length(NULL), 'a' || NULL;    -- (null), (null)
```

A NULL inside almost any expression makes the whole expression NULL. `'a' || NULL` is not `'a'`. It is NULL.

### Empty string vs NULL across databases

In PostgreSQL, `''` and `NULL` are different, as the query above shows. SQLite 3.54.1 agrees (I ran `select '' is null` and got 0).

SQL Server agrees too. Its Docker image will not start on my arm64 Mac, so I used Azure SQL Edge, which runs the same engine (15.0, the SQL Server 2019 line), and ran `SELECT CASE WHEN '' IS NULL THEN 1 ELSE 0 END`:

```
r
-----------
          0
```

Oracle is the odd one. Its SQL Language Reference says: "The database currently treats a character value with a length of zero as null. However, this may not continue to be true in future releases, and Oracle recommends that you do not treat empty strings the same as nulls." So in Oracle `'' IS NULL` is true, and an `INSERT` of `''` stores NULL. I did not run Oracle; I am quoting the [docs](https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/Nulls.html).

<img class="meme" src="/static/images/blog/memes/null-vs-empty-vs-missing-2.webp" alt="Reaction image: Betrayal, a database just did the opposite of every other one" width="680" height="641" loading="lazy">

If you move data from Oracle to PostgreSQL, every empty string in your old data turns into NULL, or the other way round, and your `WHERE col = ''` checks quietly start returning different rows.

### Tools for moving between the states

Two functions do most of the work.

`COALESCE(a, b, ...)` returns the first argument that is not NULL. It turns NULL into a default:

```sql
SELECT COALESCE(nickname, name) FROM users;
```

`NULLIF(a, b)` returns NULL when `a = b`, otherwise `a`. It turns empty into NULL:

```sql
SELECT NULLIF(email, '') FROM users;   -- '' becomes NULL
```

Chain them to treat empty and NULL as the same thing: `COALESCE(NULLIF(email, ''), 'none')`. This pair is the SQL version of what Prometheus and VictoriaLogs do for you without asking.

### The NOT IN trap

This one bites in production. I want the users whose id is not in a list:

```sql
SELECT id FROM users WHERE id NOT IN (1, 2, NULL);
```

```
(0 rows)
```

Row 3 is clearly not 1 and not 2, yet it is gone. `x NOT IN (1, 2, NULL)` expands to:

```sql
x <> 1 AND x <> 2 AND x <> NULL
```

The last term is `unknown`. `true AND true AND unknown` is unknown. `WHERE` drops it. One NULL in the list and the whole condition can never be true.

On a real table the NULL usually comes from a subquery, which makes it worse: `WHERE id NOT IN (SELECT user_id FROM orders)` works for months, then one order row gets a NULL `user_id` and the query returns nothing. No error. The fix is `NOT EXISTS`, which does not do this:

```sql
SELECT u.id FROM users u
WHERE NOT EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id);
```

The planner angle (why `NOT IN` also becomes a slow subplan instead of an anti join) is covered well in [this post on NOT IN and NULL](https://boringsql.com/posts/not-in-null/). The short version is the same: prefer `NOT EXISTS` when the inner side can hold NULL.

<!-- DIAGRAM: truth table for AND/OR/NOT with true/false/unknown, with the row "true AND true AND unknown = unknown" highlighted as the NOT IN expansion -->

## JSON and Go: the same three states, different rules

JSON has all three states, written differently:

```json
{"nick": "van"}
{"nick": ""}
{"nick": null}
{}
```

Value, empty, null, missing. Four inputs, three ideas (the first one is just a normal value). A good API can tell the last three apart. Most Go code cannot, and here is the proof. This is the whole program:

```go
type Plain struct{ Nick string          `json:"nick"` }
type Ptr   struct{ Nick *string         `json:"nick"` }
type Raw   struct{ Nick json.RawMessage `json:"nick"` }

for _, in := range []string{`{"nick":"van"}`, `{"nick":""}`, `{"nick":null}`, `{}`} {
	var a Plain; var b Ptr; var c Raw
	json.Unmarshal([]byte(in), &a)
	json.Unmarshal([]byte(in), &b)
	json.Unmarshal([]byte(in), &c)
	// print a.Nick, *b.Nick (or <nil>), string(c.Nick) (or <nil slice>)
}
```

Output on Go 1.25.4:

```
{"nick":"van"}   string="van"  *string="van"    RawMessage="van"
{"nick":""}      string=""     *string=""       RawMessage=""
{"nick":null}    string=""     *string=<nil>    RawMessage=null
{}               string=""     *string=<nil>    RawMessage=<nil slice>
```

Read it row by row:

- Plain `string` gives `""` for empty, null and missing. Three states, one answer.
- `*string` is better. Empty is `""`, but null and missing are both `nil`.
- `json.RawMessage` keeps the raw bytes, so null (`null`) and missing (`nil` slice) are different. It is the only one of the three that sees all of them.

If your API needs "set this field to null" to differ from "do not touch this field" (a PATCH endpoint is the classic case), a pointer is not enough. Use `map[string]json.RawMessage` for the body, or a small wrapper type that records whether the key was present.

The other half is writing. `omitempty` is a trap because "empty" means different things per type:

```
Ptr{}        {"nick":null}
Ptr{&""}     {"nick":""}
Omit{}       {}                  // *string with omitempty, nil
Omit{&""}    {"nick":""}         // pointer to "" is NOT omitted
OmitStr{}    {}                  // string with omitempty, "" is omitted
```

On a `string` field, `omitempty` drops the empty string, so a receiver cannot tell "empty" from "missing". On a `*string` field it drops only the nil pointer and keeps `""`. Go 1.24 added `omitzero` ([release notes](https://go.dev/doc/go1.24)), which omits a field when it is the zero value or its `IsZero()` method says so. I tested `omitzero` on a `*string`: nil is dropped, a pointer to `""` is kept, same as `omitempty`.

So in Go: a pointer separates empty from "not set", `RawMessage` or a wrapper separates null from missing, and `omitempty` on a plain string erases the difference on the way out.

## Why monitoring tools merge the states on purpose

This part confused me for a month. Databases work hard to keep these states apart. Monitoring tools do the opposite, with a good reason.

Take a Prometheus label. For `{env="prod"}`, the useful question is "which series have env=prod". Nobody wants a series with `env=""` to mean something different from a series that never had an `env` label. My reading: a second state would add nothing to a label query, so Prometheus treats them as the same thing. (I wrote about how labels are indexed in [how Prometheus finds series](/blogs/how-prometheus-finds-series).)

The [querying docs](https://prometheus.io/docs/prometheus/latest/querying/basics/) say it directly: "Label matchers that match empty label values also select all time series that do not have the specific label set at all."

So `{f=""}` means "f is empty **or** f is not set". You cannot ask for one without the other. And since a matcher that matches empty matches everything that lacks the label, a selector must have at least one matcher that does not match empty. I ran that too, on Prometheus 3.16.0:

```
{f=""}        parse error: vector selector must contain at least one non-empty matcher
{job=~".*"}   parse error: vector selector must contain at least one non-empty matcher
```

<img class="meme" src="/static/images/blog/memes/null-vs-empty-vs-missing-3.webp" alt="Reaction image: Forced acceptance, the empty and the missing were the same all along" width="611" height="611" loading="lazy">

VictoriaLogs does the same for log fields. The [LogsQL docs](https://docs.victoriametrics.com/victorialogs/logsql/) say about `host.hostname:""`: "VictoriaLogs treats empty values as non-existing". And `field:*` finds logs that have a non-empty value in that field. Same model as Prometheus: empty equals missing, non-empty is the only real state.

Grafana Tempo's TraceQL has its own spelling. The [docs](https://grafana.com/docs/tempo/latest/traceql/construct-traceql-queries/) say to use `nil` to check for attributes that are missing or null. So `{span.f = nil}` is "missing or null". An empty string is still a real value there, separate from `nil`. I'll show that in the table.

Three systems, three models:

- SQL: empty, NULL and missing are all different. Missing is not even a state of a value, it is a column you did not select.
- Prometheus and VictoriaLogs: empty and missing are the same. NULL does not exist.
- TraceQL: empty is a value. Null and missing share the name `nil`.

All of this works until a query language has to say "not equal to x". That is where my bug was.

## The bug: `!=` that matched spans with no attribute

I work on [VictoriaTraces](https://github.com/VictoriaMetrics/VictoriaTraces), which stores traces and serves them through a Tempo-compatible API. You send TraceQL, like `{span.http.status_code != 200}`, and it returns matching spans. Internally, VictoriaTraces translates TraceQL into LogsQL, because it is built on top of VictoriaLogs' storage.

I sent the same five spans to Tempo 3.1.0 and to VictoriaTraces and compared. The results are in [VictoriaTraces#269](https://github.com/VictoriaMetrics/VictoriaTraces/issues/269). In the issue, `span.istio.cluster_id` is `c1` on span A and `c2` on span C, and spans B, D and E do not have it:

| Query | Tempo 3.1.0 | VictoriaTraces |
|---|---|---|
| `{span.istio.cluster_id != "c1"}` | C | B, C, D, E |
| `{span.istio.cluster_id !~ "c.*"}` | none | B, D, E |
| `{span.http.status_code != 200}` | B, D | B, D, E |

Tempo returns only spans that have the attribute and a different value. VictoriaTraces also returned every span that has no attribute at all. For `!= 200`, span E has no status code, and it came back.

The cause is in the translation. The TraceQL `{span.http.status_code != 200}` became the LogsQL `span_attr:http.status_code:!=200`. In LogsQL a negative filter matches logs where the field is absent too. That is the same "empty equals missing" model as above, and for a log search it is what you want: "show me logs where status is not 200" should include logs with no status. For a trace search, TraceQL says the attribute has to exist.

The fix, in [VictoriaTraces#279](https://github.com/VictoriaMetrics/VictoriaTraces/pull/279) (open, not merged at the time of writing), makes the translation require the field to exist first. In the diff, `requireField` adds `span_attr:f:* and ` in front of a `!=` or `!~` filter on span, resource and instrumentation attributes. Intrinsics are left out, because span kind `0` is not stored, so `{kind != server}` must keep matching a span with no `kind` field. Event and link fields are stored with an index suffix, so `field:*` would never match them.

<a class="src-link" href="https://github.com/VictoriaMetrics/VictoriaTraces/blob/87d9dfb55c8a2fab0a250c4894f05ad36773178d/lib/traceql/filter_general.go#L150" target="_blank" rel="noopener noreferrer">↗ lib/traceql/filter_general.go · requireField</a>

I ran the queries from the issue on Tempo 3.1.0 and on the branch, and they return the same spans (that is also in the PR text).

<img class="meme" src="/static/images/blog/memes/null-vs-empty-vs-missing-4.webp" alt="Reaction image: Calm, the two systems finally agree" width="680" height="460" loading="lazy">

### The mirror: an empty value that storage throws away

While testing #279 I found the opposite problem, filed as [VictoriaTraces#280](https://github.com/VictoriaMetrics/VictoriaTraces/issues/280). VictoriaLogs drops empty fields, so VictoriaTraces stores an empty attribute value as `-` (the insert path does this). Then `{span.foo = ""}` was translated to a filter on the empty string, which matches spans **without** `foo`, and misses the span where `foo` really is `""`. Tempo returns the span with the empty value. The fix PR is [VictoriaTraces#296](https://github.com/VictoriaMetrics/VictoriaTraces/pull/296) (open). It maps the empty value to `-` for `=` and `!=`.

You see the same thing in the experiment below: on VictoriaTraces, `{span.f = ""}` returns span E (no attribute), not span C (empty). Same bug, still there.

### The same model from the other side: `NOT *:""`

A third case, [VictoriaLogs#1919](https://github.com/VictoriaMetrics/VictoriaLogs/issues/1919) (an open issue, no PR yet). Someone in Slack asked how to skip logs where any field is empty. The natural answer is `NOT *:""`, "no field is empty". It returns nothing:

```
$ curl localhost:9428/select/logsql/query -d 'query=NOT *:""'
(no output)
$ curl localhost:9428/select/logsql/query -d 'query=* | limit 10 | filter NOT *:""'
(returns the log)
```

Same filter, same data, different answer depending on whether a pipe is in front of it. Before a pipe, the wildcard filter reads stored columns. The fields `_time`, `_stream` and `_stream_id` have no stored column, so the code takes a fast path and treats them as empty. That makes `*:""` match every log. After a pipe, the rows are materialized and those fields exist, so the result is right.

<a class="src-link" href="https://github.com/VictoriaMetrics/VictoriaLogs/blob/b341d6f86239940e2e7b609dfb857433dae374db/lib/logstorage/filter_phrase.go#L79-L86" target="_blank" rel="noopener noreferrer">↗ lib/logstorage/filter_phrase.go · no column means empty</a>

I reproduced it again for this post on VictoriaLogs v1.53.0:

```
NOT *:""                              (none)
* | limit 10 | filter NOT *:""       D,C,B,E,A     <- every log
```

The obvious fix has a catch: if you make `*` cover those three fields everywhere, then `* | limit 10 | filter *:2026` matches every log from this year, because it now matches the timestamp. It is not clear which side should change, so I left the choice to the maintainers in the issue.

Meanwhile, the query that works is to select the fields you care about first, then filter. That is what I documented in [VictoriaLogs#1934](https://github.com/VictoriaMetrics/VictoriaLogs/pull/1934) (docs only, open): `drop_empty_fields` sounds like the answer but only drops the field, not the log.

```logsql
* | fields id, f | filter NOT *:""
```

## The experiment: six questions, five systems

Now the same data in every system: five records.

| Record | Value of `f` |
|---|---|
| A | `x` |
| B | `y` |
| C | empty string |
| D | explicit null (where the system has it) |
| E | not set at all |

The questions:

1. Q1: `f` is empty.
2. Q2: `f` is missing.
3. Q3: `f` is empty or missing.
4. Q4: `f != "x"`.
5. Q5: `f` has a value.
6. Q6: `f` is not one of `x`, `y`.

How I ran it, all in Docker except the two VictoriaTraces builds:

- PostgreSQL 17: `t(id text, f text)`, D and E are both NULL.
- Prometheus 3.16.0 scraping a tiny text file. Prometheus has no null, so there is no record D. The exposition line for C had `f=""` and the series came back with no `f` label at all.
- VictoriaLogs v1.53.0 with `/insert/jsonline`. D was sent as JSON `null`, C as `""`.
- Grafana Tempo 3.1.0 with the OTLP JSON receiver, one span per record. D is an attribute with an unset value.
- VictoriaTraces built from `master` (b414864) and from the head of #279 (87d9dfb), same five spans over OTLP. The head of #279 is based on an older commit than master, so it is not "master plus the fix", but the difference is small (the PR touches three files).

The queries per system:

| | Q1 empty | Q2 missing | Q3 empty or missing | Q4 `!= x` | Q5 has value | Q6 not in x,y |
|---|---|---|---|---|---|---|
| PostgreSQL | `f = ''` | `f IS NULL` | `f = '' OR f IS NULL` | `f <> 'x'` | `f IS NOT NULL` | `f NOT IN ('x','y')` |
| Prometheus | `m{f=""}` | `m{f=""}` | `m{f=""}` | `m{f!="x"}` | `m{f=~".+"}` | `m{f!~"x\|y"}` |
| VictoriaLogs | `f:""` | `f:""` | `f:""` | `-f:x` | `f:*` | `-f:in(x,y)` |
| TraceQL | `{span.f = ""}` | `{span.f = nil}` | `{span.f = "" \|\| span.f = nil}` | `{span.f != "x"}` | `{span.f != nil}` | `{span.f !~ "x\|y"}` |

And what came back (matched records, real output):

| | PostgreSQL 17 | Prometheus 3.16 | VictoriaLogs v1.53 | Tempo 3.1.0 | VictoriaTraces master | VictoriaTraces #279 head |
|---|---|---|---|---|---|---|
| Q1 empty | C | C, E | C, D, E | C | E | E |
| Q2 missing | D, E | C, E | C, D, E | E | E | E |
| Q3 empty or missing | C, D, E | C, E | C, D, E | C, E | E | E |
| Q4 `!= x` | B, C | B, C, E | B, C, D, E | B, C | B, C, D, E | B, C, D |
| Q5 has value | A, B, C | A, B | A, B | A, B, C | A, B, C, D | A, B, C, D |
| Q6 not in x, y | C | C, E | C, D, E | C | C, D, E | C, D |

Prometheus has no D, so its list covers A, B, C, E only. A few things to read off it:

- **PostgreSQL** is the only system that separates empty (C) from missing (D, E) in Q1 and Q2. It also drops NULL rows from `<>` and `NOT IN`: Q4 does not return D or E. To get them back you write `IS DISTINCT FROM`, which I also ran: `f IS DISTINCT FROM 'x'` returns B, C, D, E.
- **Prometheus** answers Q1, Q2 and Q3 with the same set, C and E. Its `!=` returns E, the series with no label. That is the same trap as in my bug, by design.
- **VictoriaLogs** merges even more: D (JSON null) is stored as no field, so C, D and E are one group. `-f:x` returns B, C, D, E, same as Prometheus.
- **Tempo 3.1.0** keeps empty (C) and missing (E) apart, and `!=` does not return E. This is the behaviour VictoriaTraces needs to match. One odd thing: Tempo never returned D (the attribute with an unset value) for any of the seven queries I ran. I did not look into why.
- **VictoriaTraces master** stores both C and D as `-` (I checked this with a LogsQL query on the storage). So Q1 fails, as in #280, and `!= x` returns B, C, D, E.
- **The #279 head** removes E from Q4 and Q6, which is the fix. The remaining difference from Tempo is D, which comes from the `-` storage, and C for Q1, which is #280 (fix in #296).

<!-- DIAGRAM: explorer, pick a system and a query, highlight which of A..E match, using the table above -->

The pattern: systems built for logs and metrics merge the states, and systems built for records separate them. Neither is wrong. The bug comes from putting a translation layer between the two kinds.

<img class="meme" src="/static/images/blog/memes/null-vs-empty-vs-missing-5.webp" alt="Reaction image: Calm, finally one table that explains the whole mess" width="680" height="672" loading="lazy">

## The rule I use now

Before I write a "not equal" or "not in" filter, I ask two things. Does a record with no value match here? Is "no value" the same as "empty" in this system? If the first is yes, I add an explicit existence check. If the second is yes, I stop trying to tell them apart in the query.

For a translation layer, I write the table above first, for the two systems I connect. Where the cells differ, the bug will be.

## Cheat sheet

**SQL**
- Test NULL with `IS NULL`, never `= NULL`. Compare with NULL treated as a value: `IS DISTINCT FROM` / `IS NOT DISTINCT FROM`.
- `WHERE` keeps only rows where the condition is true. Unknown is dropped, like false.
- Empty to NULL: `NULLIF(x, '')`. NULL to default: `COALESCE(x, default)`.
- Prefer `NOT EXISTS` over `NOT IN` when the inner side can hold NULL. `x NOT IN (1, NULL)` returns nothing.
- Oracle treats `''` as NULL. PostgreSQL and SQLite (tested) keep them apart.

**JSON and Go**
- `string` cannot tell empty, null and missing apart. `*string` merges null and missing. `json.RawMessage` or a wrapper type tells all three apart.
- `omitempty` drops `""` on a `string` and only `nil` on a pointer. `omitzero` (Go 1.24) behaves the same on a `*string`.

**Observability**
- Prometheus: `{l=""}` means empty or not set. A selector needs one matcher that does not match empty.
- LogsQL: empty equals missing. `f:""` finds empty or missing, `f:*` finds any non-empty value, `-f:x` also returns logs without `f`.
- LogsQL: `NOT *:""` before a pipe can return nothing (VictoriaLogs#1919). `fields ... | filter NOT *:""` works.
- TraceQL: `= nil` means missing or null. In Tempo 3.1.0 `!=` does not match spans without the attribute.

**The grid**

| | empty = missing? | null exists? | `!= x` matches missing? |
|---|---|---|---|
| PostgreSQL | no | yes | no |
| Prometheus | yes | no | yes |
| VictoriaLogs | yes | JSON null is dropped | yes |
| Tempo 3.1.0 | no | `nil` (with missing) | no |
| VictoriaTraces (open fixes #279, #296) | no in design, but `= ""` is wrong today (#280) | stored as `-`, same as empty | yes today, no with #279 |
