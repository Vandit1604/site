# Build stage
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache make=4.4.1-r2

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

# Final minimal stage
FROM scratch

COPY --from=builder /app/bin/ /app/bin/
COPY --from=builder /app/assets/ /app/assets/
COPY --from=builder /app/content/ /app/content/
COPY --from=builder /app/static/ /app/static/
COPY --from=builder /app/templates/ /app/templates/

EXPOSE 8080
WORKDIR /
ENTRYPOINT ["/app/bin/site"]

