# Generated files are renamed to zz_generated.*.go, because the same package also holds
# the hand-written half of the contract (input.go and its tests). Regeneration deletes
# only that prefix — never the package directory, which would take the other half too.

PROTO_DIR = api/pb
GEN_DIR = api/
PROTOS = $(shell find $(PROTO_DIR) -name '*.proto')

LOCALBIN = $(shell pwd)/bin
PROTOC = $(LOCALBIN)/protoc
PROTOC_VERSION = 29.3
PROTOC_GEN_GO = $(LOCALBIN)/protoc-gen-go
PROTOC_GEN_GO_VERSION = v1.36.12
PROTOC_GEN_GO_GRPC = $(LOCALBIN)/protoc-gen-go-grpc
PROTOC_GEN_GO_GRPC_VERSION = v1.6.2

ifeq ($(shell uname), Darwin)
	PROTOC_PLATFORM = osx
else
	PROTOC_PLATFORM = linux
endif

ifeq ($(shell uname -m), arm64)
	PROTOC_ARCH = universal_binary
else
	PROTOC_ARCH = x86_64
endif

.PHONY: generate
generate: $(PROTOC) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
	$(PROTOC) --proto_path=$(PROTO_DIR) \
		--plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(patsubst $(PROTO_DIR)/%,%,$(PROTOS))
	@find $(GEN_DIR) -name 'zz_generated.*.go' -delete
	@find $(GEN_DIR) -name '*.pb.go' -exec sh -c \
		'mv "$$1" "$$(dirname "$$1")/zz_generated.$$(basename "$$1" .pb.go).go"' _ {} \;

.PHONY: test
test:
	go test ./...

$(PROTOC):
	@mkdir -p $(LOCALBIN)
	tmp=$$(mktemp -d); trap 'rm -rf $$tmp' EXIT; \
		curl -sLo $$tmp/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(PROTOC_PLATFORM)-$(PROTOC_ARCH).zip; \
		unzip -o -j $$tmp/protoc.zip bin/protoc -d $(LOCALBIN) 1>/dev/null

$(PROTOC_GEN_GO):
	GOBIN=$(LOCALBIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)

$(PROTOC_GEN_GO_GRPC):
	GOBIN=$(LOCALBIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
