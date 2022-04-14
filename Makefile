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

include install/Makefile.core.mk
include install/Makefile.show-help.mk

#-----------------------------------------------------------------------------
# Docker-based Builds
#-----------------------------------------------------------------------------
## Build MeshSync container.
docker-build: check
	docker build -t layer5/meshery-meshsync .

## Build and run MeshSync container.
docker:
	(docker rm -f meshery-meshsync) || true
	docker run --name meshery-meshsync -d \
	-p 10007:10007 \
	-e DEBUG=true \
	layer5/meshery-meshsync

## Lint MeshSync golang.
check:
	golangci-lint run

## Build and run MeshSync locally.
run: 
	cd meshsync; \
	go mod tidy; DEBUG=true go run meshsync.go