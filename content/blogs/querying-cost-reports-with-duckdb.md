---
title: "The Right Tool in the Wrong Runtime: Querying Cloud Cost Data with DuckDB"
date: "2026-02-24"
description: "Building a cloud cost-reporting pipeline over Oracle CUR data: from a managed warehouse, to DuckDB in a serverless function that hit a ~200MB memory wall, to a long-lived VM query service. A lesson about separating the tool from where you run it."
tags: ["infra", "tech"]
---

Here's a task that sounds boring and turned into the most useful infrastructure lesson I learned all year: *let people ask questions about our cloud bill.* Not "what's the total," the finance dashboard does that. Questions like "which service, in which compartment, drove last month's spike, broken down by day." That means querying the raw, itemized billing data, and cloud billing data is genuinely large: one row per resource per hour, dozens of columns, dumped as a pile of compressed CSV files into object storage.

I went through three architectures to serve those queries. The interesting part is that the query engine I landed on, DuckDB, was also in the version that failed. The engine was never the problem. Where I ran it was. This post is the honest walk through all three, because the mistake in the middle is one I see people make constantly, and the fix reframes how I think about "serverless" now.

## What the raw cost data actually is

Every big cloud gives you a detailed billing export. Oracle calls it the **Cost and Usage Report** (CUR); AWS uses the same name and the same shape. The provider periodically writes billing line items, one row per resource per hour, into a bucket you own: on Oracle that's Object Storage, on AWS it's S3. The files are typically gzipped CSV, they have a wide and slightly awkward schema (resource IDs, tags, compartments, unit prices, usage quantities), and over a month they add up to hundreds of megabytes to gigabytes depending on how much you run.

The important property: this data already lives as files in object storage. Nobody is going to hand you a nice database with it loaded. Your job is to get from "a bucket full of CSVs" to "answer this SQL question quickly," and every architecture below is just a different answer to *where does the querying happen.*

## Attempt one: load it into a warehouse

The textbook answer is a data warehouse. On Oracle that's **Autonomous Data Warehouse** (ADW); the equivalents everyone knows are Snowflake, BigQuery, and Redshift. You set up an ingest job that pulls the CUR files, loads them into warehouse tables, and then you query with SQL and it's fast and columnar and everything you'd want.

It works. It's also a lot of machine for the job. A warehouse is a standing, billed service with its own provisioning, its own ingestion pipeline to keep fed, its own access model to manage, and its own cost that you are now, ironically, adding to the very bill you're trying to analyze. For a cost-reporting feature that runs queries intermittently, paying for an always-on analytical database felt like buying a forklift to move one box a week. It's the right tool when the query volume justifies the standing cost. Mine didn't. I wanted something that queried the files where they already sat and cost nothing when idle.

## Attempt two: DuckDB in a serverless function (and the wall I hit)

