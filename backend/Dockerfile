# ClaraTeach Backend - Multi-stage Dockerfile for Cloud Run
# Builds a minimal container for the Go API server

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary
# CGO_ENABLED=0 for static binary, GOOS=linux for Linux target
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server/

# Runtime stage - minimal image
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS calls (GCP APIs, Neon DB)
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -u 1000 appuser
USER appuser

# Copy binary from builder
COPY --from=builder /app/server /app/server

# Copy migrations (needed for DB setup)
COPY --from=builder /app/migrations /app/migrations

# Cloud Run uses PORT env var (default 8080)
ENV PORT=8080

# Expose the port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the server
CMD ["/app/server"]
