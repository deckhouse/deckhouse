SHELL := /bin/bash
base_golang_19_bullseye := golang:1.19-bullseye
repo_local := localhost:5000
repo_fox := registry.flant.com/deckhouse/storage
image_name := sds-drbd-operator

USER := $(shell whoami)
image_tag := $(USER)
run: ## run go
	go run -race ./cmd/main.go

local_build_and_push:
	docker build --build-arg="BASE_GOLANG_19_BULLSEYE=$(base_golang_19_bullseye)" . -t $(repo_local)/$(image_name):$(image_tag)
	docker push $(repo_local)/$(image_name):$(image_tag)

offline_build_and_push:
	docker build --build-arg="BASE_GOLANG_19_BULLSEYE=localhost:5000/d8-sds-drbd-operator-builder" . -t $(repo_local)/$(image_name):$(image_tag)
	docker push $(repo_local)/$(image_name):$(image_tag)

fox_build_and_push:
	docker build --build-arg="BASE_GOLANG_19_BULLSEYE=$(base_golang_19_bullseye)" . -t $(repo_fox)/$(image_name):$(image_tag)
	docker push $(repo_fox)/$(image_name):$(image_tag)

deploy_fox:
	sed 's/latest/$(USER)/g' ./dev/2-deployment-fox.yaml | kubectl -n d8-linstor apply -f -

deploy_local:
	kubectl -n d8-linstor apply -f ./dev/1-deployment-local.yaml

redeploy:
	kubectl -n d8-linstor rollout restart deployment sds-drbd-operator

