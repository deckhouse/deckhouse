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
			docker run -d -p 4999:5000 -e REGISTRY_STORAGE_DELETE_ENABLED=true --restart=always --name registry registry:2 ; \
		fi

.PHONY:
registry-down:
		docker rm -f registry; docker volume prune -fa

clean:
		werf cleanup --env module --repo localhost:4999/docs --without-kube

down:
		docker rm -f documentation 2>/dev/null
