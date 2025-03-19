PLATFORM_NAME := $(shell uname -p)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

all: up

network:
		docker network inspect deckhouse 2>&1 1>/dev/null || docker network create deckhouse

up: network
		werf compose up --follow --docker-compose-command-options='-d' --env module --repo ":local" --skip-image-spec-stage=true

clean:
		werf cleanup --env module --repo ":local" --skip-image-spec-stage=true --without-kube

down:
		docker rm -f documentation 2>/dev/null

dev: network
		werf compose up --follow --docker-compose-command-options='-d' --dev --env development  --repo ":local" --skip-image-spec-stage=true

.PHONY: up dev
