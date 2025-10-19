# ----------------------------------------
# 🧱 Build Stage
# ----------------------------------------
FROM golang:1.24.4-alpine AS builder

# Install build tools and CA certificates
RUN apk add --no-cache build-base ca-certificates curl

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64

RUN go build -ldflags="-w -s" -o /ticket-quality-service ./cmd/server

# ----------------------------------------
# 🧩 Runtime Stage
# ----------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata sqlite

WORKDIR /app

COPY --from=builder /ticket-quality-service /usr/local/bin/ticket-quality-service

ADD https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.28/grpc_health_probe-linux-amd64 /bin/grpc_health_probe
RUN chmod +x /bin/grpc_health_probe

RUN mkdir -p /data && chmod 777 /data

EXPOSE 50051

ENV \
  DB_PATH=/data/database.db \
  REDIS_ADDR=redis-service:6379 \
  GRPC_PORT=50051 \
  APP_ENV=production \
  GRPC_REFLECTION_ENABLED=false

RUN adduser -D appuser
USER appuser

ENTRYPOINT ["/usr/local/bin/ticket-quality-service"]
