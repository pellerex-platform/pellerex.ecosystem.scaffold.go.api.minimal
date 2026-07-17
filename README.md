# Marketplace Product ID API Scaffold: Minimal Cloud-native Go Gin API
*Enterprise-grade minimal API scaffold*

A minimal, stateless Go + Gin REST API scaffold with comprehensive secret management, Docker support, and CI/CD integration. Perfect for building lightweight, cloud-native APIs without database dependencies.

*This scaffold provides enterprise-grade features while leveraging Go best practices and the high-performance Gin HTTP web framework.*

## 🚀 Quick Start

### Prerequisites
- **Go 1.23+** (required for dependencies)
- macOS, Linux, or Windows
- Docker (for containerized deployment)

### 1. Setup Environment (Recommended)
```bash
# Clone and navigate to the project
cd go-gin.api.scaffold.minimal

# Run the comprehensive setup script
./start/setup-environment.sh
```

This script will:
- ✅ Install/verify Go 1.23+
- ✅ Initialize Go modules
- ✅ Fix import paths
- ✅ Download dependencies
- ✅ Test compilation
- ✅ Install development tools

### 2. Manual Setup (Alternative)
```bash
# Initialize Go modules
go mod init RepoUniqueNormalisedIdentifier
go mod tidy

# Test compilation
go build .
```

### 3. Configure Secrets (Optional)
```bash
# Set up your application secrets
./start/setup-secrets.sh
```

### 4. Run the API

**Local Development:**
```bash
# Start the development server
./start/run-local.sh

# Or directly with Go
go run .

# For hot reload during development
air
```

**Docker (Environment Configurable):**
```bash
# Development mode (default)
./start/run-docker.sh

# Production mode
ENVIRONMENT=production ./start/run-docker.sh

# Custom port
PORT=3000 ./start/run-docker.sh

# Custom environment and port
ENVIRONMENT=test PORT=9000 ./start/run-docker.sh
```

The API will be available at:
- **API:** http://localhost:8890
- **Health Check:** http://localhost:8890/health/startup
- **Sample Endpoint:** http://localhost:8890/v1/hello
- **Swagger Documentation:** http://localhost:8890/swagger/index.html

**Backward Compatibility:**
- http://localhost:8890/api/health/startup
- http://localhost:8890/api/v1/hello

## 📋 Features

### ✅ Minimal & Stateless
- **No database dependencies** - Pure stateless API
- **Secret management** - Secure configuration handling
- **Fast startup** - No database initialization delays
- **Hot reload** - Development server with automatic reloading (using `air`)
- **Docker ready** - Containerized deployment support

### ✅ Production Ready
- **Go + Gin Framework** - High-performance HTTP framework
- **Secret management system** - Local file and cloud-ready
- **Health checks** - Startup, liveness, readiness endpoints
- **API documentation** - Swagger/OpenAPI integration
- **CORS support** - Cross-origin resource sharing
- **Structured JSON logging** - Production-grade logging
- **Graceful shutdown** - Clean application termination
- **Security hardening** - Best practice security measures
- **Docker optimization** - Multi-stage builds, non-root user

### ✅ Development Features
- **Hot reload support** - Auto-restart on file changes (using `air`)
- **Environment-specific configurations** - Dev, test, prod configs
- **Cross-platform scripts** - Works on macOS, Linux, Windows
- **Comprehensive error handling** - Detailed error responses
- **Development tools** - Integrated gopls, debugging support

### ✅ Enterprise Features
- **CI/CD ready** - GitHub Actions, Azure DevOps pipelines
- **Kubernetes deployment** - Helm charts included
- **Environment variables** - 12-factor app compliance
- **Monitoring ready** - Health endpoints for orchestration
- **Tokenization support** - Template-based configuration

## 🔧 Local Development

### Environment Configuration
```bash
# Set environment variables (optional)
export ENVIRONMENT=development  # development, test, production
export PORT=8890               # Server port
export DEBUG=true              # Enable debug logging
export LOG_LEVEL=debug         # Log level
```

### With Hot Reload (Recommended)
```bash
# Hot reload is automatically installed by setup-environment.sh
# Or install manually:
go install github.com/air-verse/air@latest

# Run with hot reload
air
```

### Testing
```bash
# Test individual packages
go build ./config
go build ./handlers
go build ./middleware

# Test main application
go build .

# Run tests
go test ./...
```

