FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/web ./cmd/web
RUN CGO_ENABLED=0 go build -o /bin/worker ./cmd/worker

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ffmpeg python3 && ln -sf /usr/bin/python3 /usr/bin/python && adduser -D -g '' appuser

COPY --from=builder /bin/web /app/web
COPY --from=builder /bin/worker /app/worker
COPY internal/infra/db/migrations /app/internal/infra/db/migrations
COPY internal/transport/http/views/templates /app/internal/transport/http/views/templates
COPY scripts /app/scripts
COPY web/static /app/web/static

RUN mkdir -p /app/data/uploads /app/data/audio && chown -R appuser:appuser /app

EXPOSE 8080
USER appuser

CMD ["/app/web"]
