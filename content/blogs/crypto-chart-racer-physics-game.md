---
title: "I Turned a Crypto Price Chart Into a Dirt Bike Racetrack"
date: "2026-06-14"
tags: ["tech", "typescript"]
draft: "true"
---

Every crypto chart is basically already a set of hills. Green candles going up, red candles crashing down, that one all-time-high peak everyone's still crying about. So one weekend I asked a dumb question: what if you could *ride* it? What if Bitcoin's last 30 days was a Hill Climb Racing level, and a pump was a launch ramp, and a dump was a cliff you have to survive?

That's [crypto-chart-racer](https://github.com/Vandit1604/crypto-chart-racer). You pick any token and a time range, and its actual price history becomes a physics racetrack you ride a dirt bike across, doing flips off the volatility. It's TypeScript, one physics library, no framework, no game engine. This post is how a boring array of prices becomes a rideable, jumpable, wipeout-able hill.

<figure>
  <img src="/static/images/blog/09-chart-to-terrain.svg" alt="A jagged BTC price line at top with an all-time-high marker is transformed, via resampling to 140 points and Catmull-Rom smoothing into Matter.js bodies, into smooth rideable hills at the bottom with a dirt bike racing toward a TODAY finish line.">
  <figcaption>The whole trick in one picture: a price series is resampled, smoothed, and turned into solid ground you can ride.</figcaption>
</figure>

## Step one: getting prices that don't fall over

