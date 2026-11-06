---
title: "Deterministic vs Non-Deterministic Code"
date: "2026-12-11"
description: "Deterministic vs non-deterministic code explained from scratch: where randomness hides, how to remove it, how to test it, and a Chrome bug I hit rendering video."
tags: ["tech", "determinism"]
draft: "false"
---

I render web pages into mp4 files. Same page, same settings, run it twice. Sometimes I got two different videos.

Not by a lot. A few edge pixels on a shape, a letter a shade softer. You would never see it by eye, but a hash of the file caught it at once, and once two runs disagree you cannot trust either one.

<img class="meme" src="/static/images/blog/memes/deterministic-vs-non-deterministic-1.webp" alt="Reaction image: Confusion, same input but something is off" width="680" height="438" loading="lazy">

Deterministic code gives the same output every time you give it the same input. Non-deterministic code does not. The definition is one line, and the hard part is the word "input", because most inputs are hidden. I will start from a recipe, tour the places hidden inputs live, run a Go program that counts them, and finish with the bug in my renderer that took me longest to see.

## What deterministic means

Take a recipe function.

```go
func Cake(flour, sugar, eggs int) string {
	return fmt.Sprintf("cake(%d,%d,%d)", flour, sugar, eggs)
}
```

Call `Cake(200, 100, 2)` a thousand times and you get the same string a thousand times. That is deterministic: same input, same output, no exceptions.

Now change it.

```go
func Cake(flour, sugar, eggs int) string {
	return fmt.Sprintf("cake(%d,%d,%d) baked at %s", flour, sugar, eggs, time.Now())
}
```

Same three arguments, different string on every call. The function now has a fourth input, the clock, and it is not in the signature. Nothing in `Cake(200, 100, 2)` tells the reader that it depends on when you call it.

The definition I use:

> A function is deterministic if its output depends only on its inputs, and you count *all* the inputs, including the ones the signature does not show.

The rest of this post is a list of hidden inputs and what to do about each.

There is a second meaning, since people search for it. In theory books, a deterministic algorithm has exactly one next step at each point, and a non-deterministic one may branch several ways at once (think of a non-deterministic finite automaton). In day-to-day engineering, "non-deterministic" just means the same input can give different output. That is the meaning here.

| | Deterministic | Non-deterministic |
|---|---|---|
| Same input, run twice | same output | maybe different output |
| Can you debug a failure? | yes, replay it | only if you got lucky and logged enough |
| Can you cache the result? | yes | no |
| Can you test it with `assert equal`? | yes | not without pinning something |

The last two rows are why anyone cares. You can cache, replay, parallelize and compare a deterministic function.

## Pure functions: the clean case

A pure function is a deterministic function that also does nothing outside itself: it does not write a file, change a global, or print. Output depends only on arguments, and the only effect is the return value.

```go
func Total(prices []int, taxPercent int) int {
	sum := 0
	for _, p := range prices {
		sum += p
	}
	return sum + sum*taxPercent/100
}
```

A table of inputs tests this fully. No setup, no mocks, no cleanup. That is why the usual advice is to push the messy parts (clock, disk, network) to the edges of the program and keep the middle pure.

Real programs cannot be pure all the way down, though. They read the time and talk to servers. So the goal is to let every hidden input in through one door you control, so a test or a replay can swap what comes through.

## Where non-determinism hides

Seven places, roughly in the order you meet them.

### 1. The wall clock

`time.Now()` is the one everyone knows. The fix is to pass the clock in.

```go
type clock func() time.Time

func Stamp(c clock) string { return c().Format(time.RFC3339) }
```

Production passes `time.Now`. A test passes `func() time.Time { return fixed }`. The function stays the same, and the hidden input is now a visible one.

### 2. Random numbers

Go's global `math/rand` gives a different number on every call (since Go 1.20 it is seeded randomly at startup). If you want randomness that you can replay, make your own generator and seed it.

```go
r := rand.New(rand.NewSource(42))
r.Int63() // the same value on every run of the program
```

Games do this on purpose. In Glenn Fiedler's [deterministic lockstep](https://gafferongames.com/post/deterministic_lockstep/) article, a game sends only player inputs over the network and every machine simulates the same world. That needs the random numbers to match too. Keep the seed and you can replay any run that went wrong.

