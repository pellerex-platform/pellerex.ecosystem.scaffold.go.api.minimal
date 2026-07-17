#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔐 Setting up local secrets for RepoUniqueNormalisedIdentifier...${NC}"

# In production, secrets are mounted key-per-file by the Azure Key Vault CSI
# driver at /mnt/secrets-store (one file per secret). For local development we
# emulate that layout: one file per secret in a local directory, pointed at by
# SECRETS_MOUNT_PATH (see start/run-local.sh).
SECRETS_DIR="$HOME/.pellerex/secrets/RepoUniqueNormalisedIdentifier"

if [ ! -d "$SECRETS_DIR" ]; then
    echo -e "${YELLOW}📁 Creating secrets directory: $SECRETS_DIR${NC}"
    mkdir -p "$SECRETS_DIR"
else
    echo -e "${GREEN}📁 Secrets directory already exists: $SECRETS_DIR${NC}"
fi

# Write one file per secret (key = file name, value = file contents).
write_secret() {
    local key="$1"
    local value="$2"
    if [ ! -f "$SECRETS_DIR/$key" ]; then
        printf '%s' "$value" > "$SECRETS_DIR/$key"
        echo -e "${YELLOW}📝 Created secret: $key${NC}"
    fi
}

write_secret "APISecretKey" "your-secret-key-here-change-in-production"
write_secret "Environment" "development"
write_secret "DbConnectionString" "Server=localhost;Database=RepoUniqueNormalisedIdentifier;User Id=sa;Password=YourPassword123!;TrustServerCertificate=True;"

# Lock down permissions (owner-only).
chmod 700 "$SECRETS_DIR"
chmod 600 "$SECRETS_DIR"/* 2>/dev/null || true

echo -e "${GREEN}🔒 Set permissions (owner read/write only)${NC}"

echo -e "${BLUE}"
echo "📋 Next steps:"
echo "1. Edit the secret files in: $SECRETS_DIR (one file per secret)"
echo "2. Point the app at them:    export SECRETS_MOUNT_PATH=\"$SECRETS_DIR\""
echo "3. Run ./start/run-local.sh to start the development server"
echo -e "${NC}"

echo -e "${GREEN}🎉 Secret setup completed!${NC}"
