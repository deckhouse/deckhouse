PLATFORM_NAME := $(shell uname -p)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

all: up

network:
		docker network inspect deckhouse 2>&1 1>/dev/null || docker network create deckhouse

up: network
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --env local

down:
		docker rm -f site_site_1 site_front_1; docker network rm deckhouse

dev: network
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --dev --env development

debug: network
		werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'  --dev --env development

.PHONY: up dev
