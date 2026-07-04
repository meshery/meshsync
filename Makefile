# Copyright Meshery Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)

include install/Makefile.core.mk
include install/Makefile.show-help.mk

CURRENT_DIR:=$(shell pwd)
MESHSYNC_BINARY_TARGET_RELATIVE:=bin/meshsync
MESHSYNC_BINARY_TARGET_ABSOLUTE:=$(CURRENT_DIR)/$(MESHSYNC_BINARY_TARGET_RELATIVE)
INTEGRATION_TESTS_DIR:=$(CURRENT_DIR)/integration-tests

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

#-----------------------------------------------------------------------------
# Docker
#-----------------------------------------------------------------------------
.PHONY: docker-build
## Build the MeshSync Docker image
docker-build: lint-run
	docker build -t meshery/meshery-meshsync .

.PHONY: docker-run
## Run MeshSync in a Docker container
docker-run:
	(docker rm -f meshery-meshsync) || true
	docker run --name meshery-meshsync -d \
	-p 10007:10007 \
	-e DEBUG=true \
	meshery/meshery-meshsync

#-----------------------------------------------------------------------------
# Local Development
#-----------------------------------------------------------------------------
.PHONY: build
## Build the MeshSync binary to bin/meshsync
build:
	go build -o $(MESHSYNC_BINARY_TARGET_RELATIVE) main.go

.PHONY: run
## Run MeshSync locally against a NATS server, for local development
run: nats-run
	go mod tidy; \
	DEBUG=true GOPROXY=direct GOSUMDB=off go run main.go

.PHONY: nats-run
## Run a local NATS server in a detached Docker container
nats-run:
	(docker rm -f nats) || true
	docker run --name nats --rm -p 4222:4222 -p 8222:8222 -d nats --http_port 8222

.PHONY: mod-tidy
## Tidy Go module dependencies
mod-tidy:
	go mod tidy

#-----------------------------------------------------------------------------
# Quality & Tests
#-----------------------------------------------------------------------------
.PHONY: lint-run
## Lint the codebase with golangci-lint
lint-run:
	$(GOBIN)/golangci-lint run ./...

.PHONY: test
## Run unit tests with the race detector (lints first)
test: lint-run
	go test -failfast --short ./... -race

.PHONY: coverage-report
## Run unit tests and write an HTML coverage report to cover.html
coverage-report:
	go test -v ./... -coverprofile cover.out
	go tool cover -html=cover.out -o cover.html

#-----------------------------------------------------------------------------
# Integration Tests
#-----------------------------------------------------------------------------
.PHONY: integration-tests-check-dependencies
## Check integration-test dependencies are present (docker, kind, kubectl)
integration-tests-check-dependencies:
	./integration-tests/setup.sh check_dependencies

.PHONY: integration-tests-setup
## Set up integration tests (NATS via docker compose plus a kind cluster)
integration-tests-setup:
	./integration-tests/infrastructure/setup.sh setup

.PHONY: integration-tests-cleanup
## Clean up integration tests (stop docker compose, delete the kind cluster)
integration-tests-cleanup:
	./integration-tests/infrastructure/setup.sh cleanup

.PHONY: integration-tests-run
## Run the integration tests against the meshsync binary
integration-tests-run: build
	RUN_INTEGRATION_TESTS=true \
	MESHSYNC_BINARY_PATH=$(MESHSYNC_BINARY_TARGET_ABSOLUTE) \
	SAVE_MESHSYNC_OUTPUT=true \
	go test -v -count=1 -run Integration $(INTEGRATION_TESTS_DIR)

.PHONY: integration-tests
## Run the full integration-test cycle (setup, run, cleanup)
integration-tests: integration-tests-setup integration-tests-run integration-tests-cleanup
