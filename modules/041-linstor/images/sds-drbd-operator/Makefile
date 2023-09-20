SHELL := /bin/bash
base_golang_19_bullseye := golang:1.19-bullseye
local_repo := "localhost:5000"
image_name := "d8-sds-drbd-operator"
image_tag := "latest"
run: ## run go
	go run -race ./cmd/main.go

local_build_and_push:
	docker build --build-arg="BASE_GOLANG_19_BULLSEYE=$(base_golang_19_bullseye)" . -t $(local_repo)/$(image_name):$(image_tag)
	docker push $(local_repo)/$(image_name):$(image_tag)

offline_build_and_push:
	docker build --build-arg="BASE_GOLANG_19_BULLSEYE=localhost:5000/d8-sds-drbd-operator-builder" . -t $(local_repo)/$(image_name):$(image_tag)
	docker push $(local_repo)/$(image_name):$(image_tag)

deploy_local:
	kubectl -n d8-linstor apply -f ./dev/1-deployment-local.yaml

redeploy:
	kubectl -n d8-linstor rollout restart deployment d8-sds-drbd-operator