### 3. Go map iteration order

This one surprises people, because the map holds the same data every time.

```go
for k, v := range scores { ... }
```

The Go spec says the iteration order is not specified and not guaranteed to be the same from one iteration to the next, and in practice the order changes between runs, as the experiment below shows. Build output from a map walk (a JSON-ish string, a list, a hash input) and it changes between runs. The fix is boring: collect the keys, `sort.Strings` them, and loop over the sorted slice.

### 4. Goroutine scheduling

Start eight goroutines that each add their number to a shared slice. The mutex stops the data race. The order in which they get the lock is still up to the scheduler.

```go
mu.Lock()
out = append(out, i)
mu.Unlock()
```

Two fixes. Give each goroutine its own slot (`out[i] = i`) so position, not arrival time, decides the order. Or send results to one collector and sort them. `go run -race` will not flag the mutex version, because there is no race. The program is correct and still non-deterministic, and those are different properties.

### 5. Floating point

Float addition is not associative: `(a + b) + c` can differ from `a + (b + c)`. Sum the same numbers in a different order and you can get a different answer. That happens whenever you parallelize a sum or let a map or a goroutine pick the order.

The lockstep article above goes further: the same float code can differ across compilers, operating systems and CPU architectures, and debug and release builds probably will not match because of floating point optimizations. So fix the order of operations, and do not claim the result holds across machines.

### 6. The network

Any call to another machine is an input you do not control: latency, a retry, a different answer from a server. You cannot make the internet deterministic, so cut it out of the part you test. Fake the server, or record real responses once and replay them.

### 7. The environment of whoever runs you

Environment variables, locale, current directory, number of CPUs, the time zone. They feel like "not input" because you never typed them. They are input.

There is an eighth place, the browser, and that is where my real story is. First I will measure these seven.

<img class="meme" src="/static/images/blog/memes/deterministic-vs-non-deterministic-2.webp" alt="Reaction image: Slowly realising everything is connected" width="680" height="510" loading="lazy">

## Measure it: hash the output of 100 runs

Reading about hidden inputs only goes so far, so count them. Run the thing many times, hash each output, and count the *different* hashes. A deterministic function gives 1.

```go
func distinct(f func() []byte) int {
	seen := map[[32]byte]bool{}
	for i := 0; i < 100; i++ {
		seen[sha256.Sum256(f())] = true
	}
	return len(seen)
}
```

I use a hash because the bug below was invisible to the eye. A hash compares every byte, is cheap, and gives one number to assert on.

Here is the full program: ten cases, each hidden input from the list above next to its fix.

```go
package main

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

type clock func() time.Time

func withNow() []byte {
	return []byte(fmt.Sprint(time.Now().UnixNano()))
}

func withClock(c clock) []byte {
	return []byte(fmt.Sprint(c().UnixNano()))
}

func unseeded() []byte {
	return []byte(fmt.Sprint(rand.Int63()))
}

func seeded() []byte {
	return []byte(fmt.Sprint(rand.New(rand.NewSource(42)).Int63()))
}

var scores = map[string]int{"ada": 3, "bob": 1, "cy": 2, "dee": 5, "eve": 4, "fay": 6}

func mapRange() []byte {
	s := ""
	for k, v := range scores {
		s += fmt.Sprintf("%s=%d,", k, v)
	}
	return []byte(s)
}

func mapSorted() []byte {
	keys := make([]string, 0, len(scores))
	for k := range scores {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := ""
	for _, k := range keys {
		s += fmt.Sprintf("%s=%d,", k, scores[k])
	}
	return []byte(s)
}

func goroutinesAppend() []byte {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var out []int
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mu.Lock()
			out = append(out, i)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return []byte(fmt.Sprint(out))
}

func goroutinesOrdered() []byte {
	var wg sync.WaitGroup
	out := make([]int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out[i] = i
		}(i)
	}
	wg.Wait()
	return []byte(fmt.Sprint(out))
}

func floatSum(order []int) []byte {
	vals := []float32{1e8, 1, -1e8, 1, 0.1, 0.2, 0.3}
	var s float32
	for _, i := range order {
		s += vals[i]
	}
	return []byte(fmt.Sprint(s))
}

func floatForward() []byte { return floatSum([]int{0, 1, 2, 3, 4, 5, 6}) }
func floatShuffled() []byte {
	o := rand.Perm(7)
	return floatSum(o)
}

func distinct(f func() []byte) int {
	seen := map[[32]byte]bool{}
	for i := 0; i < 100; i++ {
		seen[sha256.Sum256(f())] = true
	}
	return len(seen)
}

func main() {
	fixed := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		f    func() []byte
	}{
		{"1 time.Now()", withNow},
		{"2 injected clock", func() []byte { return withClock(func() time.Time { return fixed }) }},
		{"3 math/rand global", unseeded},
		{"4 seeded rand", seeded},
		{"5 range over map", mapRange},
		{"6 sorted map keys", mapSorted},
		{"7 goroutines append", goroutinesAppend},
		{"8 goroutines write own slot", goroutinesOrdered},
		{"9 float32 sum, fixed order", floatForward},
		{"10 float32 sum, shuffled order", floatShuffled},
	}
	for _, c := range cases {
		fmt.Printf("%-32s %3d distinct hashes / 100 runs\n", c.name, distinct(c.f))
	}
}
```

