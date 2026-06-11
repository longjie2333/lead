# syntax=docker/dockerfile:1

FROM golang:1.23-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/lead-server ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/lead-server /app/lead-server
COPY public /app/public

RUN mkdir -p /app/data

ENV HTTP_ADDR=0.0.0.0 \
    PORT=8787 \
    TURN_LISTEN_ADDR=0.0.0.0 \
    TURN_PORT=3478 \
    AUTH_DB_PATH=/app/data/auth.db

EXPOSE 8787/tcp 3478/udp
VOLUME ["/app/data"]

ENTRYPOINT ["/app/lead-server"]