## 📁 Project Structure
```
go-gin.api.scaffold.minimal/
├── main.go                    # Application entry point
├── go.mod                     # Go module definition
├── go.sum                     # Go module checksums
├── Dockerfile                 # Container configuration
├── .env.example              # Environment variables template
├── handlers/                  # HTTP request handlers
│   ├── health.go             # Health check endpoints
│   └── hello.go              # API v1 endpoints
├── config/                    # Configuration management
│   ├── config.go             # Config loading and validation
│   ├── config.dev.json       # Development configuration
│   ├── config.prod.json      # Production configuration
│   └── config.test.json      # Test configuration
├── secrets/                   # Secret management
│   └── secrets.go            # Secret loading (local files)
├── middleware/                # HTTP middleware
│   └── middleware.go         # Logging, CORS, security
├── start/                     # Deployment scripts
│   ├── setup-environment.sh  # Complete environment setup
│   ├── setup-secrets.sh      # Secret management setup
│   ├── run-local.sh          # Local development runner
│   └── run-docker.sh         # Docker deployment runner
├── infra/                     # Infrastructure as Code
│   └── helm/                 # Kubernetes Helm charts
└── .github/                   # CI/CD pipelines
    └── workflows/            # GitHub Actions workflows
├── config.*.json             # Environment configurations
├── start/                    # Development scripts
│   ├── setup-secrets.sh     # Secret setup
│   ├── run-local.sh         # Local development
│   └── run-docker.sh        # Docker deployment
├── infrastructure/           # Helm charts, CI/CD
└── Dockerfile               # Container definition
```

## 🔑 Secret Management

This scaffold reads secrets **key-per-file** from a tmpfs mount — the same pattern in local development and production. No secret is ever stored in this repo, the image, config files, a Kubernetes Secret, or an environment variable.

### Local Development
Run `./start/setup-secrets.sh` to create one file per secret under `~/.pellerex/secrets/RepoUniqueNormalisedIdentifier/` and `export SECRETS_MOUNT_PATH` to point at it (handled automatically by `./start/run-local.sh`). Each file's name is the secret key and its contents are the value:

```
~/.pellerex/secrets/RepoUniqueNormalisedIdentifier/
├── APISecretKey         # your-secret-key-here
├── Environment          # development
└── DbConnectionString   # Server=localhost;Database=...;Trusted_Connection=true;
```

> **Note:** `DbConnectionString` is included to demonstrate secret management, even though this minimal scaffold doesn't use a database.

### Production
- Azure Key Vault, mounted into the pod by the Secrets Store CSI driver as tmpfs files at `/mnt/secrets-store` (key-per-file) — the Go equivalent of .NET's `AddKeyPerFile`.
- tmpfs only: secrets never touch etcd or the process environment.
- The only Key Vault reference in the repo is the vault *name* (tokenised per env); the values themselves never appear in the repo or image.

## 🐳 Docker Deployment

### Environment-Aware Docker Deployment
```bash
# Development mode (default)
./start/run-docker.sh

# Production mode  
ENVIRONMENT=production ./start/run-docker.sh

# Custom configuration
ENVIRONMENT=test PORT=9000 ./start/run-docker.sh
```

### Manual Docker Commands
```bash
# Build the image
docker build -t RepoUniqueNormalisedIdentifier .

# Run the container with environment configuration
docker run -d \
    --name RepoUniqueNormalisedIdentifier-api \
    -p 8890:8890 \
    -e ENVIRONMENT=development \
    -e PORT=8890 \
    -e SECRETS_MOUNT_PATH=/mnt/secrets-store \
    -v ~/.pellerex/secrets/RepoUniqueNormalisedIdentifier:/mnt/secrets-store:ro \
    RepoUniqueNormalisedIdentifier
```

## 🏥 Health Checks

The API provides three health check endpoints for orchestration:

- **Startup:** `GET /health/startup` - Application initialization status
- **Liveness:** `GET /health/live` - Application responsiveness  
- **Readiness:** `GET /health/ready` - Ready to serve traffic

**Backward Compatibility:**
- `GET /api/health/startup`
- `GET /api/health/live` 
- `GET /api/health/ready`

Example response:
```json
{
  "status": "alive",
  "timestamp": "2024-09-05T10:30:00Z",
  "service": "RepoUniqueNormalisedIdentifier",
  "version": "1.0.0"
}
```

## 📚 API Documentation

### Endpoints

#### `GET /v1/hello` or `GET /api/v1/hello`
Returns hello message with configuration information.

