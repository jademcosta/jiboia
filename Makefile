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
	$(GOTEST) -v -race -timeout 60s ./...

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

test-e2e-local: ## Runs tests against a localstack tool
	@cd test && docker compose up -d && cd ..
	@sleep 5
#@curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
#@unzip awscliv2.zip
#@sudo ./aws/install
#@aws --endpoint-url=http://localhost:4566 s3 mb s3://jiboia-local-bucket
#@aws --endpoint-url=http://localhost:4566 sqs create-queue --queue-name jiboia-local-queue
	$(GOCMD) run ./test/e2e-setup/main.go -q jiboia-local-queue
	$(GOCMD) build -o jiboia ./cmd/...
	@./jiboia --config test/config-localstack.yaml &
	@curl -d '{"key1":"first-key", "key2":"another-key!!!"}' -H "Content-Type: application/json" -X POST http://localhost:9099/jiboia-flow/async_ingestion
	$(GOCMD) run ./test/validator/main.go -q http://localhost:4566/000000000000/jiboia-local-queue
#@cd test && docker compose down && cd ..
#@killall jiboia

test-e2e-aws-ci: ## Run tests against AWS, on the CI
# @sed -i "s/<AWS_ACCESS_KEY_ID>/${AWS_ACCESS_KEY_ID}/" ./test/config-aws-ci.yaml
# @sed -i "s/<AWS_SECRET_ACCESS_KEY>/${AWS_SECRET_ACCESS_KEY}/" ./test/config-aws-ci.yaml
# @sed -i "s/<JIBOIA_S3_BUCKET>/${JIBOIA_S3_BUCKET}/" ./test/config-aws-ci.yaml
# echo ${JIBOIA_SQS_URL}
# echo ${JIBOIA_S3_BUCKET}
# sed -i "s/<JIBOIA_SQS_URL>/${JIBOIA_SQS_URL}/" ./test/config-aws-ci.yaml

# cat ./test/config-aws-ci.yaml

	$(GOCMD) build -o jiboia ./cmd/...
	./jiboia --config test/config-aws-ci.yaml &
	@sleep 3
	curl -d '{"key1":"first-key", "key2":"another-key!!!"}' -H "Content-Type: application/json" -X POST http://localhost:9099/jiboia-flow/async_ingestion
	$(GOCMD) run ./test/validator/main.go -q ${JIBOIA_SQS_URL}

coverage: ## Run the tests of the project and export the coverage
	$(GOCMD) clean -testcache
	$(GOTEST) -timeout 30s -cover -covermode=count -coverprofile=profile.cov ./...
	$(GOCMD) tool cover -func profile.cov

## Lint:
lint: ## Run all available linters
	$(GOCMD) install honnef.co/go/tools/cmd/staticcheck@latest
# $(GOCMD) install golang.org/x/lint/golint@latest
	$(GOVET) ./...
	staticcheck ./...
# golint ./...

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
