## Patches

### 001-go-mod.patch

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
git diff > patches/001-go-mod.patch
#git apply patches/001-go-mod.patch
```

### 002-Makefile.patch

Use `go mod download`
