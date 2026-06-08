#!/usr/bin/env bash
#
# optimize-gallery.sh — resize + convert photos into web-ready WebP for the
# site gallery, keeping static/images/gallery in sync with assets/gallery.
#
# Handles HEIC (iPhone), JPEG, PNG. Each photo is downscaled so its longest
# edge is at most MAX_EDGE px, then encoded to WebP at QUALITY. EXIF orientation
# is baked in. Output names are slugified so spaces/markers don't leak to URLs.
#
# Incremental: only re-encodes photos whose source is new/changed, and prunes
# WebP whose source no longer exists — so it's cheap to run on every build.
#
# Source originals live in-repo at assets/gallery (gitignored); only the
# optimized WebP in static/images/gallery is committed and served.
#
# Usage:
#   ./scripts/optimize-gallery.sh [--soft] [SOURCE_DIR]
#
#   --soft       skip cleanly (exit 0) if tools or sources are unavailable,
#                instead of erroring. Used by `make build` / CI / Docker.
#   SOURCE_DIR   defaults to <repo>/assets/gallery
#
# Requirements: sips (macOS), cwebp (`brew install webp`), exiftool
#               (`brew install exiftool`)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MAX_EDGE=1600
QUALITY=80

# --- args -------------------------------------------------------------------
SOFT=0
POSITIONAL=""
for arg in "$@"; do
  case "$arg" in
    --soft) SOFT=1 ;;
    *) POSITIONAL="$arg" ;;
  esac
done
SOURCE_DIR="${POSITIONAL:-$REPO_ROOT/assets/gallery}"
DEST_DIR="$REPO_ROOT/static/images/gallery"

# --- dependency checks ------------------------------------------------------
missing=""
for tool in sips cwebp exiftool; do
  command -v "$tool" >/dev/null 2>&1 || missing="$missing $tool"
done
if [ -n "$missing" ]; then
  if [ "$SOFT" -eq 1 ]; then
    echo "gallery: skipping optimize — missing tools:$missing (committed WebP kept)"
    exit 0
  fi
  echo "error: missing required tools:$missing" >&2
  echo "  install with: brew install webp exiftool" >&2
  exit 1
fi

# --- source dir -------------------------------------------------------------
if [ ! -d "$SOURCE_DIR" ]; then
  if [ "$SOFT" -eq 1 ]; then
    echo "gallery: skipping optimize — no source dir ($SOURCE_DIR) (committed WebP kept)"
    exit 0
  fi
  echo "error: source dir not found: $SOURCE_DIR" >&2
  exit 1
fi

mkdir -p "$DEST_DIR"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# slugify: lowercase, drop extension + trailing " 2" copy marker, spaces->dash
slugify() {
  local base="$1"
  base="${base%.*}"
  base="$(printf '%s' "$base" | tr '[:upper:]' '[:lower:]')"
  base="$(printf '%s' "$base" | sed -E 's/ 2$//; s/[^a-z0-9]+/-/g; s/^-+|-+$//g')"
  printf '%s' "$base"
}

shopt -s nullglob nocaseglob
sources=("$SOURCE_DIR"/*.{jpg,jpeg,png,heic,webp})

# Guard: no source originals (e.g. fresh clone) -> never touch committed WebP.
if [ ${#sources[@]} -eq 0 ]; then
  echo "gallery: no source photos in $SOURCE_DIR — leaving $DEST_DIR untouched."
  exit 0
fi

# Expected output names from the current source set.
expected=()
for src in "${sources[@]}"; do
  expected+=("$(slugify "$(basename "$src")").webp")
done

# Prune WebP whose source no longer exists.
pruned=0
for f in "$DEST_DIR"/*.webp; do
  [ -f "$f" ] || continue
  bn="$(basename "$f")"
  keep=0
  for e in "${expected[@]}"; do
    [ "$e" = "$bn" ] && keep=1 && break
  done
  if [ "$keep" -eq 0 ]; then
    rm -f "$f"
    echo "  - pruned stale $bn"
    pruned=$((pruned + 1))
  fi
done

converted=0
skipped=0
for src in "${sources[@]}"; do
  [ -f "$src" ] || continue
  name="$(basename "$src")"
  slug="$(slugify "$name")"
  out="$DEST_DIR/$slug.webp"

  # Incremental: skip if the WebP exists and is newer than its source.
  if [ -f "$out" ] && [ "$out" -nt "$src" ]; then
    skipped=$((skipped + 1))
    continue
  fi

  # sips decodes HEIC but does NOT bake EXIF orientation, so phone photos come
  # out sideways. Read the real orientation (1-8) and apply the matching
  # rotate/flip ourselves so pixels end up upright with no orientation tag.
  orient="$(exiftool -n -Orientation -s3 "$src" 2>/dev/null)"
  rotate=""
  flip=""
  case "$orient" in
    2) flip="horizontal" ;;
    3) rotate="180" ;;
    4) flip="vertical" ;;
    5) rotate="90"; flip="horizontal" ;;
    6) rotate="90" ;;
    7) rotate="270"; flip="horizontal" ;;
    8) rotate="270" ;;
    *) ;; # 1 or unknown -> leave as-is
  esac

  # sips reads HEIC/JPEG/PNG; rotate/flip, resize, emit intermediate JPEG.
  jpg="$tmp/$slug.jpg"
  sips_args=(-s format jpeg)
  [ -n "$rotate" ] && sips_args+=(-r "$rotate")
  [ -n "$flip" ] && sips_args+=(-f "$flip")
  sips_args+=(-Z "$MAX_EDGE")
  sips "${sips_args[@]}" "$src" --out "$jpg" >/dev/null 2>&1
  cwebp -quiet -q "$QUALITY" "$jpg" -o "$out"

  size="$(du -h "$out" | cut -f1)"
  echo "  ✓ $name -> ${slug}.webp ($size)"
  converted=$((converted + 1))
done
shopt -u nullglob nocaseglob

echo "gallery: $converted converted, $skipped unchanged, $pruned pruned -> $DEST_DIR"
