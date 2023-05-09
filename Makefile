PROJECT_PATH := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))


.PHONY: test
test: unit-test ## Run all tests

.PHONY: unit-test
unit-test: ## Run unit tests
	go test ./...
