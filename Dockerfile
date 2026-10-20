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

# Unit test stage: the unit tests with coverage, on the build stage's source and toolchain.
# The build runs it by building the unit-test-results target before the image. The default
# build never runs it, because the runtime stage stays last.
FROM builder AS unit-tests

# Go's own tools write neither JUnit nor Cobertura XML: gotestsum (Apache-2.0) writes the
# JUnit file and gocover-cobertura (MIT) converts Go's coverage profile.
RUN go install gotest.tools/gotestsum@v1.13.0 \
    && go install github.com/boumenot/gocover-cobertura@v1.5.0

# This RUN always succeeds, so the results are kept when a test fails: the test command's
# exit code goes into a file instead, and the build stops after uploading the results when
# it is not 0. A coverage file that cannot be made is removed rather than left half written,
# so the test results are still read, without coverage. Coverage counts every line any test
# runs, whichever package the test is in (-coverpkg=./...), as in the other languages, and
# only lines that hold code count as lines (-ignore-non-code-lines leaves out blank lines,
# comments and closing braces).
RUN mkdir -p /unit-test-results/test-coverage/unit-tests /unit-test-results/test-coverage/coverage; \
    gotestsum --junitfile /unit-test-results/test-coverage/unit-tests/junit.xml -- -coverpkg=./... -coverprofile=/tmp/coverage.out ./...; \
    echo "$?" > /unit-test-results/unit-tests-exit-code; \
    gocover-cobertura -ignore-non-code-lines < /tmp/coverage.out > /unit-test-results/test-coverage/coverage/cobertura.xml \
        || rm -f /unit-test-results/test-coverage/coverage/cobertura.xml

# Only the results, so exporting this target gives test-coverage/ and unit-tests-exit-code
# at the root of the export folder.
FROM scratch AS unit-test-results
COPY --from=unit-tests /unit-test-results /

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
# Expose the port the app runs on (the <port-number> token; overridable via PORT env var)
EXPOSE <port-number>

# Set default environment variables (can be overridden at runtime)
ENV ENVIRONMENT=development
ENV PORT=<port-number>
ENV GIN_MODE=debug

# Health check (using configurable port)
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health/startup || exit 1

# Run the application
CMD ["./main"]
