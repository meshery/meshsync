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
# Docker-based Builds
#-----------------------------------------------------------------------------
.PHONY: docker-check
## Build Meshsync's docker image
docker: check
	docker build -t layer5/meshery-meshsync .

.PHONY: docker-run
## Runs Meshsync in docker
docker-run:
	(docker rm -f meshery-meshsync) || true
	docker run --name meshery-meshsync -d \
	-p 10007:10007 \
	-e DEBUG=true \
	layer5/meshery-meshsync

PHONY: nats
## Runs a local instance of NATS server in detached mode
nats:
	(docker rm -f nats) || true
	docker run --name nats --rm -p 4222:4222 -p 8222:8222 -d nats --http_port 8222 

#-----------------------------------------------------------------------------
# Local Builds
#-----------------------------------------------------------------------------
.PHONY: build
## Build Meshsync binary to $(MESHSYNC_BINARY_TARGET_RELATIVE)
build:
	go build -o $(MESHSYNC_BINARY_TARGET_RELATIVE) main.go

.PHONY: run-check
## Runs local instance of Meshsync: can be used during local development
run: nats	
	go$(v) mod tidy; \
	DEBUG=true GOPROXY=direct GOSUMDB=off go run main.go

.PHONY: check
## Lint check Meshsync.
check:
	$(GOBIN)/golangci-lint run ./...

.PHONY: go-mod-tidy
## Run go mod tidy for dependency management
go-mod-tidy:
	go mod tidy

#-----------------------------------------------------------------------------
# Tests
#-----------------------------------------------------------------------------

# Test covergae
.PHONY: coverage
## Runs coverage tests for Meshsync
coverage:
	go test -v ./... -coverprofile cover.out
	go tool cover -html=cover.out -o cover.html
## Runs unit tests
test: check 
	go test -failfast --short ./... -race 
## Lint check Golang
lint:
	golangci-lint run ./...

## Runs integration tests check dependencies (if docker, kind, kubectl is present)
integration-tests-check-dependencies:
	./integration-tests/setup.sh check_dependencies

## Runs integration tests set up (runs docker compose with nats and creates a test kind cluster)
## docker compose exposes nats on default ports to host, so they must be available
integration-tests-setup:
	./integration-tests/infrastructure/setup.sh setup

## Runs integration tests clean up (stops docker compose and deletes test cluster)
integration-tests-cleanup:
	./integration-tests/infrastructure/setup.sh cleanup

## Runs integration tests
integration-tests-run: build
	RUN_INTEGRATION_TESTS=true \
	MESHSYNC_BINARY_PATH=$(MESHSYNC_BINARY_TARGET_ABSOLUTE) \
	SAVE_MESHSYNC_OUTPUT=true \
	go test -v -count=1 -run Integration $(INTEGRATION_TESTS_DIR)

## Runs integration tests full cycle (setup, run, cleanup)
integration-tests: integration-tests-setup integration-tests-run integration-tests-cleanup
