PROJECT_PATH := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

REGISTRY ?= quay.io/philipgough
TAG ?= latest
IMAGE_NAME = hashring-controller:$(TAG)

.PHONY: test
test: unit-test integration-test ## Run all tests

.PHONY: unit-test
unit-test: ## Run unit tests
	go test ./...

.PHONY: integration-test
integration-test: ## Run integration tests
	go test -tags integration -run=TestCreateUpdateDeleteCycleNoCache -run=TestCreateUpdateDeleteCycleWithCache -run=TestCreateUpdateCycle ./...

.PHONY: build-image
build-image: ## Build docker image
	docker build --tag $(REGISTRY)/$(IMAGE_NAME) .
