# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/lsx-server .

FROM alpine:3.22.4

RUN apk add --no-cache ca-certificates \
    && addgroup -S lsx \
    && adduser -S -G lsx -h /app lsx

WORKDIR /app

COPY --from=build /out/lsx-server /usr/local/bin/lsx-server

RUN mkdir -p /app/data \
    && chown -R lsx:lsx /app

USER lsx

EXPOSE 80

VOLUME ["/app/data"]

ENV LSX_ADDR=:80 \
    LSX_DATA=/app/data/lsx.sqlite3 \
    LSX_PLAIN=true

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1/healthz || exit 1

ENTRYPOINT ["lsx-server"]
