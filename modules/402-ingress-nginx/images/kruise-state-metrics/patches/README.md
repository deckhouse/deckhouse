## Patches

### 001-go-mod.patch

To create this patch run commands:

```shell
go mod edit -go 1.20
go get gopkg.in/yaml.v3@v3.0.1
go get github.com/prometheus/client_golang@v1.17.0
go get golang.org/x/crypto@v0.14.0
go get github.com/prometheus/exporter-toolkit@v0.7.2
go get google.golang.org/protobuf@v1.33.0
go get golang.org/x/net@v0.23.0
go mod tidy
git diff
```
