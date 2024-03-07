## Patches

### Go mod

To create this patch run commands:

```shell
go mod edit -go 1.20
go get golang.org/x/net@v0.17.0
go get gopkg.in/yaml.v3@v3.0.1
go get github.com/prometheus/client_golang@v1.17.0
go get golang.org/x/crypto@v0.14.0
go mod tidy
git diff
```
