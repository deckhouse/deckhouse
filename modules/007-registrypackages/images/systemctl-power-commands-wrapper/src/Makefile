# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO=go
GOFLAGS=
LDFLAGS=-s -w
STRIP=strip

DEV_CONTAINER_IMAGE=golang:1.21-alpine
DEV_CONTAINER_IMAGE_ALT=golang:1.21
DEV_CONTAINER_NAME=wrapper-dev-container

systemctl-power-commands-wrapper: main.go go.mod
	CGO_ENABLED=0 GOOS=linux $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ main.go
	$(STRIP) --strip-unneeded $@ 2>/dev/null || true

all: systemctl-power-commands-wrapper

.PHONY: clean dev-container dev-container-alt test

clean:
	rm -f systemctl-power-commands-wrapper
	docker rm -f $(DEV_CONTAINER_NAME) 2>/dev/null || true

test:
	$(GO) test -v ./...

dev-container:
	docker create --name $(DEV_CONTAINER_NAME) -v $$(pwd):/app -w /app $(DEV_CONTAINER_IMAGE) ash -c "trap : TERM INT; sleep infinity & wait"
	docker start $(DEV_CONTAINER_NAME)
	docker exec -ti $(DEV_CONTAINER_NAME) ash

dev-container-alt:
	docker create --name $(DEV_CONTAINER_NAME) -v $$(pwd):/app -w /app $(DEV_CONTAINER_IMAGE_ALT) bash -c "trap : TERM INT; sleep infinity & wait"
	docker start $(DEV_CONTAINER_NAME)
	docker exec -ti $(DEV_CONTAINER_NAME) bash

