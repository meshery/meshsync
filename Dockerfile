FROM golang:1.23 AS builder
ARG GIT_VERSION
ARG GIT_COMMITSHA

WORKDIR /build
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
# Copy the go source
COPY . .
# Build
RUN CGO_ENABLED=0 GO111MODULE=on go build -ldflags="-w -s -X main.version=$GIT_VERSION -X main.commitsha=$GIT_COMMITSHA" -a -o meshery-meshsync main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/base-debian10
WORKDIR /
ENV GODISTRO="debian"
COPY --from=builder /build/meshery-meshsync .
ENTRYPOINT ["/meshery-meshsync"]
