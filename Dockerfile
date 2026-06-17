# CSS stage: compile Tailwind from the templates into a static stylesheet so
# the runtime CDN never ships to the browser (no FOUC, faster first paint).
FROM node:20-alpine AS css
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY tailwind.config.js ./
COPY static/css/tailwind-input.css ./static/css/tailwind-input.css
COPY templates/ ./templates/
COPY static/js/ ./static/js/
RUN npx tailwindcss -i ./static/css/tailwind-input.css -o ./static/css/tailwind.css --minify

# Build stage
FROM golang:1.22-alpine AS builder
# ca-certificates provides a CA bundle to copy into the scratch image so the
# server can make outbound HTTPS calls (e.g. the Spotify API). Without it a
# scratch container fails TLS verification with "unknown authority".
RUN apk add --no-cache make ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# CGO disabled => fully static binary that runs on scratch.
ENV CGO_ENABLED=0 GOOS=linux
RUN make build

# Final minimal stage
FROM scratch

# Everything the running site needs, baked in:
#   bin/       - the static server binary
#   assets/    - images & svgs referenced by templates
#   content/   - blogs, projects, talks, library data
#   static/    - css, js, optimized gallery webp, robots/sitemap
#   templates/ - html templates
COPY --from=builder /app/bin/ /app/bin/
COPY --from=builder /app/assets/ /app/assets/
COPY --from=builder /app/content/ /app/content/
COPY --from=builder /app/static/ /app/static/
COPY --from=builder /app/templates/ /app/templates/
# CA bundle so outbound HTTPS (Spotify API) can verify certificates on scratch.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
# Freshly compiled Tailwind from the css stage wins over the committed copy.
COPY --from=css /app/static/css/tailwind.css /app/static/css/tailwind.css

WORKDIR /app
EXPOSE 8080

# Production-only, non-sensitive feature flag: notify IndexNow on startup. Lives
# here (committed) rather than in the Coolify UI so deploys need no manual env
# setup; local `make run` never sets it, so dev never submits production URLs.
ENV INDEXNOW_ON_START=1

# Container health check — uses the binary's own -health probe since scratch
# has no shell or curl/wget.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/app/bin/site", "-health"]

ENTRYPOINT ["/app/bin/site"]
