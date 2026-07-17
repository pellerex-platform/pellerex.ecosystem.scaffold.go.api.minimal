#!/bin/bash

# Go Gin API Scaffold - Environment Setup Script
# This script sets up the complete development environment and handles common setup issues

set -e  # Exit on any error

echo "🚀 Setting up Go Gin API Scaffold Environment..."
echo "=================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect operating system
OS="unknown"
case "$(uname -s)" in
    Darwin*)    OS="macOS";;
    Linux*)     OS="Linux";;
    CYGWIN*|MINGW32*|MSYS*|MINGW*) OS="Windows";;
esac

print_status "Detected OS: $OS"

# Install Go based on OS
install_go() {
    print_status "Installing Go 1.23+..."

    case $OS in
        "macOS")
            if command -v brew >/dev/null 2>&1; then
                print_status "Installing Go via Homebrew..."
                brew install go
            else
                print_error "Homebrew not found. Please install from: https://brew.sh/"
                echo "   Then run: brew install go"
                exit 1
            fi
            ;;
        "Linux")
            print_status "Installing Go on Linux..."
            # Remove any existing Go installation
            sudo rm -rf /usr/local/go

            # Download and install Go 1.23
            GO_VERSION="1.23.1"
            print_status "Downloading Go ${GO_VERSION}..."
            curl -L "https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o go.tar.gz
            sudo tar -C /usr/local -xzf go.tar.gz
            rm go.tar.gz

            # Add to PATH if not already there
            if ! echo $PATH | grep -q "/usr/local/go/bin"; then
                echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
                echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.zshrc 2>/dev/null || true
                export PATH=$PATH:/usr/local/go/bin
            fi
            ;;
        "Windows")
            print_warning "Please install Go manually on Windows:"
            echo "   1. Download Go 1.23+ from: https://golang.org/dl/"
            echo "   2. Run the installer"
            echo "   3. Restart your terminal"
            echo "   4. Run this script again"
            exit 1
            ;;
        *)
            print_error "Unsupported OS: $OS"
            echo "   Please install Go 1.23+ manually from: https://golang.org/dl/"
            exit 1
            ;;
    esac
}

# Check if Go is installed and verify version
check_go_version() {
    if command -v go >/dev/null 2>&1; then
        GO_VERSION=$(go version | cut -d' ' -f3)
        print_success "Go is installed: $GO_VERSION"

        # Check Go version (minimum 1.23)
        GO_MAJOR=$(echo $GO_VERSION | sed 's/go//' | cut -d'.' -f1)
        GO_MINOR=$(echo $GO_VERSION | sed 's/go//' | cut -d'.' -f2)

        if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 23 ]); then
            print_warning "Go version $GO_VERSION is older than required (1.23+)"
            print_error "This project requires Go 1.23+ due to dependency requirements"
            print_status "Upgrading Go..."
            install_go
        fi
    else
        print_warning "Go is not installed"
        install_go
    fi
}

print_status "Checking Go installation..."
check_go_version

# Verify Go installation
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version)
    print_success "Go installation verified: $GO_VERSION"
    print_status "Go location: $(which go)"
    print_status "GOPATH: $(go env GOPATH)"
    print_status "GOROOT: $(go env GOROOT)"
else
    print_error "Go installation failed"
    echo "   Please install Go 1.23+ manually from: https://golang.org/dl/"
    exit 1
fi

# Navigate to project directory
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"
print_status "Working in directory: $PROJECT_DIR"

# Clean up any conflicting files
print_status "Cleaning up conflicting files..."
if [ -f "main-broken.go" ]; then
    rm main-broken.go
    print_success "Removed main-broken.go"
fi

if [ -f "main-simple.go" ]; then
    rm main-simple.go
    print_success "Removed main-simple.go"
fi

# Check for main.go
if [ ! -f "main.go" ]; then
    print_error "main.go not found in project directory"
    exit 1
fi

# Initialize Go module if not exists
print_status "Setting up Go module..."
if [ ! -f "go.mod" ]; then
    print_status "Initializing Go module..."
    go mod init RepoUniqueNormalisedIdentifier
    print_success "Go module initialized"
