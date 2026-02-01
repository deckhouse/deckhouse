PLATFORM_NAME := $(shell uname -p)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

.DEFAULT_GOAL := help

registry: ## Start local Docker registry
		@if [ -z "$$(docker ps --filter name=registry --format '{{.Names}}')" ]; then \
			docker rm -f registry 2>/dev/null 1>/dev/null; \
			docker run -d -p 4999:5000 -e REGISTRY_STORAGE_DELETE_ENABLED=true --restart=always --name registry registry:2 ; \
		fi

.PHONY:
registry-down: ## Stop and remove local Docker registry
		docker rm -f registry; docker volume prune -fa

up: ## Build and start the site in production mode
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --env local --repo localhost:4999/docs

clean: ## Clean up werf artifacts
		werf cleanup --env local --repo localhost:4999/docs --without-kube

down: ## Stop and remove all site containers and networks
		docker rm -f site-site-1 site_site_1 site-router-1  site_router_1  site-front-1 site_front_1 site-frontend-1 site_frontend_1 2>/dev/null || true ; docker network rm deckhouse 2>/dev/null || true

dev: ## Build and start the site in development mode
		werf compose up --follow --docker-compose-command-options='-d --force-recreate' --dev --env development --repo localhost:4999/docs

debug: ## Build and start the site in debug mode
		werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'  --dev --env development --repo localhost:4999/docs

regenerate-menu: ## Regenerate navigation menu from API (requires DOC_API_URL and DOC_API_KEY)
		curl --fail --retry-delay 5 -H "Referer: $${DOC_API_REFERER:-https://deckhouse.io}" --retry 5 $${DOC_API_URL:?"Please set URL for API"}?api-key=$${DOC_API_KEY:?"Please set KEY for API"}  -o /tmp/menus.json

		@jq '.header | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty header array!") else ( . | {"topnav": { "en": [{"items": .en}], "ru": [ {"items": .ru}  ]}}) end ' /tmp/menus.json > _data/topnav.json
		@jq '."header-products" | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty header array!") else . end' /tmp/menus.json > _data/topnav-l2-products.json
		@jq '.footer | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty footer array!") else ( . | {"columns": . }) end ' /tmp/menus.json > _data/footer.json
		@jq '.copyright | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty copyright array!") else . end ' /tmp/menus.json > _data/copyright.json

# Regenerate embedded modules metadata file (for local development purposes)
regenerate-metadata: ## Regenerate embedded modules metadata file
		METADATA_SOURCE_DIR=../../ METADATA_TARGET_FILE=backends/docs-builder-template/data/dkp/embedded_modules_metadata.json ../documentation/_tools/modules_generate_metadata.sh
		werf build d8-docs-artifacts --env development --repo localhost:4999/docs
		docker rm -f d8-docs-artifacts
		IMAGE=$$(werf stage image d8-docs-artifacts --env development --repo localhost:4999/docs) && \
		docker create --name d8-docs-artifacts $$IMAGE && \
		docker cp d8-docs-artifacts:/export/editions.json backends/docs-builder-template/data/dkp/ && \
		docker cp d8-docs-artifacts:/export/embedded_modules.json backends/docs-builder-template/data/dkp/ && \
		docker cp d8-docs-artifacts:/export/embedded_modules_metadata.json backends/docs-builder-template/data/dkp/
		docker rm -f d8-docs-artifacts

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: help up dev regenerate-menu
