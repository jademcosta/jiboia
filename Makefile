GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
BINARY_NAME=jiboia
VERSION?=0.0.0

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: all test build vendor

all: help

## Test:
test: ## Run all the tests
	$(GOCMD) clean -testcache
	$(GOTEST) -v -race ./...

# test-fuzz: ## Run fuzzing tests
# 	$(GOCMD) clean -testcache
# 	$(GOTEST) -v -fuzz=Fuzz -timeout 30s -fuzztime=5s ./...

test-integration: ## Runs only fast tests
	$(GOCMD) clean -testcache
	$(GOTEST) -v -timeout 30s -race -run Integration ./...

test-e2e: ## Runs only fast tests
	$(GOCMD) clean -testcache
	$(GOTEST) -v -race -run E2E ./...

test-unit: ## Runs only fast tests
	$(GOCMD) clean -testcache
	$(GOTEST) -v -short -timeout 20s ./...

coverage: ## Run the tests of the project and export the coverage
	$(GOCMD) clean -testcache
	$(GOTEST) -timeout 30s -cover -covermode=count -coverprofile=profile.cov ./...
	$(GOCMD) tool cover -func profile.cov

## Lint:
lint: ## Run all available linters
	$(GOCMD) install golang.org/x/lint/golint@latest
	$(GOVET) ./...
	golint ./...

## Fmt:
fmt: ## Fixes deprecated APIs and formats the code
	$(GOCMD) tool fix .
	$(GOCMD) fmt ./...

## Help:
help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)
