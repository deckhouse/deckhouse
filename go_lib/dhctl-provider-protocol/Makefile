# Regenerates the wire contract from api/pb into api/<action>/v<N>. Generated code is
# committed, so this only runs when a .proto changes.
#
# Generated files are renamed to zz_generated.*.go, because the same package also
# holds the hand-written half of the contract (input.go, output.go and their tests).
# The prefix makes a file's origin obvious by its name and lets this target clean up
# after itself: everything matching zz_generated.* is regenerated, anything else is
# written by hand. Never delete the package directory wholesale — that takes the
# hand-written half with it.
#
# protoc versions are pinned by the repository's existing protoc tooling; a second
# pin here would be a second thing to drift.
#
# paths=source_relative mirrors the tree under api/pb into api/, so generated code
# sits where its import path says it does.

PROTO_DIR = api/pb
GEN_DIR = api/
PROTOS = $(shell find $(PROTO_DIR) -name '*.proto')

generate:
	@mkdir -p $(GEN_DIR)
	@find $(GEN_DIR) -name 'zz_generated.*.go' -delete
	protoc --proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(patsubst $(PROTO_DIR)/%,%,$(PROTOS))
	@# protoc names its output after the .proto it read; rename so generated code is
	@# recognisable without opening the file.
	@find $(GEN_DIR) -name '*.pb.go' -exec sh -c \
		'mv "$$1" "$$(dirname "$$1")/zz_generated.$$(basename "$$1" .pb.go).go"' _ {} \;

test:
	go test ./...
