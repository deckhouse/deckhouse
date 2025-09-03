## Patches

### 001-go-mod.patch

To create this patch run commands:

```shell
go get golang.org/x/crypto@v0.35.0
go get golang.org/x/net@v0.38.0
go get golang.org/x/oauth2@v0.27.0
go get github.com/prometheus/client_golang@v1.11.1
go get github.com/prometheus/exporter-toolkit@v0.7.2
go get google.golang.org/protobuf@v1.33.0
go get gopkg.in/yaml.v3@v3.0.0-20220521103104-8f96da9f5d5e
go mod tidy
git diff
```
