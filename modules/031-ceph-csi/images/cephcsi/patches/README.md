## Patches

### Go mod

To create this patch run commands:

```shell
go mod tidy -go=1.21
go get github.com/docker/distribution@v2.8.2-beta.1

go mod tidy
git diff > patches/go_mod.patch
#git apply patches/go_mod.patch
```
