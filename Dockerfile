ARG VERSION=dev

FROM golang:1.25-alpine AS builder
ARG VERSION
WORKDIR /app
RUN apk add --no-cache git
# go.mod's local `replace github.com/noggrj/hacktown-fase-5-events =>
# ../fiapx-events` resolves to /fiapx-events from here (WORKDIR /app).
# The "fiapx-events" build context is supplied by docker-compose
# (additional_contexts) for local dev; this whole COPY line goes away
# together with the replace directive once that module is published.
COPY --from=fiapx-events / /fiapx-events
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o server \
    ./cmd/api

FROM alpine:3.20 AS production
# ffmpeg is the whole point of this service — everything else in this
# Dockerfile mirrors the other fiapx-* services.
RUN apk add --no-cache ca-certificates tzdata wget ffmpeg
WORKDIR /app
COPY --from=builder /app/server .

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

EXPOSE 8080
USER nobody
CMD ["./server"]
