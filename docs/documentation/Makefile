export BASE_NGINX_ALPINE=nginx:1.15.12-alpine@sha256:57a226fb6ab6823027c0704a9346a890ffb0cacde06bc19bbc234c8720673555
export BASE_ALPINE=alpine:3.12.1@sha256:c0e9560cda118f9ec63ddefb4a173a2b2a0347082d7dff7dc14272e7841a5b5a
export BASE_GOLANG_16_ALPINE=golang:1.16.3-alpine3.12@sha256:371dc6bf7e0c7ce112a29341b000c40d840aef1dbb4fdcb3ae5c0597e28f3061
export BASE_JEKYLL=jekyll/jekyll:3.8@sha256:9521c8aae4739fcbc7137ead19f91841b833d671542f13e91ca40280e88d6e34

all: up


network:
		docker network inspect deckhouse 2>&1 1>/dev/null || docker network create deckhouse

up: network
		werf compose up --follow --docker-compose-command-options='-d'

down:
		werf compose down

dev: network
		werf compose up --follow --docker-compose-command-options='-d' --dev --env development

.PHONY: up dev
