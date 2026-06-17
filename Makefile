.PHONY: build deploy run css gallery optimize-gallery indexnow health docker-build push-to-docker build-linux docker-build-linux docker-buildx-push

# Build compiles Tailwind (soft) and optimizes the gallery (soft) — both skip
# cleanly if their tools are absent (e.g. inside the Go/alpine Docker builder,
# where the dedicated Node stage handles CSS) — then compiles the binary.
build: css optimize-gallery
	@go build -o bin/site

# Compile Tailwind from the templates into static/css/tailwind.css.
# Soft: skips if npx is unavailable, relying on the committed tailwind.css
# and/or the Node stage in the Dockerfile.
css:
	@if command -v npx >/dev/null 2>&1; then \
		npx tailwindcss -i ./static/css/tailwind-input.css -o ./static/css/tailwind.css --minify; \
	else \
		echo "css: npx unavailable — skipping Tailwind build (committed tailwind.css kept)"; \
	fi

deploy: build
	@./bin/site

# Run the server locally
run: build
	@./bin/site

# Soft optimize used by `build` — never fails the build if bash, tools, or
# source photos are unavailable (CI / Docker / fresh clone keep committed WebP).
optimize-gallery:
	@if command -v bash >/dev/null 2>&1; then \
		bash ./scripts/optimize-gallery.sh --soft; \
	else \
		echo "gallery: bash unavailable — skipping optimize (committed WebP kept)"; \
	fi

# Strict optimize for explicit local use — errors if tools are missing.
# Bakes EXIF orientation, resizes, converts to WebP. Requires: sips, cwebp, exiftool.
gallery:
	@./scripts/optimize-gallery.sh

# Notify IndexNow (Bing, Yandex, Seznam, Naver, …) of every sitemap URL.
# Run AFTER deploying so the key file at /<key>.txt is publicly reachable.
indexnow: build
	@./bin/site -indexnow

# Probe a locally running server's health endpoint.
health: build
	@./bin/site -health && echo "healthy"

# Build the production Docker image locally.
docker-build:
	@docker build -t vandit1604/site:latest .

push-to-docker: docker-buildx-push
	@echo "Pushed multi-arch Docker image to registry."
	@echo "Notifying IndexNow once the new key file is live on vandit.dev…"
	@$(MAKE) --no-print-directory indexnow || \
		echo "indexnow: skipped (key file not live yet) — run 'make indexnow' after the host pulls the new image."

# Build a Linux binary (useful when building images for Linux hosts)
build-linux:
	@GOOS=linux GOARCH=amd64 go build -o bin/site-linux

# Build and push a single-platform Linux image (amd64)
docker-build-linux: build-linux
	@docker build --platform linux/amd64 -t vandit1604/site:linux-amd64 .
	@docker push vandit1604/site:linux-amd64

# Build and push a multi-arch image using buildx (requires docker buildx set up)
docker-buildx-push: build-linux
	@docker buildx build --platform linux/amd64,linux/arm64 -t vandit1604/site:latest --push .
