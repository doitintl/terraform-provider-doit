default: fmt lint build

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

# Generate OpenAPI models and Terraform resource schemas
# Must be run in order: extract-inline-schemas -> openapi -> framework -> models
generate:
	go run ./tools/extract-inline-schemas -input OpenAPI/openapi_spec_full.yml -output OpenAPI/openapi_spec_processed.yml
	cd OpenAPI/1_tfplugingen-openapi && go generate ./...
	cd OpenAPI/2_tfplugingen-framework && go generate ./...
	cd internal/provider/models && go generate ./...

# Generate provider documentation from templates
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate

# Validate provider documentation is up-to-date and structurally valid
validate-docs: docs
	@if [ -n "$$(git status --porcelain -- docs/)" ]; then \
		echo "ERROR: Docs are out of date. Run 'make docs' and commit the changes."; \
		git status --short -- docs/; \
		exit 1; \
	fi
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

# Run acceptance tests (loads environment from .envrc.local)
testacc:
	@test -f .envrc.local && . ./.envrc.local; TF_ACC=1 go test -v -cover -timeout 120m ./...

# Run a specific acceptance test
# Usage: make testacc-run TEST=TestAccBudget
testacc-run:
	@test -f .envrc.local && . ./.envrc.local; TF_ACC=1 go test -v -timeout 120m ./internal/provider/... -run '$(TEST)'

# Validate all examples against the provider schema
validate-examples:
	./scripts/validate_examples.sh

.PHONY: fmt lint test testacc testacc-run build install generate docs validate-docs validate-examples
