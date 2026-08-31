# Regenerates the wire contract from api/pb into api/gen. Generated code is
# committed, so this only runs when a .proto changes.
#
# protoc versions are pinned by the repository's existing protoc tooling; a second
# pin here would be a second thing to drift.
#
# paths=source_relative mirrors the tree under api/pb into api/gen, so generated code
# sits where its import path says it does.

PROTO_DIR = api/pb
GEN_DIR = api/gen
PROTOS = $(shell find $(PROTO_DIR) -name '*.proto')

.PHONY: proto
proto:
	@mkdir -p $(GEN_DIR)
	protoc --proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(patsubst $(PROTO_DIR)/%,%,$(PROTOS))

.PHONY: test
test:
	go test ./...