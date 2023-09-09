# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)

include install/Makefile.core.mk
include install/Makefile.show-help.mk

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: go-checks
go-checks: go-lint go-fmt go-mod-tidy

.PHONY: go-vet
go-vet:
	go vet ./...

.PHONY: go-lint
go-lint:
	$(GOBIN)/golangci-lint run ./...

.PHONY: go-fmt
go-fmt:
	go fmt ./...

.PHONY: go-mod-tidy
go-mod-tidy:
	./scripts/go-mod-tidy.sh

.PHONY: go-test
go-test:
	./scripts/go-test.sh

.PHONY: check
check: error
	$(GOBIN)/golangci-lint run ./...

.PHONY: docker-check
docker: check
	docker build -t layer5/meshery-meshsync .

.PHONY: docker-run
docker-run:
	(docker rm -f meshery-meshsync) || true
	docker run --name meshery-meshsync -d \
	-p 10007:10007 \
	-e DEBUG=true \
	layer5/meshery-meshsync

.PHONY: run-check
run: check
	go$(v) mod tidy; \
	DEBUG=true GOPROXY=direct GOSUMDB=off go run main.go

.PHONY: error
error:
	go run github.com/layer5io/meshkit/cmd/errorutil -d . analyze -i ./helpers -o ./helpers

 # runs a local instance of nats server in detached mode
PHONY: nats
nats:
	docker run --name nats --rm -p 4222:4222 -p 8222:8222 -d nats --http_port 8222 