I ran it with Go 1.25.4 on an Apple M-series Mac (`go run .`). This is the real output:

```text
1 time.Now()                      33 distinct hashes / 100 runs
2 injected clock                   1 distinct hashes / 100 runs
3 math/rand global               100 distinct hashes / 100 runs
4 seeded rand                      1 distinct hashes / 100 runs
5 range over map                   6 distinct hashes / 100 runs
6 sorted map keys                  1 distinct hashes / 100 runs
7 goroutines append                2 distinct hashes / 100 runs
8 goroutines write own slot        1 distinct hashes / 100 runs
9 float32 sum, fixed order         1 distinct hashes / 100 runs
10 float32 sum, shuffled order    24 distinct hashes / 100 runs
```

The odd rows are the broken versions and each fix gives exactly 1. A few numbers deserve a closer look.

Row 1 is 33, not 100. `time.Now()` on macOS has a coarse tick, so some of my 100 calls landed on the same nanosecond count. Non-deterministic does not mean different every time, only not guaranteed the same. On a second run I got 36, and 34 on others. Your numbers will differ. The only stable claim is that the fixed versions always give 1.

Row 5 is 6. Six keys have 720 possible orders, yet I saw only six. A Go map range does not shuffle everything: it starts at a random spot and walks the table from there. Still not one.

Row 7 is only 2. Eight goroutines that only append mostly finish in the order they started, because the job is tiny. With the race detector on (`go run -race .`) the same case gave 99 distinct hashes, since `-race` slows and perturbs scheduling. So a test that passes 100 times on your laptop is weaker evidence than it looks. Put real work inside the goroutines, or run with `-race`, to shake the ordering loose.

Row 10 is the float one. Same seven numbers, only the order of addition changes, and I got 24 different float32 answers in 100 runs. Row 9 adds them in a fixed order and always agrees.

Find the hidden input, move it through a door you control, count hashes again. That is the whole method.

## Flaky tests are non-determinism the test did not pin

A flaky test passes on one run and fails on the next with no code change. Every flaky test I have debugged was one of the rows above:

- It compares output built from a map walk. Sometimes the keys come out in the other order. (row 5)
- It starts goroutines and asserts on the result before they finish, or on an order they never promised. (row 7)
- It sleeps for 100 ms and hopes the server is ready. The server is sometimes slow. (the network, and the clock)
- It uses `rand` and sometimes draws a value that hits the edge case. (row 3)
- It checks "created less than a second ago" on a loaded CI machine. (row 1)

<img class="meme" src="/static/images/blog/memes/deterministic-vs-non-deterministic-3.webp" alt="Reaction image: Guilty relief when the rerun goes green" width="680" height="666" loading="lazy">

The tempting fix is "re-run failed tests up to three times". It turns the build green and leaves the bug alive. A test that passes on the second try just told you an input is unpinned. The real fix is to find which hidden input differs between the passing and failing run, and move it behind a door you control. Hashing finds it: run the suspect code 100 times, and the count of distinct outputs tells you whether it is a code bug or an environment bug.

## Reproducible builds: the same idea, for binaries

