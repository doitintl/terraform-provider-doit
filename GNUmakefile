default: fmt lint lint-tools build

build:
	go build -v ./...

install: build
	go install -v ./...

lint: lint-build
	./custom-gcl run

# Build the custom golangci-lint binary with our module plugins.
# Rebuilds only when the plugin source, config, or go.sum changes.
LINT_SOURCES := $(shell find tools/linters -name '*.go' -not -path '*/testdata/*')
custom-gcl: .custom-gcl.yml .golangci.yml go.sum tools/linters/go.mod tools/linters/go.sum $(LINT_SOURCES)
	golangci-lint custom
	@touch $@

lint-build: custom-gcl

# Lint the linter source code itself with stock linters via custom-gcl.
lint-tools: lint-build
	cd tools/linters && $(CURDIR)/custom-gcl run

# Generate OpenAPI models and Terraform resource schemas
# Must be run in order: extract-inline-schemas -> openapi -> framework -> models
generate:
	go run ./tools/extract-inline-schemas -input OpenAPI/openapi_spec_full.yml -output OpenAPI/openapi_spec_processed.yml -datasources OpenAPI/1_tfplugingen-openapi/datasources.yml -resources OpenAPI/1_tfplugingen-openapi/resources.yml -extra-paths OpenAPI/extra_paths.yml
	cd OpenAPI/1_tfplugingen-openapi && go generate ./...
	cd OpenAPI/2_tfplugingen-framework && go generate ./...
	# Ensure generated JSON files end with a trailing newline (required by pre-commit end-of-file-fixer)
	for f in OpenAPI/2_tfplugingen-framework/output_datasources.json OpenAPI/2_tfplugingen-framework/output_resources.json; do \
		[ -n "$$(tail -c 1 "$$f")" ] && echo >> "$$f"; \
	done
	cd internal/provider/models && go generate .

# Generate the v2 (api-specs) models and Terraform schemas. Independent of `generate`.
# V2_SPEC can point at a local file (e.g. a downloaded external-api-v2-full.yaml)
# to exercise generation while the public spec has no paths; do not commit its output.
# The schema generator rejects an empty config, so it only runs for configs with entries.
V2_SPEC ?= OpenAPI/openapi_spec_v2.yml
V2_FRAMEWORK_DIR := OpenAPI/2_tfplugingen-framework
generate-v2:
	go run ./tools/extract-inline-schemas -input $(V2_SPEC) -output OpenAPI/openapi_spec_v2_processed.yml -datasources OpenAPI/v2/datasources.yml -resources OpenAPI/v2/resources.yml -extra-paths OpenAPI/v2/extra_paths.yml
	@if grep -qE '^  [A-Za-z0-9_]+:$$' OpenAPI/v2/resources.yml; then \
		set -e; \
		go run github.com/doitintl/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi generate -config OpenAPI/v2/resources.yml --output $(V2_FRAMEWORK_DIR)/output_resources_v2.json OpenAPI/openapi_spec_v2_processed.yml; \
		(cd $(V2_FRAMEWORK_DIR) && go run github.com/doitintl/terraform-plugin-codegen-framework/cmd/tfplugingen-framework generate resources --input output_resources_v2.json --output ../../internal/provider); \
		[ -n "$$(tail -c 1 $(V2_FRAMEWORK_DIR)/output_resources_v2.json)" ] && echo >> $(V2_FRAMEWORK_DIR)/output_resources_v2.json || true; \
	else echo "v2: no resources configured, skipping resource schemas"; fi
	@if grep -qE '^  [A-Za-z0-9_]+:$$' OpenAPI/v2/datasources.yml; then \
		set -e; \
		go run github.com/doitintl/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi generate -config OpenAPI/v2/datasources.yml --output $(V2_FRAMEWORK_DIR)/output_datasources_v2.json OpenAPI/openapi_spec_v2_processed.yml; \
		(cd $(V2_FRAMEWORK_DIR) && go run github.com/doitintl/terraform-plugin-codegen-framework/cmd/tfplugingen-framework generate data-sources --input output_datasources_v2.json --output ../../internal/provider); \
		[ -n "$$(tail -c 1 $(V2_FRAMEWORK_DIR)/output_datasources_v2.json)" ] && echo >> $(V2_FRAMEWORK_DIR)/output_datasources_v2.json || true; \
	else echo "v2: no data sources configured, skipping data source schemas"; fi
	cd internal/provider/models/v2 && go generate .

# Generate provider documentation from templates
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name doit --rendered-provider-name terraform-provider-doit

# Validate provider documentation is up-to-date and structurally valid
validate-docs: docs
	@if [ -n "$$(git status --porcelain -- docs/)" ]; then \
		echo "ERROR: Docs are out of date. Run 'make docs' and commit the changes."; \
		git status --short -- docs/; \
		exit 1; \
	fi
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate --provider-name doit

fmt:
	gofmt -s -w -e .

