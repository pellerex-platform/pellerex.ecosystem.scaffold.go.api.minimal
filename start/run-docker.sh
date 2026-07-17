#!/bin/bash

# Go Gin API Docker Runner
#
# Usage:
#   ./run-docker.sh                           # Run in development mode on port 8890
#   ENVIRONMENT=production ./run-docker.sh    # Run in production mode
#   PORT=3000 ./run-docker.sh                 # Run on custom port
#   ENVIRONMENT=test PORT=9000 ./run-docker.sh # Custom environment and port
#
# Environment variables:
#   ENVIRONMENT: development (default), production, test
#   PORT: 8890 (default)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Environment configuration (can be overridden)
ENVIRONMENT=${ENVIRONMENT:-development}
PORT=${PORT:-8890}

echo -e "${BLUE}🐳 Starting RepoUniqueNormalisedIdentifier with Docker...${NC}"
echo -e "${BLUE}📋 Configuration: Environment=$ENVIRONMENT, Port=$PORT${NC}"

# Change to the parent directory (project root)
cd "$(dirname "$0")/.." || exit 1

# Docker image and container names
IMAGE_NAME="RepoUniqueNormalisedIdentifier"
CONTAINER_NAME="RepoUniqueNormalisedIdentifier"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}❌ Docker is not running. Please start Docker and try again.${NC}"
    exit 1
fi

# Check for secrets (key-per-file layout, emulating the CSI tmpfs mount)
SECRETS_DIR="$HOME/.pellerex/secrets/RepoUniqueNormalisedIdentifier"

if [ ! -d "$SECRETS_DIR" ] || [ -z "$(ls -A "$SECRETS_DIR" 2>/dev/null)" ]; then
    echo -e "${YELLOW}⚠️  Local secrets not found. Running setup-secrets.sh...${NC}"
    ./start/setup-secrets.sh
fi

# Stop and remove existing container if it exists
if docker ps -a --format 'table {{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo -e "${YELLOW}🛑 Stopping and removing existing container...${NC}"
    docker stop "$CONTAINER_NAME" > /dev/null 2>&1
    docker rm "$CONTAINER_NAME" > /dev/null 2>&1
fi

# Build the Docker image
echo -e "${BLUE}🔨 Building Docker image: $IMAGE_NAME${NC}"
if docker build -t "$IMAGE_NAME" .; then
    echo -e "${GREEN}✅ Docker image built successfully${NC}"
else
    echo -e "${RED}❌ Docker build failed${NC}"
    exit 1
fi

# Create and run the container
echo -e "${YELLOW}🚀 Starting Docker container with environment: $ENVIRONMENT${NC}"
docker run -d \
    --name "$CONTAINER_NAME" \
    -p "$PORT:$PORT" \
    -v "$SECRETS_DIR:/mnt/secrets-store:ro" \
    -e ENVIRONMENT="$ENVIRONMENT" \
    -e PORT="$PORT" \
    -e GIN_MODE="$([ "$ENVIRONMENT" = "production" ] && echo "release" || echo "debug")" \
    -e SECRETS_MOUNT_PATH="/mnt/secrets-store" \
    "$IMAGE_NAME"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Container started successfully!${NC}"
    echo ""
    echo -e "${BLUE}📋 Container Information:${NC}"
    echo -e "${BLUE}  Name: $CONTAINER_NAME${NC}"
    echo -e "${BLUE}  Image: $IMAGE_NAME${NC}"
    echo -e "${BLUE}  Environment: $ENVIRONMENT${NC}"
    echo -e "${BLUE}  Port: $PORT${NC}"
    echo ""
    echo -e "${GREEN}🌐 API Endpoints:${NC}"
    echo -e "${GREEN}  API: http://localhost:$PORT${NC}"
    echo -e "${GREEN}  Health: http://localhost:$PORT/health/startup${NC}"
    echo -e "${GREEN}  Hello: http://localhost:$PORT/v1/hello${NC}"
    echo -e "${GREEN}  Swagger: http://localhost:$PORT/swagger/index.html${NC}"
    echo ""
    echo -e "${YELLOW}📊 Useful commands:${NC}"
    echo -e "${YELLOW}  View logs: docker logs $CONTAINER_NAME${NC}"
    echo -e "${YELLOW}  Follow logs: docker logs -f $CONTAINER_NAME${NC}"
    echo -e "${YELLOW}  Stop container: docker stop $CONTAINER_NAME${NC}"
    echo -e "${YELLOW}  Remove container: docker rm $CONTAINER_NAME${NC}"
    echo ""

    # Show initial logs
    echo -e "${BLUE}📜 Initial container logs:${NC}"
    docker logs "$CONTAINER_NAME"
else
    echo -e "${RED}❌ Failed to start container${NC}"
    exit 1
fi
