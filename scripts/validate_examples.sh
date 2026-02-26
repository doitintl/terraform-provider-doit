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

# Create a temporary CLI config file with dev_overrides to use the local provider
# This tells Terraform to use our locally built binary instead of downloading from registry
TFRC_FILE="$PROVIDER_DIR/.terraformrc-validate"
cat > "$TFRC_FILE" << EOF
provider_installation {
  dev_overrides {
    "doitintl/doit" = "$PROVIDER_DIR"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$TFRC_FILE"

# Verify dev_overrides is working by checking that the local provider is used
echo -e "${YELLOW}Verifying dev_overrides configuration...${NC}"
VERIFY_DIR=$(mktemp -d)
cat > "$VERIFY_DIR/main.tf" << 'EOF'
terraform {
  required_providers {
    doit = {
      source = "doitintl/doit"
    }
  }
}

provider "doit" {}
EOF
cd "$VERIFY_DIR"
VALIDATE_OUTPUT=$(terraform validate 2>&1 || true)
cd "$PROVIDER_DIR"
rm -rf "$VERIFY_DIR"

if echo "$VALIDATE_OUTPUT" | grep -q "Provider development overrides are in effect"; then
    echo -e "${GREEN}✓${NC} Provider dev_overrides active (using locally built provider)"
else
    echo -e "${RED}ERROR: dev_overrides not working!${NC}"
    echo -e "${RED}Expected 'Provider development overrides are in effect' warning.${NC}"
    echo -e "${RED}Output was:${NC}"
    echo "$VALIDATE_OUTPUT" | sed 's/^/  /'
    echo -e "${RED}Check that TF_CLI_CONFIG_FILE is set correctly and not overridden (e.g. by terraform_wrapper).${NC}"
    rm -f "$TFRC_FILE"
    exit 1
fi

# Clean any existing temp files
rm -f "$PROVIDER_DIR/.validate_passed" "$PROVIDER_DIR/.validate_failed"
touch "$PROVIDER_DIR/.validate_passed" "$PROVIDER_DIR/.validate_failed"

# Find all directories containing .tf files
echo -e "${YELLOW}Validating examples...${NC}"
echo ""

find "$EXAMPLES_DIR" -mindepth 2 -name "*.tf" -print0 | xargs -0 -I{} dirname {} | sort -u | while IFS= read -r dir; do
    reldir="${dir#$EXAMPLES_DIR/}"

    # Create a temporary directory for validation
    TEMP_DIR=$(mktemp -d)

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
  }
}

provider "doit" {}
EOF
    fi

    cd "$TEMP_DIR"

    # Try terraform validate directly first — with dev_overrides, init is not
    # needed for the doit provider. If validate fails because a third-party
    # provider is missing (e.g. hashicorp/time), fall back to init + validate.
    VALIDATE_RESULT=$(terraform validate 2>&1) || true
    if echo "$VALIDATE_RESULT" | grep -q "Missing required provider"; then
        # Example uses a third-party provider — install it first
        if ! terraform init -backend=false > /dev/null 2>&1; then
            echo -e "${RED}✗${NC} $reldir (init failed)"
            terraform init -backend=false 2>&1 | grep -i error | head -5 | sed 's/^/  /'
            echo "$reldir" >> "$PROVIDER_DIR/.validate_failed"
            rm -rf "$TEMP_DIR"
            cd "$PROVIDER_DIR"
            continue
        fi
        VALIDATE_RESULT=$(terraform validate 2>&1) || true
    fi

    if echo "$VALIDATE_RESULT" | grep -q "Success"; then
        echo -e "${GREEN}✓${NC} $reldir"
        echo "$reldir" >> "$PROVIDER_DIR/.validate_passed"
    else
        echo -e "${RED}✗${NC} $reldir"
        echo "  Error:"
        echo "$VALIDATE_RESULT" | head -20 | sed 's/^/  /'
        echo "$reldir" >> "$PROVIDER_DIR/.validate_failed"
    fi

    # Cleanup
    rm -rf "$TEMP_DIR"
    cd "$PROVIDER_DIR"
done

# Count results from temp files
PASSED=$(wc -l < "$PROVIDER_DIR/.validate_passed" 2>/dev/null || echo 0)
FAILED=$(wc -l < "$PROVIDER_DIR/.validate_failed" 2>/dev/null || echo 0)

echo ""
echo "================================"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ -f "$PROVIDER_DIR/.validate_failed" ]; then
    echo -e "\nFailed examples:"
    sed 's/^/  - /' "$PROVIDER_DIR/.validate_failed"
fi

# Cleanup temp files
rm -f "$PROVIDER_DIR/.validate_passed" "$PROVIDER_DIR/.validate_failed" "$PROVIDER_DIR/.terraformrc-validate" "$PROVIDER_DIR/terraform-provider-doit"

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi

echo -e "\n${GREEN}All examples validated successfully!${NC}"