else
    # Verify module name
    MODULE_NAME=$(head -1 go.mod | cut -d' ' -f2)
    if [ "$MODULE_NAME" != "RepoUniqueNormalisedIdentifier" ]; then
        print_warning "Module name is '$MODULE_NAME', expected 'RepoUniqueNormalisedIdentifier'"
        print_status "This might cause import issues"
    fi
    print_success "Go module already exists"
fi

# Fix import paths in source files
print_status "Checking and fixing import paths..."

# Function to fix imports in a file
fix_imports() {
    local file=$1
    if [ -f "$file" ]; then
        if grep -q "<RepoUniqueNormalisedIdentifier>" "$file"; then
            print_status "Fixing tokenized imports in $file"
            sed -i '' 's/<RepoUniqueNormalisedIdentifier>/RepoUniqueNormalisedIdentifier/g' "$file" 2>/dev/null || \
            sed -i 's/<RepoUniqueNormalisedIdentifier>/RepoUniqueNormalisedIdentifier/g' "$file"
            print_success "Fixed imports in $file"
        fi
    fi
}

# Fix imports in all Go files
find . -name "*.go" -not -path "./vendor/*" | while read -r file; do
    fix_imports "$file"
done

# Download dependencies
print_status "Installing Go dependencies..."
if go mod download && go mod tidy; then
    print_success "Dependencies installed successfully"
else
    print_error "Failed to install dependencies"
    exit 1
fi

# Test compilation of individual packages
print_status "Testing package compilation..."

PACKAGES=("config" "handlers" "middleware" "secrets")
for package in "${PACKAGES[@]}"; do
    if [ -d "$package" ]; then
        print_status "Testing $package package..."
        if go build "./$package"; then
            print_success "$package package compiles successfully"
        else
            print_error "$package package compilation failed"
            exit 1
        fi
    fi
done

# Test main application compilation
print_status "Testing main application compilation..."
if go build -o /tmp/test-build .; then
    rm -f /tmp/test-build
    print_success "Main application compiles successfully"
else
    print_error "Main application compilation failed"
    exit 1
fi

# Create .env file if it doesn't exist
if [ ! -f ".env" ] && [ -f ".env.example" ]; then
    print_status "Creating .env file from .env.example..."
    cp .env.example .env
    print_success ".env file created"
fi

# Run quick test
print_status "Running quick application test..."
timeout 5s go run . &
APP_PID=$!
sleep 2

# Test if application started
if ps -p $APP_PID > /dev/null 2>/dev/null; then
    print_success "Application started successfully"
    kill $APP_PID 2>/dev/null || true
    wait $APP_PID 2>/dev/null || true
else
    print_warning "Application test completed (may have exited quickly)"
fi

# Install useful Go tools
print_status "Installing Go development tools..."
go install golang.org/x/tools/gopls@latest 2>/dev/null || print_warning "Failed to install gopls"
go install github.com/air-verse/air@latest 2>/dev/null || print_warning "Failed to install air"

echo ""
print_success "Go Gin API Scaffold environment setup complete! 🎉"
echo "=================================================="
echo ""
echo "📋 What was installed:"
echo "   ✅ Go programming language (1.23+)"
echo "   ✅ Go module initialized (RepoUniqueNormalisedIdentifier)"
echo "   ✅ Project dependencies"
echo "   ✅ Development tools (gopls, air)"
echo "   ✅ Import paths fixed"
echo "   ✅ Compilation verified"
echo ""
echo "🚀 Next steps:"
echo "   1. Run: ./start/setup-secrets.sh (optional, for secret management)"
echo "   2. Run: go run . (to start the API)"
echo "   3. Or: ./start/run-local.sh (using the run script)"
echo "   4. Or: ./start/run-docker.sh (using Docker)"
echo ""
echo "📍 API endpoints:"
echo "   • Health: http://localhost:8890/health/startup"
echo "   • Hello: http://localhost:8890/v1/hello"
echo "   • Swagger: http://localhost:8890/swagger/index.html"
echo ""
echo "💡 For hot reload during development:"
echo "   air"
