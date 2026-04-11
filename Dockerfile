FROM node:22-bookworm-slim AS web-builder

WORKDIR /web

COPY web/package.json web/package-lock.json* ./
RUN npm install

COPY web/ ./
RUN npm run build

FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /disparago ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /disparago /usr/local/bin/disparago
COPY --from=web-builder /web/dist /app/web/dist

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/disparago"]
