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

CC=gcc
CFLAGS=-I.
CFLAGS+=-W -Wall -Werror
CFLAGS+=-static

STRIP=strip

DEV_CONTAINER_IMAGE=alpine:3.20
DEV_CONTAINER_IMAGE_ALT=alt:p11
DEV_CONTAINER_NAME=wrapper-dev-container

systemctl-power-commands-wrapper: wrapper.c wrapper.h
	$(CC) $^ -o $@ $(CFLAGS)
	$(STRIP) --strip-unneeded -R .comment -R .GCC.command.line $@

all: systemctl-power-commands-wrapper

.PHONY: clean dev-container dev-container-alt

clean:
	rm -f wrapper
	docker rm -f $(DEV_CONTAINER_NAME)

dev-container:
	docker create --name $(DEV_CONTAINER_NAME) -v $$(pwd):/app $(DEV_CONTAINER_IMAGE) ash -c "trap : TERM INT; sleep infinity & wait"
	docker start $(DEV_CONTAINER_NAME)
	docker exec $(DEV_CONTAINER_NAME) ash -c "apk add gcc make musl-dev"
	docker exec -ti $(DEV_CONTAINER_NAME) ash

dev-container-alt:
	docker create --name $(DEV_CONTAINER_NAME) -v $$(pwd):/app $(DEV_CONTAINER_IMAGE_ALT) bash -c "trap : TERM INT; sleep infinity & wait"
	docker start $(DEV_CONTAINER_NAME)
	docker exec $(DEV_CONTAINER_NAME) bash -c "apt-get update -y ; apt-get install gcc make musl-devel musl-devel-static which -y"
	docker exec -ti $(DEV_CONTAINER_NAME) bash

