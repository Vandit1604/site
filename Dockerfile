FROM golang:1.22-alpine AS builder

# add make
RUN apk add --no-cache make=4.4.1-r2

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

EXPOSE 8080

RUN make build

CMD ["./bin/site"]