**Response:**
```json
{
  "message": "Hello from RepoUniqueNormalisedIdentifier!",
  "timestamp": "2024-09-05T10:30:00Z",
  "version": "v1", 
  "environment": "development",
  "score": 110,
  "configuration": {
    "debug": true,
    "cors_allowed_origins": ["*"],
    "log_level": "debug"
  }
}
```

### Interactive Documentation
- **Swagger UI:** http://localhost:8890/swagger/index.html
- **OpenAPI Spec:** http://localhost:8890/swagger/doc.json

## ⚙️ Configuration

### Environment-Specific Configuration Files
## ⚙️ Configuration

### Configuration Files
- `config/config.dev.json` - Development settings
- `config/config.test.json` - Test environment
- `config/config.prod.json` - Production optimizations

### Environment Variables
- `ENVIRONMENT` - Environment name (development, test, production)
- `PORT` - API port (default: 8890)
- `DEBUG` - Enable debug mode (true/false)
- `LOG_LEVEL` - Logging level (debug, info, warn, error)
- `CORS_ALLOWED_ORIGINS` - Allowed CORS origins

## 🚀 Deployment

### Kubernetes Deployment
The scaffold includes complete Kubernetes deployment configurations:

```bash
# Deploy to development
helm install RepoUniqueNormalisedIdentifier ./infra/helm \
    -f ./infra/helm/values.dev.yaml

# Deploy to production
helm install RepoUniqueNormalisedIdentifier ./infra/helm \
    -f ./infra/helm/values.prod.yaml
```

### Container Deployment
```bash
# Build and deploy using provided scripts
# Environment-aware Docker deployment included
./start/run-docker.sh

# Or with custom configuration
ENVIRONMENT=production PORT=8080 ./start/run-docker.sh
```

## 🧪 Testing

### Run Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Manual Testing
```bash
# Test health endpoint
curl http://localhost:8890/health/startup

# Test API endpoint
curl http://localhost:8890/v1/hello

# Test with backward compatibility
curl http://localhost:8890/api/v1/hello
```

### Linting and Code Quality
```bash
# Run linting checks
go vet ./...
go fmt ./...

# Install additional linting tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run
```
```

## 📊 Logging & Observability

Config-as-code plus Serilog-equivalent structured logging — capability parity with the .NET scaffold.

### Configuration (config-as-code)
Precedence: in-code defaults → `config.{ENVIRONMENT}.json` → environment variables. Everything in
the config file (port, debug, `log_level`, CORS, monitoring, logging) takes effect; env vars override
for containers/CI. A missing config file warns and falls back to defaults (non-fatal).

### Logging — three sinks (like .NET Serilog)
Structured logging via Go's stdlib `log/slog`, fanned out to:
1. **Console** — JSON to stdout (always); collected by Kubernetes.
2. **Files** — daily-rolling `log-<date>.log` (text) + `log-<date>.json` (JSON) under
   `logging.file_directory`, pruned after `retention_days`. Under the read-only root filesystem the
   directory is a writable `emptyDir` volume (`/var/log/app`) mounted by the Helm chart.
3. **Azure Application Insights** — every record is bridged to OpenTelemetry logs and exported
   (alongside traces + metrics) to App Insights.

Every record is enriched with `service.name`, `service.version`, `deployment.environment`; each HTTP
request is logged with a generated/propagated `X-Correlation-Id`. The minimum level is config-driven.

### App Insights delivery (GO-D7)
Go has no in-process App Insights SDK, so telemetry flows **app → OTLP → OpenTelemetry Collector →
Azure Monitor**. The Helm chart runs the Collector as a **sidecar** (`otelCollector.enabled: true`)
configured with the `azuremonitor` exporter and the per-env connection string; the app sends OTLP to
it via `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`. To use a shared cluster gateway instead of
a per-pod sidecar, set `otelCollector.enabled: false` and point `OTEL_EXPORTER_OTLP_ENDPOINT` at the
gateway service.

### Logging settings
| Setting | config.{env}.json | Env var | Default |
|---|---|---|---|
| Level | `log_level` | `LOG_LEVEL` | info |
| File sink on | `logging.file_enabled` | `LOG_FILE_ENABLED` | true |
| File directory | `logging.file_directory` | `LOG_FILE_DIRECTORY` | `logs` (local) / `/var/log/app` (cluster) |
| Retention (days) | `logging.retention_days` | `LOG_RETENTION_DAYS` | 31 |

## 🛠️ Troubleshooting

### Common Issues

#### Go Version Compatibility
```bash
# Check Go version (requires 1.23+)
go version

