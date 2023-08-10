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
	$(GOTEST) -race -timeout 120s ./...

# test-fuzz: ## Run fuzzing tests
# 	$(GOCMD) clean -testcache
# 	$(GOTEST) -v -fuzz=Fuzz -timeout 30s -fuzztime=5s ./...

test-unit: ## Runs only fast tests
	$(GOCMD) clean -testcache
	$(GOTEST) -short -timeout 20s ./...

test-e2e-aws-ci: ## Run tests against AWS, on the CI
	envsubst < test/config-aws-ci-without-values.yaml > test/config-aws-ci.yaml
	$(GOCMD) build -o jiboia ./cmd/...
	./jiboia --config test/config-aws-ci.yaml &
	@sleep 3
	@$(GOCMD) run ./test/validator/main.go -q ${JIBOIA_SQS_URL}

coverage: ## Run the tests of the project and export the coverage
	$(GOCMD) clean -testcache
	$(GOTEST) -timeout 30s -cover -covermode=count -coverprofile=profile.cov ./...
	$(GOCMD) tool cover -func profile.cov

## Lint:
lint: ## Run all available linters
# $(GOCMD) install honnef.co/go/tools/cmd/staticcheck@latest
# $(GOCMD) install github.com/kisielk/errcheck@latest
# $(GOCMD) install golang.org/x/lint/golint@latest
# $(GOVET) -lostcancel=false ./...
# staticcheck ./...
# errcheck ./...
# golint ./...
# curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

lint-experimental: ## Linters that we are still testings
	@$(GOCMD) install github.com/alexkohler/nakedret/cmd/nakedret@latest
	@$(GOCMD) install github.com/alexkohler/prealloc@latest
	@$(GOCMD) install github.com/ashanbrown/makezero@latest
	@$(GOCMD) install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest

	@fieldalignment -fix ./...
	@nakedret ./...
	@prealloc -set_exit_status ./...
	@makezero -set_exit_status ./...

## Fmt:
fmt: ## Fixes deprecated APIs and formats the code
	$(GOCMD) tool fix .
	$(GOCMD) fmt ./...

## Security:
vuln:
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

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