A build is a function too: source code in, binary out. The [Reproducible Builds project](https://reproducible-builds.org/) defines a build as reproducible when, given identical source, build environment and build instructions, any party can recreate bit-by-bit identical copies of the specified artifacts. So anyone can rebuild and check that the binary you shipped came from that source.

The hidden inputs are the same kind: timestamps, the order files are read from disk, the absolute path of the build directory. Go embeds file paths in the binary for stack traces. I tested it with one program, built from two different directories:

```bash
for d in b1 b2; do (cd $d && go build -o app . && shasum -a 256 app | cut -c1-16); done
# 44004875bdde9521
# f516e66e20bd9bb6

for d in b1 b2; do (cd $d && go build -trimpath -o app . && shasum -a 256 app | cut -c1-16); done
# ec30836aabdad6e7
# ec30836aabdad6e7
```

Same source, same Go version, same machine, two different binaries, until `-trimpath` removes the directory from the output. The hidden input was the folder name.

## Browsers have a clock you did not inject

Now the bug. First, some background on screenshotting a web page from a script.

You drive headless Chrome from Puppeteer or Playwright: open the page, wait for it to load, take a screenshot. Anyone searching "headless chrome screenshot" hits the same wall. You pick the moment by guessing. "Wait for page load" and "add a delay" are the usual advice, and both are bets on how long the page needs. The page has its own clock, and it keeps running while you decide.

If the page has animation, the screenshot depends on how far the animation got, which depends on how long you waited, which depends on how loaded your machine is. The clock is the hidden input again, same as row 1.

The usual cure is to take control of time. [timesnap](https://github.com/tungs/timesnap) replaces `Date`, `performance.now`, `setTimeout` and `requestAnimationFrame` (and the interval and cancel variants) with a virtual clock, so the page believes whatever time the tool says. Remotion asks you to code animations so they run purely off `useCurrentFrame()`, and has a separate page titled "Don't use CSS animations". My renderer does a version of the first. In vawe, a page that reads `Date.now`, `performance.now` or `Math.random` gets a virtual clock that is a function of the seek time, and a test renders each such page twice and compares frame hashes ([`580faf088`](https://github.com/vawe-hq/vawe/commit/580faf08825b2c3e719593154e002c5128486a74)). That is the "inject the clock" fix from row 1, at the size of a whole browser.

<a class="src-link" href="https://github.com/vawe-hq/vawe/blob/580faf08825b2c3e719593154e002c5128486a74/tests/media/render-page-determinism.test.mjs" target="_blank" rel="noopener noreferrer">↗ tests/media/render-page-determinism.test.mjs</a>

What is left is the case both approaches skip. Replacing JavaScript timer functions does not touch animations the browser runs itself: CSS animations and transitions, and the Web Animations API (`el.animate(...)`). The timesnap README lists this as a limitation: pages where changes come from CSS transitions or animations will likely not render as intended. I wanted to keep those. Page authors, and the coding agents that write pages for me, reach for `el.animate` first, and I did not want to ask anyone to rewrite everything as a function of the frame number.

So: if I keep real browser animations, can I still get the same pixels every time?

## The bug: a first seek at 1.5 s painted different pixels

My renderer works by seeking. To make frame N, it loads the page, sets every animation's time to the exact moment for frame N, and takes a screenshot. I set the animation time, so it should not matter when the screenshot happens. Page load plus 0.05 s or plus 1.5 s: same frame.

It mattered. A first seek 1.5 s after page load painted different edge pixels than a first seek at 0.05 s, a few pixels on shape edges. The commit does not record the numbers, so I will not invent any.

<img class="meme" src="/static/images/blog/memes/deterministic-vs-non-deterministic-4.webp" alt="Reaction image: Obsessed discovery after hours of nothing" width="486" height="568" loading="lazy">

The browser's document timeline, the clock every animation on the page is measured against, runs in real time from page load. While the page sits there, Chrome's compositor already rasterizes the animated layers against that running clock. My seek then moves each animation to the right time, but the pixels were painted for a moment I did not choose, and which moment depended on how long the page waited before the first seek. The wait was a hidden input.

The fix is nine lines in `openPage` ([`1454df446`](https://github.com/vawe-hq/vawe/commit/1454df4469bb165b61b37ba9d80799040d0c76f2)):

```js
const cdp = await opened.page.createCDPSession();
await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
// The document timeline runs in real time from page load, and the compositor rasterizes the animated layers against that
// time before the first seek pauses them: a first seek 1.5 s after load painted other edge pixels than one at 0.05 s.
// Rate 0 freezes the timeline, so only a seek moves an animation.
await cdp.send('Animation.enable');
await cdp.send('Animation.setPlaybackRate', { playbackRate: 0 });
```

<a class="src-link" href="https://github.com/vawe-hq/vawe/blob/1454df4469bb165b61b37ba9d80799040d0c76f2/harness/media/render-page.mjs" target="_blank" rel="noopener noreferrer">↗ harness/media/render-page.mjs · openPage</a>

`Animation.setPlaybackRate` is a Chrome DevTools Protocol call; the protocol docs describe it as setting the playback rate of the document timeline. At rate 0 the timeline stops advancing, so only my own seek can move an animation. Same idea as the injected clock, applied to the one clock the JS-level tricks cannot reach.

### The test

The fix needed a test, because "I think it is fixed" is the sentence this post warns against. The test page ([`late-first-seek.html`](https://github.com/vawe-hq/vawe/blob/1454df4469bb165b61b37ba9d80799040d0c76f2/tests/fixtures/pages/late-first-seek.html)) has two `el.animate` calls: a soft gradient blob that drifts, and a heading that fades and scales in with a blur. The test opens the page twice, waits 0 ms the first time and 2000 ms the second, then seeks to 0, 0.1, 0.5 and 1.2 s and hashes a screenshot at each:

```js
test('a first seek 2 s after load paints the same frames as one right after load', async () => {
  const times = [0, 0.1, 0.5, 1.2];
  const soon = await framesAfter(0, times);
  const late = await framesAfter(2000, times);
  assert.deepEqual(late, soon);
  assert.ok(new Set(soon).size > 1, 'the frames move');
});
```

<a class="src-link" href="https://github.com/vawe-hq/vawe/blob/1454df4469bb165b61b37ba9d80799040d0c76f2/tests/media/render-first-seek-late.test.mjs" target="_blank" rel="noopener noreferrer">↗ tests/media/render-first-seek-late.test.mjs</a>

The second assertion matters as much as the first. "The late hashes equal the early hashes" is trivially true if every frame is a blank page. `new Set(soon).size > 1` proves the frames move, so equal means equal and alive. A determinism test that passes on empty output is not a test.

I ran the test 20 times on the parent of `1454df446` and 20 times on `1454df446`, on my M4 MacBook (16 GB), with the same Chrome (156.0.8078.80) for both. The parent failed 12 of 20 runs. On `1454df446` it passed 20 of 20. In the failing runs I looked at, one hash of the four differed, the first one (the seek to 0 s), and the other three matched:

```text
+ actual - expected
[
+   'f8e39f7a82da08f91c49727b3084f00c',
-   '2e6a4ab2c5bc0ea863716db7275f85b4',
    'f7dcf49e0825366ebdf617b75a29fc72',
    '5b0f5023b60aef3bf73aceaf6192fdf3',
    'be0daa35fef1b199c40bc6ed27cd986a'
]
```

So the bug was a coin flip, not a constant. A test that fails 12 times in 20 on the old code is a real test, and it also shows why one green run on the old code would have proved nothing.

## One symptom, two causes

Two days later (Oct 10, the timeline fix was Oct 8), a different page showed the same symptom. A "gooey" effect (an SVG filter that blurs shapes and sharpens the blur back into blobs) rendered differently between two runs. The commit message for the fix says it directly: GPU compositing rasterized the gooey filter in two states at random in a shared draft browser, and 70 of 216 frames differed between two renders ([`0b1d35912`](https://github.com/vawe-hq/vawe/commit/0b1d35912526884406dc9f7e3bed9fe4e0f53fa7)).

That is a separate cause with the same outward look: same input, two videos. Here GPU compositing chose between two ways to rasterize the SVG filter. The fix: each draft lane gets its own Chrome, and a page that draws an SVG filter also composites in software. Its test renders the gooey page three times with two workers and asserts all three hash lists are equal, and that the window moves ([`render-draft-filter-determinism.test.mjs`](https://github.com/vawe-hq/vawe/blob/0b1d35912526884406dc9f7e3bed9fe4e0f53fa7/tests/media/render-draft-filter-determinism.test.mjs)).

I learned this part slowly. A hash test tells you *that* two runs differ, not *why*. I had one symptom with two causes behind it. Had I stopped after the first fix because the symptom went away on my test page, the second would still be in the product. Change one thing per commit, keep one test per cause, and when the symptom shows up again, assume a new cause until the hash test says otherwise.

I re-ran it on my M4 MacBook, with the same Chrome for both commits. I rendered the gooey demo (3.6 s at 60 fps, 216 frames, 640 by 360, two workers) twice and counted the frames whose `framemd5` line differed. Before the fix, five pairs of renders gave 214, 212, 212, 213 and 214 of 216 differing frames. After the fix, three pairs gave 0 of 216 each time. My "before" is far worse than the 70 in the commit message, so I do not know what the 70 measured; the "after" matches.

## What determinism unlocked

Once I trusted the hash, I could make the renderer faster without fear. Capturing a final render used one shared Chrome for all workers, with a lock around screenshots, so the workers took turns. The change in [`8efe38cad`](https://github.com/vawe-hq/vawe/commit/8efe38cadb08674cac60ee09ea31ed19cd18f0d8) gives each worker its own Chrome, so lanes share no screenshot lock. The commit message says capture dropped from 140 s to 47 s on the Tidepool test film.

<a class="src-link" href="https://github.com/vawe-hq/vawe/blob/8efe38cadb08674cac60ee09ea31ed19cd18f0d8/harness/media/render-page.mjs" target="_blank" rel="noopener noreferrer">↗ harness/media/render-page.mjs · openLane</a>

The link is my own reasoning, not something the commit says. Parallel workers are a classic way to create non-determinism (row 7), and I only felt safe adding more because a hash test would fail if two workers painted differently. Parallelism, caching and retries all rest on one promise: running the thing again gives the same answer. Break it and each of them gives you wrong output, faster.

<img class="meme" src="/static/images/blog/memes/deterministic-vs-non-deterministic-5.webp" alt="Reaction image: Relief and speed after a long painful stretch" width="680" height="472" loading="lazy">

I re-ran it on my M4 MacBook (10 cores, 16 GB), rendering the current Tidepool film as a final (1080p, 60 fps, 1,200 frames, 32 subframes each). The capture step took 276 s and 303 s on the parent of `8efe38cad`, and 124 s and 89 s on `8efe38cad`. That is about 2.5 to 3.4 times faster, not the 140 s to 47 s from the commit message. The direction holds; the exact numbers do not carry over.

## Cheat sheet

**The definition.** Deterministic: same visible *and hidden* inputs, same output. Count the hidden ones.

| Hidden input | Symptom | Fix |
|---|---|---|
| Wall clock | output contains "now" | pass a clock in; use a fixed time in tests |
| Random numbers | different values each run | own generator with a seed; log the seed |
| Go map order | output order changes | collect keys, sort, loop |
| Goroutine scheduling | result order changes | write to your own slot, or collect and sort |
| Floating point | tiny differences in sums | fix the order of operations; do not assume across machines |
| Network | slow or different replies | fake the server; record and replay |
| Environment | works on one machine | pin env, locale, timezone, paths (`-trimpath`) |
| Browser animation clock | edge pixels differ by wait time | stop the timeline (`Animation.setPlaybackRate` 0), move it only by seeking |

**The test.** Run the thing N times, hash each output, count distinct hashes. Pass means 1. Also assert that the output *changes* when the input changes, or an empty output will pass.

**What a pass cannot prove.** It proves these N runs on this machine agreed. It does not prove another CPU, another Go version or another load level agrees.

**Flaky test.** An input you did not pin. Retries hide it. Find the input.

**Reproducible build.** Pin timestamps, file order and paths. In Go, `-trimpath` made two builds from two directories identical in my test.

**Order of work.** Make it deterministic, prove it with a hash, then make it fast with parallelism or caching. Never the other way round.

**When the hash differs.** The test says *that*, not *why*. Change one thing, re-hash, and expect more than one cause.
