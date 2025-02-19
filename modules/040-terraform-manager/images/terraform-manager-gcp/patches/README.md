## Patches

### remove_routes_on_deletion
https://github.com/flant/terraform-provider-google/compare/v3.48.0...v3.48.0-flant.1

### Go mod

To create this patch run commands:

```shell
go mod edit -go 1.23
go get golang.org/x/net@v0.33.0
go get github.com/aws/aws-sdk-go@v1.34.0
go get github.com/hashicorp/go-getter@v1.6.2
go mod tidy
git diff > patches/go_mod.patch
#git apply patches/go_mod.patch
```