# Unit suite only. All provider credentials, host configs, and TF_ACC are
# explicitly cleared so unit tests execute in a cleanroom environment even if
# .envrc.local is loaded in the developer's shell.
test:
	TF_ACC= DOIT_API_TOKEN= DOIT_HOST= DOIT_CUSTOMER_CONTEXT= DOIT_REQUEST_TIMEOUT= \
	go test -v -cover -timeout=120s -parallel=10 -count=3 -shuffle=on ./...

# gotestsum wraps `go test` and automatically reruns individual failed tests,
# which tames acceptance-test flakiness caused by the upstream API. Pinned and
# invoked via `go run` so no separate install is needed locally.
GOTESTSUM := go run gotest.tools/gotestsum@v1.13.0

# Helper to load .envrc.local for acceptance tests.
# In secondary git worktrees, loads .envrc.local from the main repository first,
# then allows local overrides in the current worktree if .envrc.local exists.
LOAD_ACC_ENV := \
	main_git_dir="$$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null)"; \
	repo_root="$$(git rev-parse --show-toplevel 2>/dev/null)"; \
	if [ -n "$$main_git_dir" ] && [ "$$main_git_dir" != "$$repo_root/.git" ] && [ -f "$$main_git_dir/../.envrc.local" ]; then \
		. "$$main_git_dir/../.envrc.local"; \
	fi; \
	if [ -f .envrc.local ]; then \
		. ./.envrc.local; \
	fi

# Run acceptance tests (loads environment from .envrc.local)
# --rerun-fails retries each failed test up to N times; --rerun-fails-max-failures
# is a circuit breaker that skips reruns entirely if the initial run has many
# failures (i.e. a systemic break rather than flakiness).
testacc:
	@$(LOAD_ACC_ENV); \
	TF_ACC=1 $(GOTESTSUM) \
		--format standard-verbose \
		--rerun-fails=2 \
		--rerun-fails-max-failures=5 \
		--rerun-fails-report=/tmp/testacc-reruns.txt \
		--packages='./...' \
		--junitfile test-results.xml \
		-- -cover -timeout 120m -run '^TestAcc'

# Run a specific acceptance test
# Usage: make testacc-run TEST=TestAccBudget
# Regex anchors work: make testacc-run TEST='TestAccA$|TestAccB$'
# ($(value TEST) keeps the pattern literal; $(TEST) would let make eat the "$".)
testacc-run:
	@$(LOAD_ACC_ENV); \
	TF_ACC=1 $(GOTESTSUM) \
		--format standard-verbose \
		--rerun-fails=2 \
		--rerun-fails-max-failures=5 \
		--rerun-fails-report=/tmp/testacc-reruns.txt \
		--packages='./internal/provider/...' \
		-- -timeout 120m -run '$(value TEST)'

# Validate all examples against the provider schema
validate-examples:
	./scripts/validate_examples.sh

# Apply diff from a specific upstream PR: make apply-pr PR=61514
apply-pr:
	@if [ -z "$(PR)" ]; then echo "Error: PR number required (e.g. make apply-pr PR=61514)"; exit 1; fi
	gh pr diff $(PR) --repo doiteng/omni | \
		sed 's|services/external-api/openapi\.yaml|OpenAPI/openapi_spec_full.yml|g' | \
		git apply --include='OpenAPI/openapi_spec_full.yml' -v

# Fetch full spec from a PR or ref: make sync-spec PR=61514 or make sync-spec REF=dev
sync-spec:
	@if [ -n "$(PR)" ]; then \
		gh api "repos/doiteng/omni/contents/services/external-api/openapi.yaml?ref=pull/$(PR)/head" \
			-H "Accept: application/vnd.github.raw" > OpenAPI/openapi_spec_full.yml && \
		echo "Synced OpenAPI/openapi_spec_full.yml from doiteng/omni PR #$(PR)"; \
	else \
		REF=$${REF:-dev}; \
		gh api "repos/doiteng/omni/contents/services/external-api/openapi.yaml?ref=$$REF" \
			-H "Accept: application/vnd.github.raw" > OpenAPI/openapi_spec_full.yml && \
		echo "Synced OpenAPI/openapi_spec_full.yml from doiteng/omni ref: $$REF"; \
	fi

# Fetch the public v2 spec from an api-specs release: make sync-spec-v2 VERSION=1.0.1
# (default: the external-api-v2-latest release). Requires read access to doiteng/api-specs.
sync-spec-v2:
	@TAG=external-api-v2-$${VERSION:-latest}; \
	gh release download "$$TAG" --repo doiteng/api-specs -p external-api-v2.yaml -O OpenAPI/openapi_spec_v2.yml --clobber && \
	echo "Synced OpenAPI/openapi_spec_v2.yml from doiteng/api-specs release $$TAG"

.PHONY: fmt lint lint-build lint-tools test testacc testacc-run build install generate generate-v2 docs validate-docs validate-examples apply-pr sync-spec sync-spec-v2
