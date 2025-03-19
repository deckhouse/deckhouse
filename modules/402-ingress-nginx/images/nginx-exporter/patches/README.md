## Patches

### 001-go-mod.patch

To create this patch run commands:

```shell
go mod edit -go 1.23
go get github.com/prometheus/client_golang@v1.17.0
go get google.golang.org/protobuf@v1.33.0
go get golang.org/x/sys@v0.25.0
go mod tidy
go mod vendor
git diff
```
