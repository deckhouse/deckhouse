PLATFORM_NAME := $(shell uname -p)
ifneq ($(filter arm%,$(PLATFORM_NAME)),)
	export WERF_PLATFORM=linux/amd64
endif

.DEFAULT_GOAL := help

ROOT_DIR := ../..
WERF_BIN := $(ROOT_DIR)/bin/werf
MODULE_PATH ?=
CHANNEL ?= alpha
MODULE_VERSION ?= v0.1.0

WERF_REPO ?= :local
ifeq ($(USE_LOCALHOST_REPO),1)
WERF_REPO := localhost:4999/docs
endif

werf-bin: ## Install/update werf binary from root Makefile
		$(MAKE) -C $(ROOT_DIR) bin/werf

registry: ## Start local Docker registry (set USE_LOCALHOST_REPO=1)
		@if [ -z "$$(docker ps --filter name=registry --format '{{.Names}}')" ]; then \
			docker rm -f registry 2>/dev/null 1>/dev/null; \
			docker run -d -p 4999:5000 -e REGISTRY_STORAGE_DELETE_ENABLED=true --restart=always --name registry registry:2; \
			echo "✅ Local Docker registry started on port 4999"; \
		else \
			echo "ℹ️ Local Docker registry is already running"; \
		fi;

.PHONY:
registry-down: ## Stop and remove local Docker registry
		docker rm -f registry; docker volume prune -fa

up: werf-bin ## Start documentation containers in a werf watch mode (rebuilding on a commit)
		@if [ "$(USE_LOCALHOST_REPO)" = "1" ]; then \
			$(MAKE) registry; \
			echo "\nStarting documentation in a watch mode using $(WERF_REPO) registry..."; \
		else \
			echo "\nStarting documentation in a watch mode using local docker images store..."; \
		fi
		@echo -n "\nwerf: "; $(WERF_BIN) version
		@$(WERF_BIN) compose up --follow --docker-compose-command-options='-d --force-recreate' --env local --repo $(WERF_REPO) --skip-image-spec-stage=true

dev: werf-bin ## Start documentation containers in a werf watch mode (rebuilding on any documentation changes)
		@if [ "$(USE_LOCALHOST_REPO)" = "1" ]; then \
			$(MAKE) registry; \
			echo "\nStarting documentation in a DEV watch mode using $(WERF_REPO) registry..."; \
		else \
			echo "\nStarting documentation in a DEV watch mode using local docker images store..."; \
		fi
		@echo -n "\nwerf: "; $(WERF_BIN) version
		@$(WERF_BIN) compose up --follow --docker-compose-command-options='-d --force-recreate' --dev --env development --repo $(WERF_REPO) --skip-image-spec-stage=true

external-module: werf-bin ## Build external module docs and run the local portal
		##~ Options: MODULE_PATH=/path/to/module [CHANNEL=alpha] [MODULE_VERSION=v0.1.0]
		@if [ "$(USE_LOCALHOST_REPO)" = "1" ]; then \
			$(MAKE) registry; \
			echo "\nStarting external module documentation using $(WERF_REPO) registry..."; \
		else \
			echo "\nStarting external module documentation using local docker images store..."; \
		fi
		MODULE_PATH="$(MODULE_PATH)" CHANNEL="$(CHANNEL)" MODULE_VERSION="$(MODULE_VERSION)" $(ROOT_DIR)/tools/docs/external-module-docs.sh
		@echo -n "\nwerf: "; $(WERF_BIN) version
		@$(WERF_BIN) compose up --docker-compose-command-options='-d' --env local --repo $(WERF_REPO) --skip-image-spec-stage=true
		@echo "Open http://localhost/products/kubernetes-platform/documentation/v1/ to access DKP documentation..."

clean: werf-bin ## Clean up werf artifacts (set USE_LOCALHOST_REPO=1)
		@if [ "$(USE_LOCALHOST_REPO)" = "1" ]; then \
			$(MAKE) registry; \
			$(WERF_BIN) cleanup --env local --repo $(WERF_REPO) --without-kube; \
		else \
			echo "Cleanup can be used only with images repo. Set USE_LOCALHOST_REPO=1 to use cleanup with the local registry."; \
		fi

down: ## Stop and remove all site containers and networks
		docker rm -f site-site-1 site_site_1 site-router-1  site_router_1  site-front-1 site_front_1 site-frontend-1 site_frontend_1 2>/dev/null || true ; docker network rm deckhouse 2>/dev/null || true
		$(MAKE) registry-down
		@echo "❌ Local Docker registry has been stopped"

regenerate-menu: ## Regenerate navigation menu from API (requires DOC_API_URL and DOC_API_KEY)
		curl --fail --retry-delay 5 -H "Referer: $${DOC_API_REFERER:-https://deckhouse.io}" --retry 5 $${DOC_API_URL:?"Please set URL for API"}?api-key=$${DOC_API_KEY:?"Please set KEY for API"}  -o /tmp/menus.json

		@jq '.header | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty header array!") else ( . | {"topnav": { "en": [{"items": .en}], "ru": [ {"items": .ru}  ]}}) end ' /tmp/menus.json > _data/topnav.json
		@jq '."header-products" | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty header array!") else . end' /tmp/menus.json > _data/topnav-l2-products.json
		@jq '.footer | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty footer array!") else ( . | {"columns": . }) end ' /tmp/menus.json > _data/footer.json
		@jq '.copyright | if ( ( .en | length) < 1 ) or ( ( .ru | length) < 1 ) then error("Got empty copyright array!") else . end ' /tmp/menus.json > _data/copyright.json

# Regenerate embedded modules metadata file (for local development purposes)
regenerate-metadata: werf-bin ## Regenerate embedded modules metadata file
		METADATA_SOURCE_DIR=../../ METADATA_TARGET_FILE=backends/docs-builder-template/data/dkp/embedded_modules_metadata.json ../documentation/_tools/modules_generate_metadata.sh
		$(WERF_BIN) build d8-docs-artifacts --env development --repo $(WERF_REPO)
		docker rm -f d8-docs-artifacts
		IMAGE=$$($(WERF_BIN) stage image d8-docs-artifacts --env development --repo $(WERF_REPO)) && \
		docker create --name d8-docs-artifacts $$IMAGE && \
		docker cp d8-docs-artifacts:/export/editions.json backends/docs-builder-template/data/dkp/ && \
		docker cp d8-docs-artifacts:/export/embedded_modules.json backends/docs-builder-template/data/dkp/ && \
		docker cp d8-docs-artifacts:/export/embedded_modules_metadata.json backends/docs-builder-template/data/dkp/
		docker rm -f d8-docs-artifacts

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Variables:'
	@echo '  USE_LOCALHOST_REPO=1  Use --repo localhost:4999/docs instead of default --repo :local'
	@echo '                         Also starts local Docker registry in targets that need it'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {\
		FS = ":.*?## "; \
	} \
	/^[a-zA-Z0-9_-]+:.*?## / { printf "  %-20s %s\n", $$1, $$2 } \
	/^.?.?##~/              { printf "  %-20s %s\n", "", substr($$1, 6) }' $(MAKEFILE_LIST)

.PHONY: help up dev regenerate-menu
.PHONY: werf-bin clean down regenerate-metadata registry registry-down external-module
