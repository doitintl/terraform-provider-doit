#!/bin/bash
# Validate all Terraform examples against the provider schema
# This script uses terraform validate to check that all examples
# use correct schema fields and values.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVIDER_DIR="$(dirname "$SCRIPT_DIR")"
EXAMPLES_DIR="$PROVIDER_DIR/examples"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Build the provider first
echo -e "${YELLOW}Building provider...${NC}"
cd "$PROVIDER_DIR"
go build -o terraform-provider-doit .

# Track results
PASSED=0
FAILED=0
FAILED_DIRS=""

# Find all directories containing .tf files
echo -e "${YELLOW}Validating examples...${NC}"
echo ""

for dir in $(find "$EXAMPLES_DIR" -mindepth 2 -name "*.tf" -exec dirname {} \; | sort -u); do
    reldir="${dir#$EXAMPLES_DIR/}"

    # Create a temporary directory for validation
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    # Copy the example files
    cp "$dir"/*.tf "$TEMP_DIR/"

    # Create a minimal provider configuration if not present
    if ! grep -q 'required_providers' "$TEMP_DIR"/*.tf 2>/dev/null; then
        cat > "$TEMP_DIR/provider.tf" << 'EOF'
terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
    time = {
      source = "hashicorp/time"
    }
  }
}

provider "doit" {}
EOF
    fi

    # Run terraform init and validate
    cd "$TEMP_DIR"

    if terraform init -backend=false > /dev/null 2>&1; then
        if terraform validate > /dev/null 2>&1; then
            echo -e "${GREEN}✓${NC} $reldir"
            ((PASSED++))
        else
            echo -e "${RED}✗${NC} $reldir"
            echo "  Error:"
            terraform validate 2>&1 | head -20 | sed 's/^/  /'
            FAILED_DIRS="$FAILED_DIRS\n  - $reldir"
            ((FAILED++))
        fi
    else
        echo -e "${RED}✗${NC} $reldir (init failed)"
        terraform init -backend=false 2>&1 | grep -i error | head -5 | sed 's/^/  /'
        FAILED_DIRS="$FAILED_DIRS\n  - $reldir"
        ((FAILED++))
    fi

    # Cleanup
    rm -rf "$TEMP_DIR"
    trap - EXIT
done

echo ""
echo "================================"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ $FAILED -gt 0 ]; then
    echo -e "\nFailed examples:$FAILED_DIRS"
    exit 1
fi

echo -e "\n${GREEN}All examples validated successfully!${NC}"
