# Patches

## 001-go-mod.patch

Fixes several CVEs.

``` sh
go mod edit -go 1.23
go get golang.org/x/net@v0.33.0
go get google.golang.org/protobuf@v1.33.0
go mod tidy
git diff
```
