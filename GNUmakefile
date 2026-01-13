default: fmt lint build

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

# Generate OpenAPI models and Terraform resource schemas
# Must be run in order: openapi -> framework -> models
generate:
	cd OpenAPI/1_tfplugingen-openapi && go generate ./...
	cd OpenAPI/2_tfplugingen-framework && go generate ./...
	cd internal/provider/models && go generate ./...

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
	@test -f .envrc.local && . ./.envrc.local; TF_ACC=1 go test -v -timeout 120m ./internal/provider/... -run $(TEST)

.PHONY: fmt lint test testacc testacc-run build install generate