Before any fun, you need the data, and this is where "toy project" quietly becomes "real engineering." I pull price history from [CoinGecko](https://www.coingecko.com/en/api). But free crypto APIs rate-limit you, go down, and generally cannot be trusted to be there when a player clicks "race." So the fetch isn't one call, it's a fallback chain:

```ts
// 1) Primary: CoinGecko (broadest coverage).
try {
    const data = await cg(`/coins/${id}/market_chart?vs_currency=usd&days=${days}`);
    prices = (data.prices || [])
        .filter((row) => Array.isArray(row) && Number.isFinite(row[1]))
        .map(([t, p]) => ({ t, p }));
    if (prices.length < 2) prices = null;
} catch (err) { primaryErr = err; }

// 2) Fallback: Binance public klines (huge limits, major coins only).
if (!prices && symRaw) {
    const binance = await fetchBinance(symRaw, range);
    if (binance && binance.length >= 2) { prices = binance; source = 'binance'; }
}

// 3) Last resort: a previously cached good snapshot (slightly stale).
if (!prices) {
    const stale = cache.get(staleKey);
    if (stale) return stale.value;
}
```

<a class="src-link" href="https://github.com/Vandit1604/crypto-chart-racer/blob/92055a530c116d269312a3607c6d9f1f276e2807/api/_coingecko.ts#L145-L179" target="_blank" rel="noopener noreferrer">↗ api/_coingecko.ts</a>

CoinGecko first. If it's down or rate-limited, fall back to Binance's public candles (way higher limits, but only the major coins). If *both* fail, serve a slightly stale cached snapshot rather than showing the player an error. Three levels of "the show must go on."

<aside class="callout callout--tip" data-label="Serve stale over serving nothing">
For a game, a chart that's six hours old is completely fine. A blank error screen is not. That priority order, "slightly wrong beats broken," drives the whole data layer: cache aggressively, fall back gracefully, and only ever fail if literally every source is dead.
</aside>

There's also a caching trick I'm quietly proud of. The cache key is the current time floored into 6-hour buckets. That means everyone who rides "BTC, 30 days" in the same 6-hour window gets the *exact same track*, and the API only gets hit once per window instead of once per player. Deterministic tracks and rate-limit safety from one line of `Math.floor`.

## Step two: a price line is too spiky to ride

Here's the first real game problem. Raw price data is jagged. A single volatile candle is a vertical spike, and if you turn that straight into ground, you get an un-rideable needle that launches the bike into orbit or stops it dead. Real terrain needs to be *smooth*.

Two moves fix this. First, resample the messy, irregularly-spaced price series down to a fixed **140 control points**, so a 7-day race and a 1-year race produce tracks of the same length and feel. Then smooth between those points with [Catmull-Rom interpolation](https://en.wikipedia.org/wiki/Cubic_Hermite_spline#Catmull%E2%80%93Rom_spline), a spline that passes through every control point but rounds off the path between them:

```ts
const norm = (p: number) => (p - minPrice) / span;   // 0..1
const toY  = (p: number) => -norm(p) * AMPLITUDE;    // higher price -> up

for (let j = 0; j < c - 1; j++) {
    const y0 = yAt(j - 1); const y1 = yAt(j);
    const y2 = yAt(j + 1); const y3 = yAt(j + 2);
    for (let s = 0; s < SUBDIV; s++) {
        const t = s / SUBDIV;
        surface.push({ x: (j * SUBDIV + s) * SEGMENT_W,
                       y: catmullRom(y0, y1, y2, y3, t) });
    }
}
```

<a class="src-link" href="https://github.com/Vandit1604/crypto-chart-racer/blob/92055a530c116d269312a3607c6d9f1f276e2807/src/data/terrain.ts#L96-L121" target="_blank" rel="noopener noreferrer">↗ src/data/terrain.ts</a>

Notice `toY` flips the sign: higher price means further *up* (negative Y is up in screen coordinates), so an expensive coin literally becomes a high hill. Then Catmull-Rom subdivides each gap into 9 smooth steps. A jagged candle becomes a rolling slope you can actually ride. Each little segment of that smoothed surface then gets turned into a solid physics body (a trapezoid down to a baseline) so the bike has something to touch.

<aside class="callout" data-label="Why not just draw the line?">
It would be easy to draw the price line and stop. But drawing isn't physics. To ride it, every bump has to be a real collision surface the wheels can push against. The smoothing isn't cosmetic, it's what makes the difference between "a chart with a bike sticker on it" and a chart the bike genuinely interacts with.
</aside>

## Step three: making it feel like a game, not a physics demo

The bike is [Matter.js](https://brm.io/matter-js/), a 2D physics engine: a chassis and two wheels, with a motor torque on the rear wheel and mid-air pitch control for flips. But the thing that makes it feel *good* is less obvious than the physics. It's the loop.

```ts
private loop = (now: number): void => {
    this.rafId = requestAnimationFrame(this.loop);
    const frame = Math.min(now - this.lastTime, 100);
    this.lastTime = now;
    if (this.status === 'running') {
        this.accumulator += frame;
        let steps = 0;
        while (this.accumulator >= PHYSICS.STEP_MS && steps < PHYSICS.MAX_STEPS_PER_FRAME) {
            this.fixedStep();
            this.accumulator -= PHYSICS.STEP_MS;
            steps += 1;
        }
    }
};
```

<a class="src-link" href="https://github.com/Vandit1604/crypto-chart-racer/blob/92055a530c116d269312a3607c6d9f1f276e2807/src/game/Game.ts#L153-L185" target="_blank" rel="noopener noreferrer">↗ src/game/Game.ts</a>

That's a **fixed-timestep accumulator**, and it's the single most important pattern in game physics. Instead of advancing the simulation by however long the last frame took (which makes physics behave differently at 60fps vs 144fps vs a laggy 30fps), you always step the physics by a fixed slice of time, and just run more or fewer steps to catch up. Result: the bike handles identically on every machine. Without this, the same jump would clear a gap on a fast laptop and faceplant on a slow one.

## The bit I got weirdly right: flips

Detecting a flip sounds trivial and is a trap. The naive version, "did the bike end upright after spinning," breaks constantly. My version integrates the *total* rotation while airborne and counts a flip each time it crosses a full 360 degrees. And critically, a run only ends when the bike is upside down **and has stopped spinning**:

- A flip in progress still has high angular velocity, so it's never cut off mid-rotation. You always get to complete the trick.
- Just grazing the ground with the chassis doesn't kill you. Only settling inverted does.

<aside class="callout callout--warn" data-label="The subtle one">
"Landed a flip" and "is currently upside down" look like the same check and are completely different. If you crash the instant the bike is inverted, every flip is a wipeout and the game is unplayable. The fix is state plus patience: track rotation over time, and only declare a crash once the bike is inverted AND settled. Getting this wrong is why a lot of amateur physics games feel unfair.
</aside>

I even wrote headless tuning scripts that drive the *real running game* in a browser and sweep the physics constants to find the values that feel best, instead of guessing. Turns out "feel" is something you can partly measure if you're willing to automate playing your own game a thousand times.

## Why build this

No reason. That's the reason. It taught me physics loops, spline smoothing, and API resilience better than any tutorial, because I actually cared whether the bike felt good. "What if the chart was a racetrack" is a stupid question, and chasing stupid questions is how I learn the serious things almost by accident.

## TL;DR

- A crypto price chart is already hill-shaped. crypto-chart-racer turns any token's real price history into a **rideable physics racetrack**.
- **Data resilience:** CoinGecko, then Binance, then a stale cache. Never show an error if a six-hour-old chart will do. 6-hour cache buckets give everyone the same track and spare the rate limit.
- **Terrain:** resample prices to 140 points, smooth with **Catmull-Rom** so spiky candles become rideable slopes, then turn each segment into a solid physics body.
- **Feel:** a **fixed-timestep** physics loop makes the bike handle the same on every machine. This is the pattern that matters most.
- **Flips:** integrate rotation over time and only crash when inverted *and settled*, so tricks always get to finish.

## Go deeper

- The game's code: [github.com/Vandit1604/crypto-chart-racer](https://github.com/Vandit1604/crypto-chart-racer)
- [Matter.js](https://brm.io/matter-js/), the 2D physics engine doing the heavy lifting
- Glenn Fiedler's classic ["Fix Your Timestep!"](https://gafferongames.com/post/fix_your_timestep/), the canonical explanation of the loop above
- [Catmull-Rom splines](https://en.wikipedia.org/wiki/Cubic_Hermite_spline#Catmull%E2%80%93Rom_spline), the smoothing that makes the terrain rideable

---

*Fun fact: the most fun I had wasn't riding the bike, it was watching a real market crash render as an actual cliff and realizing my portfolio and my racetrack were, for once, shaped exactly the same.*
