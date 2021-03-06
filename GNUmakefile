default: lint test build

tools: ## Install the tools used to test and build
	@echo "==> Installing build tools"
	go get github.com/ahmetb/govvv
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint

build: ## Build Sherpa for development purposes
	@echo "==> Running $@..."
	govvv build -o sherpa ./cmd -version local -pkg github.com/jrasell/sherpa/pkg/build

test: ## Run the Sherpa test suite with coverage
	@echo "==> Running $@..."
	@go test ./... -cover -v -tags -race \
		"$(BUILDTAGS)" $(shell go list ./... | grep -v vendor)

release: ## Trigger the release build script
	@echo "==> Running $@..."
	@goreleaser --rm-dist

.PHONY: lint
lint: ## Run golangci-lint
	@echo "==> Running $@..."
	golangci-lint run cmd/... pkg/...

HELP_FORMAT="    \033[36m%-25s\033[0m %s\n"
.PHONY: help
help: ## Display this usage information
	@echo "Sherpa make commands:"
	@grep -E '^[^ ]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		sort | \
		awk 'BEGIN {FS = ":.*?## "}; \
			{printf $(HELP_FORMAT), $$1, $$2}'
