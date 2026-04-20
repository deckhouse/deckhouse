CONTROLLER_GEN := $(shell if [ -n "$$(go env GOBIN)" ]; then printf "%s/controller-gen" "$$(go env GOBIN)"; else printf "%s/bin/controller-gen" "$$(go env GOPATH)"; fi)

.PHONY: generate-api
generate-api:
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	$(CONTROLLER_GEN) object paths="./src/api/..."
