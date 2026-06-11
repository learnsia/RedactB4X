APP_NAME := RedactB4X
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)

GOFLAGS := -trimpath
CGO_ENABLED ?= 0

# Output directories
DIST := dist

.PHONY: all clean test vet build help
.PHONY: build-linux-amd64 build-linux-arm64
.PHONY: build-darwin-amd64 build-darwin-arm64
.PHONY: build-windows-amd64 build-windows-arm64
.PHONY: toGitHub
.PHONY: docs-serve docs-build

all: clean build-all

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}'

clean: ## Remove build artifacts
	rm -rf $(DIST)

test: ## Run all tests
	CGO_ENABLED=1 go test ./...

vet: ## Run static analysis
	go vet ./...

# --- Individual platform targets ---

build-linux-amd64: ## Build for Linux amd64
	@mkdir -p $(DIST)/linux-amd64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/linux-amd64/$(APP_NAME) .

build-linux-arm64: ## Build for Linux arm64
	@mkdir -p $(DIST)/linux-arm64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/linux-arm64/$(APP_NAME) .

build-darwin-amd64: ## Build for macOS amd64
	@mkdir -p $(DIST)/darwin-amd64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/darwin-amd64/$(APP_NAME) .

build-darwin-arm64: ## Build for macOS arm64 (Apple Silicon)
	@mkdir -p $(DIST)/darwin-arm64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=arm64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/darwin-arm64/$(APP_NAME) .

build-windows-amd64: ## Build for Windows amd64
	@mkdir -p $(DIST)/windows-amd64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/windows-amd64/$(APP_NAME).exe .

build-windows-arm64: ## Build for Windows arm64
	@mkdir -p $(DIST)/windows-arm64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=arm64 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/windows-arm64/$(APP_NAME).exe .

# --- Aggregate targets ---

build-linux: build-linux-amd64 build-linux-arm64 ## Build for Linux (amd64 + arm64)

build-darwin: build-darwin-amd64 build-darwin-arm64 ## Build for macOS (amd64 + arm64)

build-windows: build-windows-amd64 build-windows-arm64 ## Build for Windows (amd64 + arm64)

build-all: build-linux build-darwin build-windows ## Build for all platforms

# --- Local development ---

build: ## Build for current platform
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(APP_NAME) .

run: build ## Build and run locally
	./$(APP_NAME)

# --- GitHub release prep ---

toGitHub: build-all ## Prepare GitHub release directory with binaries, docs, source, and tutorials
	@echo "Building GitHub release directory..."
	@rm -rf $(DIST)/github/$(APP_NAME)
	@mkdir -p $(DIST)/github/$(APP_NAME)/dist
	@mkdir -p $(DIST)/github/$(APP_NAME)/docs
	@mkdir -p $(DIST)/github/$(APP_NAME)/src
	@mkdir -p $(DIST)/github/$(APP_NAME)/tutorials
	@cp -r $(DIST)/linux-amd64 $(DIST)/linux-arm64 $(DIST)/darwin-amd64 $(DIST)/darwin-arm64 $(DIST)/windows-amd64 $(DIST)/windows-arm64 $(DIST)/github/$(APP_NAME)/dist/
	@cp -r docs $(DIST)/github/$(APP_NAME)/
	@cp -r redactorpii internal $(DIST)/github/$(APP_NAME)/src/
	@cp main.go go.mod go.sum Makefile $(DIST)/github/$(APP_NAME)/src/
	@cp TUTORIAL.md TUTORIAL.html TUTORIAL.pdf $(DIST)/github/$(APP_NAME)/tutorials/
	@cp README.md $(DIST)/github/$(APP_NAME)/
	@echo "Created $(DIST)/github/$(APP_NAME)/"

# --- Documentation ---

docs-build: ## Build GitBook locally
	gitbook build docs/ $(DIST)/docs

docs-serve: ## Serve GitBook locally (http://localhost:4000)
	gitbook serve docs/
