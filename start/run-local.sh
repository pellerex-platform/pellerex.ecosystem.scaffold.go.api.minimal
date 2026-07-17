#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Starting RepoUniqueNormalisedIdentifier locally...${NC}"

# Change to the parent directory (project root)
cd "$(dirname "$0")/.." || exit 1

# Check if Go is installed
if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go is not installed or not in PATH"
    echo ""
    echo "🔧 Quick Setup Options:"
    echo "   1. Auto-install: ./start/setup-environment.sh"
    echo "   2. Manual install: https://golang.org/dl/"
    echo ""
    echo "📋 Manual Installation Instructions:"
    echo "   macOS: brew install go"
    echo "   Linux: Download from golang.org and extract to /usr/local"
    echo "   Windows: Download installer from golang.org"
    echo ""
    exit 1
fi

# Display Go version
GO_VERSION=$(go version)
echo -e "${GREEN}✅ Go found: $GO_VERSION${NC}"

# Check if go.mod exists
if [ ! -f "go.mod" ]; then
    echo -e "${RED}❌ go.mod not found. Make sure you're in the project root directory.${NC}"
    exit 1
fi

# Set environment variables for development
export ENVIRONMENT=development
export PORT=8890
export DEBUG=true
export LOG_LEVEL=debug

echo -e "${YELLOW}🔧 Environment: $ENVIRONMENT${NC}"
echo -e "${YELLOW}🔧 Port: $PORT${NC}"

# Check for secrets (key-per-file layout, emulating the CSI tmpfs mount)
SECRETS_DIR="$HOME/.pellerex/secrets/RepoUniqueNormalisedIdentifier"
export SECRETS_MOUNT_PATH="$SECRETS_DIR"

if [ ! -d "$SECRETS_DIR" ] || [ -z "$(ls -A "$SECRETS_DIR" 2>/dev/null)" ]; then
    echo -e "${YELLOW}⚠️  Local secrets not found. Running setup-secrets.sh...${NC}"
    ./start/setup-secrets.sh
fi

# Download dependencies
echo -e "${BLUE}📦 Downloading Go modules...${NC}"
if ! go mod download; then
    echo -e "${RED}❌ Failed to download Go modules${NC}"
    exit 1
fi

echo -e "${BLUE}🔧 Tidying Go modules...${NC}"
go mod tidy

# Run the application
echo -e "${GREEN}🌟 Starting the API server...${NC}"
echo -e "${BLUE}API will be available at: http://localhost:$PORT${NC}"
echo -e "${BLUE}Health check: http://localhost:$PORT/health/startup${NC}"
echo -e "${BLUE}Hello endpoint: http://localhost:$PORT/v1/hello${NC}"
echo -e "${BLUE}API docs: http://localhost:$PORT/docs/index.html${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop the server${NC}"
echo ""

# Run the application with hot reload using air if available, otherwise use go run
if command -v air &> /dev/null; then
    echo -e "${GREEN}🔄 Using air for hot reload...${NC}"
    air
else
    echo -e "${YELLOW}💡 Tip: Install 'air' for hot reload: go install github.com/cosmtrek/air@latest${NC}"
    go run main.go
fi
