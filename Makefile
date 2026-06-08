.PHONY: build deploy run gallery optimize-gallery health docker-build push-to-docker build-linux docker-build-linux docker-buildx-push

# Build optimizes the gallery first (soft: skips cleanly if tools/sources are
# absent, e.g. inside the Docker/alpine builder), then compiles the binary.
build: optimize-gallery
	@go build -o bin/site

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

# Probe a locally running server's health endpoint.
health: build
	@./bin/site -health && echo "healthy"

# Build the production Docker image locally.
docker-build:
	@docker build -t vandit1604/site:latest .

push-to-docker: docker-buildx-push
	@echo "Pushed multi-arch Docker image to registry."

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
