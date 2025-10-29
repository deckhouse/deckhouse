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

up:
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --env local --repo localhost:4999/docs

clean:
		werf cleanup --env local --repo localhost:4999/docs --without-kube

down:
		docker rm -f site-site-1 site_site_1 site-router-1  site_router_1  site-front-1 site_front_1 site-frontend-1 site_frontend_1 2>/dev/null || true ; docker network rm deckhouse 2>/dev/null || true

dev:
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --dev --env development --repo localhost:4999/docs

debug:
		werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'  --dev --env development --repo localhost:4999/docs

regenerate-menu:
		curl --retry-delay 5 -H "Referer: {{- env "DOC_API_REFERER" | default "https://deckhouse.io" }}" --retry 5 ${DOC_API_URL}?api-key=${DOC_API_KEY}  -o /tmp/menus.json

		jq '.header | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty header array!") else ( . | {"topnav": { "en": [{"items": .en}], "ru": [ {"items": .ru}  ]}}) end ' /tmp/menus.json > _data/topnav.json
		jq '."header-products" | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty header array!") else . end' /tmp/menus.json > _data/topnav-l2-products.json
		jq '.footer | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty footer array!") else ( . | {"columns": . }) end ' /tmp/menus.json > _data/footer.json
		jq '.copyright | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty copyright array!") else . end ' /tmp/menus.json > _data/copyright.json

.PHONY: up dev regenerate-menu
