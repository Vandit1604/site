# Build stage
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache make

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

WORKDIR /app
EXPOSE 8080

# Container health check — uses the binary's own -health probe since scratch
# has no shell or curl/wget.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/app/bin/site", "-health"]

ENTRYPOINT ["/app/bin/site"]
