# Build stage
FROM golang:1.25.11-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application (static binary; -trimpath + stripped for a small,
# reproducible artifact — GO-D15)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o main .

# Runtime stage
FROM alpine:3.20

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create app user
RUN addgroup -g 1001 -S appuser && adduser -S appuser -u 1001

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/main .

# Copy configuration files
COPY --from=builder /app/config.*.json ./

# Ensure the non-root user owns the app directory. Secrets are NOT stored here —
# they are mounted read-only from the CSI tmpfs at /mnt/secrets-store (G11).
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
# Expose the port the app runs on (default 8890, configurable via PORT env var)
EXPOSE 8890

# Set default environment variables (can be overridden at runtime)
ENV ENVIRONMENT=development
ENV PORT=8890
ENV GIN_MODE=debug

# Health check (using configurable port)
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health/startup || exit 1

# Run the application
CMD ["./main"]