This is where [DuckDB](https://duckdb.org/) enters, and if you haven't used it, the elevator pitch is "SQLite for analytics." It's an embedded, in-process OLAP engine: no server, no cluster, you import a library and run columnar SQL right inside your program. Its killer feature for this problem is the [`httpfs` extension](https://duckdb.org/docs/extensions/httpfs.html), which lets it read CSV and Parquet **directly from object storage** over the S3 API. You can literally point it at a bucket and write `SELECT ... FROM 's3://bucket/cur/*.csv.gz'` and it streams the files, no load step, no warehouse.

So the plan was clean and cheap: a serverless function (Oracle Functions, the same idea as AWS Lambda). A request comes in, the function spins up, DuckDB reads the relevant CUR files from Object Storage, runs the aggregation, returns JSON, and the function shuts down. Zero standing cost, pay only per query. On paper this is basically what **AWS Athena** is: serverless SQL over files in S3. It felt obviously right.

Then it OOMed. The serverless function had a memory ceiling of around **200MB**, and DuckDB running a real aggregation over a month of CUR blew straight through it and got killed.

<aside class="callout callout--warn" data-label="The wall">
A serverless function's memory limit is a hard ceiling on the whole process, and an analytical query engine wants to use a lot of memory. Those two facts are in direct conflict. It isn't a config you tune your way out of at the small end; the workload genuinely needs more room than the runtime is willing to give.
</aside>

## Why it OOMed, and why that's not DuckDB's fault

It's worth understanding *why*, because "just add more memory" misses the point. Analytical queries are memory-hungry by nature. A `GROUP BY` builds a hash table keyed by every distinct group. An `ORDER BY` needs the rows in memory to sort. Joining or aggregating a wide dataset means materializing columns. DuckDB is genuinely good at this: it's vectorized, it compresses, and crucially it can **spill to disk** when a query exceeds memory, streaming intermediate results to local storage instead of dying.

But spilling needs a local disk to spill *to*, and a serverless function has almost no usable scratch space and a tiny memory budget besides. So the one feature that would have saved me, out-of-core execution, had nothing to work with. The engine wasn't too weak. The environment starved it of the two things analytical work needs: RAM headroom and a disk to overflow onto. FaaS is optimized for short, stateless, small-footprint request handlers. A cost aggregation over a month of billing data is none of those things.

There was a second, quieter tax too: cold starts and re-fetching. Every invocation was a fresh process that re-downloaded the CUR files from Object Storage and re-initialized DuckDB from nothing. Even if memory hadn't killed it, I'd have been paying the network and startup cost on every single query, with no way to cache anything between requests.

## Attempt three: give DuckDB a real home

The fix was almost anticlimactic: run the exact same DuckDB, in the exact same "read files from Object Storage" way, but inside a **long-lived process on a VM** instead of a serverless function. A small query service that boots once, holds a DuckDB instance, reads the CUR from Object Storage, and stays up to answer requests.

Everything that was fighting me flipped to working for me:

- **Real memory.** A modestly-sized VM has gigabytes of RAM, so the aggregations that OOM'd at 200MB just run.
- **A real disk to spill to.** Now DuckDB's out-of-core execution actually has somewhere to go, so a query bigger than RAM degrades to "a bit slower" instead of "killed."
- **Persistence between queries.** The process stays alive, so it can cache: keep hot data around, avoid re-downloading and re-parsing the same CUR files on every request, reuse the initialized engine.
- **No cold starts.** The engine is already warm when the request arrives.

Same engine, same query-in-place approach, completely different outcome, because the runtime finally matched the shape of the work.

<aside class="callout callout--tip" data-label="The reframe">
"Serverless vs. server" isn't a default you pick by fashion. It's a question of whether your workload is short, stateless, and small-footprint (serverless is great) or long, stateful, and memory-hungry (it isn't). I'd picked the runtime first and tried to cram the workload in. The fix was to let the workload's shape pick the runtime.
</aside>

## The pattern underneath, and one upgrade

Step back and there's a tier I'd underrated between "spin up a warehouse" and "load it into Postgres": **query the files in place.** DuckDB reading Parquet/CSV straight from object storage is a legitimate middle path. No ingestion pipeline, no standing warehouse bill, just SQL over the files that already exist. It's the same idea AWS sells as Athena, except you own the process and there's no per-query service in between. For intermittent analytical queries over data that already lives in a bucket, it's often exactly right. You just have to run it somewhere that isn't starved.

The one upgrade I'd make every time now: **convert the CUR to Parquet before querying.** The raw exports are row-oriented gzipped CSV, which means every query re-parses text and reads every column even when you need three of them. Parquet is columnar and compressed with real per-column statistics, so DuckDB can skip whole files and columns it doesn't need (predicate and projection pushdown). A one-time "CSV to Parquet" pass turns most of these queries from "read and parse everything" into "touch only the relevant columns of the relevant files," and it's the single biggest speedup available for this kind of workload.

## TL;DR

- Cloud billing exports (Oracle **CUR**, AWS CUR) are just piles of gzipped CSV in object storage. The whole design question is *where the querying happens.*
- A **warehouse** (ADW / Snowflake / BigQuery) works but is a standing, billed, heavyweight service. Overkill for intermittent queries.
- **DuckDB reading files in place** from object storage is the cheap, elegant middle tier. The engine is not the risk.
- Running that DuckDB **inside a serverless function** failed: a ~200MB memory ceiling and no spill disk starve an analytical query. FaaS is for short, small, stateless handlers, which this is not.
- Running the **same DuckDB on a long-lived VM** fixed it: real RAM, a disk to spill to, caching between queries, no cold starts.
- Separate the **tool** from the **runtime**. Let the workload's shape (long, stateful, memory-hungry) choose where it runs. And convert CSV to **Parquet** for a big, free speedup.

The lesson I actually walked away with: when something OOMs, ask whether the tool is wrong or whether you've just put the right tool somewhere it can't breathe. Most of the time, for me, it was the second one.

## Go deeper

- [DuckDB](https://duckdb.org/), the embedded OLAP engine, and its [`httpfs` extension](https://duckdb.org/docs/extensions/httpfs.html) for querying object storage directly
- [Out-of-core / larger-than-memory execution in DuckDB](https://duckdb.org/2024/07/09/memory-management.html), the spill-to-disk behavior a serverless function can't use
- [AWS Cost and Usage Reports](https://docs.aws.amazon.com/cur/latest/userguide/what-is-cur.html), the same data shape on the AWS side, and [Athena](https://aws.amazon.com/athena/), the managed "SQL over S3" this pattern reimplements
- [Apache Parquet](https://parquet.apache.org/), the columnar format worth converting to before you query

---

*Fun fact: the whole detour happened because "serverless" sounds cheaper and simpler by default, and for request handlers it usually is. Analytics is the workload that quietly breaks that reflex. A warm VM holding a fat in-memory engine is unfashionable and, for this, completely correct.*
