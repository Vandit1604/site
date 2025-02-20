# Build stage
FROM golang:1.22-alpine AS builder

# Add make
RUN apk add --no-cache make=4.4.1-r2

WORKDIR /app

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code and build the binary
COPY . .
RUN make build

# Final minimal stage
FROM golang:1.22-alpine

# Copy the binary
COPY --from=builder /app/bin/site /bin/site

# Copy required static files and directories
COPY --from=builder /app/assets /assets
COPY --from=builder /app/content/ /content/
COPY --from=builder /app/static /static
COPY --from=builder /app/templates /templates

# Expose the necessary port
EXPOSE 8080

# Set the working directory
WORKDIR /

# Set the entrypoint to the binary
ENTRYPOINT ["/bin/site"]
