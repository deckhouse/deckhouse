# Regenerates the wire contract from api/pb into api/gen.
#
# Generated code is committed, so a plugin implementing the protocol never runs this
# target; it only runs when a .proto changes.
#
# protoc and both plugins come from the repository's existing protoc tooling, which
# already pins their versions — a second pin here would be a second thing to drift.
# They are used by absolute path rather than from $PATH, so a protoc that happens to
# be installed on the machine cannot generate code with a different compiler version.
# protoc-gen-go and protoc-gen-go-grpc are found via $PATH by protoc itself, hence the
# PATH prefix.
#
# --proto_path=api/pb plus paths=source_relative puts api/pb/<action>/v1/x.proto in
# api/gen/<action>/v1/x.pb.go, so an action's generated code sits where its import
# path says it does.

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