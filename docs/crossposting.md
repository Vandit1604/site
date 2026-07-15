# Cross-posting new blog posts to dev.to

Every new blog post gets cross-posted to [dev.to](https://dev.to) with the
canonical URL pointing back to vandit.dev, so the original keeps all SEO credit
and there's no duplicate-content penalty. dev.to is the only target — Hashnode
was dropped because its API now requires a paid Pro plan.

The tool is `scripts/crosspost.go`.

## The rule: never auto-publish

When a new post goes live on vandit.dev, **do not publish it to dev.to
immediately. Ask first, get an explicit go-ahead, then publish.** The tool
enforces this: with no action flag it only previews and sends nothing.

## Workflow for a new post

1. Write and deploy the post (`content/blogs/<slug>.md` live on vandit.dev).
2. Make sure its assets exist on the site:
   - the OG cover card at `static/images/blog/og/<slug>.png`
   - PNG rasters of any inline diagram SVGs (dev.to can't render remote SVG)
3. Preview the conversion (sends nothing):
   ```bash
   go run scripts/crosspost.go -slug <slug> -tags go,devops,networking,programming
   ```
4. **Ask before publishing.**
5. On the go-ahead, publish as a draft to eyeball on dev.to first, or live:
   ```bash
   go run scripts/crosspost.go -slug <slug> -tags go,devops -draft     # dev.to draft
   go run scripts/crosspost.go -slug <slug> -tags go,devops -publish   # live
   ```

## Flags

| Flag | Effect |
|------|--------|
| `-slug <slug>` | which `content/blogs/<slug>.md` to post |
| `-tags a,b,c,d` | up to 4 dev.to tags (falls back to the post's front-matter tags) |
| _(none)_ | **preview only** — prints title/tags/canonical/cover, sends nothing |
| `-draft` | create a dev.to draft (not public) |
| `-publish` | publish live |
| `-list` | list publishable slugs (drafts excluded) |

## What it does to the body

Reads `content/blogs/<slug>.md` and converts on the fly so it renders on dev.to:
callouts → blockquotes, `<figure>` → image + italic caption, source-link anchors
→ markdown links, all `/static` and `/blogs` URLs made absolute, inline diagram
`.svg` → `.png`. Sets `canonical_url`, `main_image` (the OG card), tags, and
description.

## Credentials

`DEVTO_API_KEY` from `.env` (git-ignored). Rotate at dev.to → Settings →
Extensions → DEV Community API Keys.

## The existing 17 posts

The first 17 posts were scheduled on dev.to manually (weekly, from 2026-07-21).
**Do not** run this tool on those — it would create duplicates. This is for new
posts going forward.
