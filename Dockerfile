# Build stage
FROM golang:1.22-alpine AS builder

# Add make
RUN apk add --no-cache make=4.4.1-r2

WORKDIR /app

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code and build
COPY . .
RUN make build

# Final minimal stage
FROM scratch

# Copy only the binary from the build stage
COPY --from=builder /app/bin/site /bin/site

# Expose the necessary port
EXPOSE 8080

# Set the entrypoint to the binary
ENTRYPOINT ["/bin/site"]

