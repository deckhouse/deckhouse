PLATFORM_NAME := $(shell uname -p)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

all: up

network:
		docker network inspect deckhouse 2>&1 1>/dev/null || docker network create deckhouse

.PHONY:
registry:
		@if ! docker ps | grep -q registry ; then \
			docker rm -f registry 2>/dev/null 1>/dev/null; \
			docker run -d -p 5000:5000 --restart=always --name registry registry:2 ; \
		fi

.PHONY:
registry-down:
		docker rm -f registry; docker volume prune -fa

up: network
		werf compose up --follow --docker-compose-command-options='-d' --env module --repo localhost:5000/docs

down:
		docker rm -f documentation 2>/dev/null

dev: network
		werf compose up --follow --docker-compose-command-options='-d' --dev --env development  --repo localhost:5000/docs

.PHONY: up dev
