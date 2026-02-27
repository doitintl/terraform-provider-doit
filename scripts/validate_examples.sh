#!/bin/bash
# Validate all Terraform examples against the provider schema.
#
# Strategy:
#   1. Build the provider binary locally.
#   2. Scan all examples for third-party providers (non-doit) and pre-download
#      them into a local filesystem mirror via `terraform providers mirror`.
#   3. Configure Terraform with:
#        - dev_overrides  → forces the locally-built doit provider (never downloaded)
#        - filesystem_mirror → serves pre-mirrored third-party providers (no per-example downloads)
#        - direct {} fallback → catches any provider not yet in the mirror
#   4. For each example: terraform init -backend=false && terraform validate.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVIDER_DIR="$(dirname "$SCRIPT_DIR")"
EXAMPLES_DIR="$PROVIDER_DIR/examples"
MIRROR_DIR="$PROVIDER_DIR/.terraform-mirror"

# Ensure temp files and built binary are cleaned up on exit (including early failures)
cleanup() {
    rm -f "$PROVIDER_DIR/.validate_passed" "$PROVIDER_DIR/.validate_failed" \
         "$PROVIDER_DIR/.terraformrc-validate" "$PROVIDER_DIR/terraform-provider-doit"
    rm -rf "$MIRROR_DIR"
    [ -n "${MIRROR_CONFIG_DIR:-}" ] && rm -rf "$MIRROR_CONFIG_DIR"
    [ -n "${VERIFY_DIR:-}" ] && rm -rf "$VERIFY_DIR"
    [ -n "${TEMP_DIR:-}" ] && rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ─────────────────────────────────────────────────────────────────────────────
# Step 1: Build the provider
# ─────────────────────────────────────────────────────────────────────────────
echo -e "${YELLOW}Building provider...${NC}"
cd "$PROVIDER_DIR"
go build -o terraform-provider-doit .

# ─────────────────────────────────────────────────────────────────────────────
# Step 2: Mirror third-party providers
# ─────────────────────────────────────────────────────────────────────────────
# Collect all required_providers blocks across examples to find non-doit
# providers. Also detect implicit providers from resource/data prefixes.
echo -e "${YELLOW}Scanning examples for third-party providers...${NC}"

# Create a temporary Terraform config that aggregates all third-party provider
# requirements. We build this by scanning all example .tf files.
MIRROR_CONFIG_DIR=$(mktemp -d)
THIRD_PARTY_FOUND=false

# Collect unique provider sources from required_providers blocks (non-doit).
# Uses awk to only match source= lines inside required_providers { } blocks.
PROVIDERS=$(find "$EXAMPLES_DIR" -name "*.tf" -print0 | \
    xargs -0 awk '/required_providers\s*\{/{in_rp=1} in_rp && /source\s*=/{print} /\}/{if(in_rp) in_rp=0}' 2>/dev/null | \
    grep -v 'doitintl/doit' | \
    sed 's/.*source\s*=\s*"\([^"]*\)".*/\1/' | \
    sort -u)

# Also detect implicit providers from resource/data type prefixes.
# e.g. time_static → hashicorp/time, null_resource → hashicorp/null
# Extracts the resource TYPE (first quoted string), then takes the prefix before "_".
IMPLICIT_PROVIDERS=$(find "$EXAMPLES_DIR" -name "*.tf" -print0 | \
    xargs -0 grep -hE '^\s*(resource|data)\s+"[a-z0-9]+_[a-z0-9]+' 2>/dev/null | \
    sed -E 's/^[[:space:]]*(resource|data)[[:space:]]+"([a-z0-9]+)_.*/\2/' | \
    grep -v '^doit$' | \
    sort -u)

# Build a combined provider requirements file for mirroring
{
    echo 'terraform {'
    echo '  required_providers {'

    # Explicit sources
    for src in $PROVIDERS; do
        name=$(echo "$src" | sed 's|.*/||')
        echo "    $name = { source = \"$src\" }"
        THIRD_PARTY_FOUND=true
    done

    # Implicit providers (assume hashicorp/ namespace)
    for prefix in $IMPLICIT_PROVIDERS; do
        # Only add if not already covered by an explicit source
        if ! echo "$PROVIDERS" | grep -q "/$prefix$"; then
            echo "    $prefix = { source = \"hashicorp/$prefix\" }"
            THIRD_PARTY_FOUND=true
        fi
    done

    echo '  }'
    echo '}'
} > "$MIRROR_CONFIG_DIR/providers.tf"

mkdir -p "$MIRROR_DIR"

if [ "$THIRD_PARTY_FOUND" = true ]; then
    echo -e "${YELLOW}Mirroring third-party providers into local cache...${NC}"
    cat "$MIRROR_CONFIG_DIR/providers.tf" | sed 's/^/  /'

    cd "$MIRROR_CONFIG_DIR"
    if ! terraform providers mirror -platform="$(go env GOOS)_$(go env GOARCH)" "$MIRROR_DIR" 2>&1; then
        echo -e "${RED}ERROR: Failed to mirror third-party providers${NC}"
        cd "$PROVIDER_DIR"
        rm -rf "$MIRROR_CONFIG_DIR"
        exit 1
    fi
    cd "$PROVIDER_DIR"
    echo -e "${GREEN}✓${NC} Third-party providers mirrored successfully"
else
    echo -e "${GREEN}✓${NC} No third-party providers found — mirror step skipped"
fi
rm -rf "$MIRROR_CONFIG_DIR"

# ─────────────────────────────────────────────────────────────────────────────
# Step 3: Configure Terraform CLI
# ─────────────────────────────────────────────────────────────────────────────
# dev_overrides: the locally-built doit provider (never downloaded from registry)
# filesystem_mirror: pre-mirrored third-party providers (downloaded once above)
# direct: fallback for anything not in the mirror (should not be needed)
TFRC_FILE="$PROVIDER_DIR/.terraformrc-validate"
cat > "$TFRC_FILE" << EOF
provider_installation {
  dev_overrides {
    "doitintl/doit" = "$PROVIDER_DIR"
  }
  filesystem_mirror {
    path = "$MIRROR_DIR"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$TFRC_FILE"

# ─────────────────────────────────────────────────────────────────────────────
# Step 4: Verify dev_overrides is working
# ─────────────────────────────────────────────────────────────────────────────
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
    exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# Step 5: Validate each example
# ─────────────────────────────────────────────────────────────────────────────
rm -f "$PROVIDER_DIR/.validate_passed" "$PROVIDER_DIR/.validate_failed"
touch "$PROVIDER_DIR/.validate_passed" "$PROVIDER_DIR/.validate_failed"

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

    # Initialize (installs third-party providers from the filesystem mirror;
    # doit provider is handled by dev_overrides and skipped by init).
    if ! INIT_OUTPUT=$(terraform init -backend=false 2>&1); then
        echo -e "${RED}✗${NC} $reldir (init failed)"
        echo "$INIT_OUTPUT" | grep -i error | head -5 | sed 's/^/  /'
        echo "$reldir" >> "$PROVIDER_DIR/.validate_failed"
        cd "$PROVIDER_DIR"
        rm -rf "$TEMP_DIR"
        continue
    fi

    # Validate
    if VALIDATE_OUTPUT=$(terraform validate 2>&1); then
        echo -e "${GREEN}✓${NC} $reldir"
        echo "$reldir" >> "$PROVIDER_DIR/.validate_passed"
    else
        echo -e "${RED}✗${NC} $reldir"
        echo "  Error:"
        echo "$VALIDATE_OUTPUT" | head -20 | sed 's/^/  /'
        echo "$reldir" >> "$PROVIDER_DIR/.validate_failed"
    fi

    # Cleanup
    cd "$PROVIDER_DIR"
    rm -rf "$TEMP_DIR"
done

# Count results from temp files
PASSED=$(wc -l < "$PROVIDER_DIR/.validate_passed" 2>/dev/null || echo 0)
FAILED=$(wc -l < "$PROVIDER_DIR/.validate_failed" 2>/dev/null || echo 0)

echo ""
echo "================================"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ -s "$PROVIDER_DIR/.validate_failed" ]; then
    echo -e "\nFailed examples:"
    sed 's/^/  - /' "$PROVIDER_DIR/.validate_failed"
fi

# Cleanup temp files (also handled by trap, but explicit for clarity)
rm -f "$PROVIDER_DIR/.validate_passed" "$PROVIDER_DIR/.validate_failed" "$PROVIDER_DIR/.terraformrc-validate" "$PROVIDER_DIR/terraform-provider-doit"
rm -rf "$MIRROR_DIR"

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi

echo -e "\n${GREEN}All examples validated successfully!${NC}"
