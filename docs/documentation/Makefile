PLATFORM_NAME := $(shell uname -p)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

all: up

network:
		docker network inspect deckhouse 2>&1 1>/dev/null || docker network create deckhouse

up: network
		werf compose up --follow --docker-compose-command-options='-d' --env module

down:
		docker rm -f documentation

dev: network
		werf compose up --follow --docker-compose-command-options='-d' --dev --env development

.PHONY: up dev
