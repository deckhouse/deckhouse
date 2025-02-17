## Patches

### Go mod

To create this patch run commands:

```shell
go mod edit -go 1.23
go get google.golang.org/protobuf@v1.33.0
go get golang.org/x/net@v0.33.0
go mod tidy
cd test/client
go mod edit -go 1.23
go get google.golang.org/protobuf@v1.33.0
go get golang.org/x/net@v0.33.0
go mod tidy
cd test/server
go mod edit -go 1.23
go get google.golang.org/protobuf@v1.33.0
go get golang.org/x/net@v0.33.0
go mod tidy
git diff > patches/go_mod.patch
#git apply patches/go_mod.patch
```

### Makefile

Use `go mod download`
