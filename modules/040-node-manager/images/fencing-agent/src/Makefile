NODE_NAME:=virtlab-pt-1
tidy:
	go mod tidy
test:
	go test ./...
cover:
	go test -coverprofile=coverage.out ./...
cover-html: go-cover
	go tool cover -html=coverage.out -o coverage.html && open coverage.html
run:
	go run ./cmd/main.go
add_annotation:
	kubectl annotate node ${NODE_NAME} node-manager.deckhouse.io/fencing-disable=""
remove_annotation:
	kubectl annotate node ${NODE_NAME} node-manager.deckhouse.io/fencing-disable-
grpc_generate:
	protoc --go_out=pkg/api/ \
	--go_opt=paths=source_relative \
  --go-grpc_out=pkg/api/v1 \
  --go-grpc_opt=paths=source_relative \
  --proto_path=api/v1 \
  api/v1/fencing.proto
