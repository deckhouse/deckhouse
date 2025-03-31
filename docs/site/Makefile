PLATFORM_NAME := $(shell uname -p)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

all: up

registry:
		@if ! docker ps | grep -q registry ; then \
			docker rm -f registry 2>/dev/null 1>/dev/null; \
			docker run -d -p 4999:5000 -e REGISTRY_STORAGE_DELETE_ENABLED=true --restart=always --name registry registry:2 ; \
		fi

.PHONY:
registry-down:
		docker rm -f registry; docker volume prune -fa

network: registry
		docker network inspect deckhouse 2>&1 1>/dev/null || docker network create deckhouse

up: network
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --env local --repo localhost:4999/docs

clean:
		werf cleanup --env local --repo localhost:4999/docs --without-kube

down:
		docker rm -f site-site-1 site-front-1 site_site_1 site_front_1 2>/dev/null; docker network rm deckhouse

dev: network
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --dev --env development --repo localhost:4999/docs

debug: network
		werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'  --dev --env development --repo localhost:4999/docs

.PHONY: up dev
