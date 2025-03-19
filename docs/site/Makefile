PLATFORM_NAME := $(shell uname -p)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

all: up

network:
		docker network inspect deckhouse 2>&1 1>/dev/null || docker network create deckhouse

up: network
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --env local --repo ":local" --skip-image-spec-stage=true

clean:
		werf cleanup --env local --repo localhost:4999/docs --without-kube

down:
		docker rm -f site-site-1 site-front-1 site_site_1 site_front_1 2>/dev/null; docker network rm deckhouse

dev: network
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --dev --env development --repo ":local" --skip-image-spec-stage=true

debug: network
		werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'  --dev --env development --repo ":local" --skip-image-spec-stage=true

.PHONY: up dev