# Upgrade Go on macOS
brew upgrade go

# Upgrade Go on Linux
./start/setup-environment.sh
```

#### Import Path Issues
```bash
# Fix tokenized imports automatically
./start/setup-environment.sh

# Or manually check for RepoUniqueNormalisedIdentifier references
grep -r "RepoUniqueNormalisedIdentifier" . --exclude-dir=.git
```

#### Compilation Errors
```bash
# Test individual packages
go build ./config
go build ./handlers
go build ./middleware

# Clean and rebuild
go clean -cache
go mod tidy
go build .
```

#### Docker Issues
```bash
# Build with no cache
docker build --no-cache -t RepoUniqueNormalisedIdentifier .

# Check container logs
docker logs RepoUniqueNormalisedIdentifier

# Environment configuration
ENVIRONMENT=development PORT=8890 ./start/run-docker.sh
```

### Performance Monitoring
- **Structured Logging**: JSON-formatted logs with correlation IDs
- **Request Tracking**: HTTP request/response logging middleware
- **Health Metrics**: Built-in health check endpoints

### Health Monitoring
- **Kubernetes-ready**: Health probes for orchestration
- **Custom Implementations**: Configurable health check logic
- **Real-time Monitoring**: Integration-ready endpoints

### No Database Setup Required
This scaffold is **stateless** and requires no database setup. Perfect for:
- **Microservices**: Single-responsibility services
- **API Gateways**: Request routing and transformation
- **Serverless Functions**: Stateless compute workloads
- **Demo Applications**: Quick prototypes and POCs

## 🔧 Scaffold Features Applied

This scaffold provides a **production-ready, minimal API** architecture:

### ✅ What's Included
- **Go + Gin Framework**: High-performance HTTP web framework
- **Stateless Design**: Pure API responses without persistent storage
- **Secret Management**: Local file and cloud-ready secret loading
- **Configuration Management**: Environment-specific JSON configurations
- **Docker Support**: Multi-stage builds with environment configuration
- **Health Endpoints**: Kubernetes-ready health probes
- **CORS Support**: Cross-origin resource sharing middleware
- **Structured Logging**: JSON logging with request correlation
- **Hot Reload**: Development server with automatic restart
- **Comprehensive Setup**: Automated environment configuration

### 🎯 Perfect For
- **Microservices Architecture**: Lightweight, focused services
- **API Gateway Pattern**: Request routing and transformation
- **Cloud-native Applications**: Container-ready deployments
- **Serverless Workloads**: Stateless compute functions
- **Rapid Prototyping**: Quick API development and testing

### 🚀 Getting Started Summary
```bash
# 1. One-command setup
./start/setup-environment.sh

# 2. Run locally
go run .

# 3. Or with Docker
./start/run-docker.sh

# 4. Test endpoints
curl http://localhost:8890/health/startup
curl http://localhost:8890/v1/hello
```

---

## 📋 Quick Reference

| Component | Endpoint | Purpose |
|-----------|----------|---------|
| Health Check | `/health/startup` | Application initialization |
| Health Check | `/health/live` | Liveness probe |
| Health Check | `/health/ready` | Readiness probe |
| API | `/v1/hello` | Sample API endpoint |
| Swagger | `/swagger/index.html` | API documentation |

**Environment Configuration:**
- `ENVIRONMENT`: development, test, production
- `PORT`: Server port (default: 8890)
- `LOG_LEVEL`: debug, info, warn, error

---

*Enterprise-grade Go API scaffold for modern cloud-native applications*

With **pellerex**, it only takes less than 2 minutes for your APIs to be up and running on the Cloud, and you can start building your product straight away.

**pellerex** provides your teams with the below capabilities out of the box:
- API scaffolds built in a variety of languages currently Python and .NET
- Versioning of the APIs
- Route documentation provided using Swagger
- Codified configuration management across all different environments
- Secret management backed by secure key-vaults
- Protected endpoints and integration with Pellerex Identity Service
- Rate limiting for your APIs
- Immediate access to an API route to start consuming your repository endpoints
- Fully automated CI/CD pipelines to build and deploy your models and APIs, which will make them available to your clients within a few minutes
- Access to a database of your choice for your APIs
- Availability of large memory for your APIs running heavy-duty AI models

You can find more information on Pellerex.com

---

*Made with ❤️ by [Pellerex.com](pellerex.com/)*
