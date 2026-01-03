# PocketBase Enterprise Multi-Stage Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -o pocketbase \
    .

# Runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    sqlite

# Create non-root user
RUN addgroup -S pocketbase && adduser -S pocketbase -G pocketbase

# Copy binary
COPY --from=builder /build/pocketbase /usr/local/bin/pocketbase

# Create data directories
RUN mkdir -p /data && chown -R pocketbase:pocketbase /data

USER pocketbase
WORKDIR /data

EXPOSE 7000 8080 8090 8091

ENTRYPOINT ["/usr/local/bin/pocketbase"]
CMD ["serve"]
