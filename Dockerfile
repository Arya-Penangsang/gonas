# ============================================================
# Multi-Stage Dockerfile for GoNAS Cloud
# Ultralight Go build (< 20 MB container)
# ============================================================

FROM golang:1.23-alpine AS builder

WORKDIR /src

# Copy dependency definition
COPY go.mod ./
COPY . .

# Compile static Linux binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gonas .

# Runner image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /src/gonas /app/gonas

# Create volume mount points
RUN mkdir -p /storage /app/data

# Default environment configuration
ENV NAS_PORT=8082
ENV NAS_ROOT_PATH=/storage
ENV TZ=Asia/Jakarta

EXPOSE 8082

ENTRYPOINT ["/app/gonas"]
