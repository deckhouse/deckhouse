tidy:
	go mod tidy
test:
	go test ./...
cover:
	go test -coverprofile=coverage.out ./...
cover-html: cover
	go tool cover -html=coverage.out -o coverage.html && open coverage.html
run:
	go run .
.PHONY: tidy test cover cover-html run
