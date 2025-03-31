## Patches

### 001-go-mod.patch

To create this patch run commands:

```shell
go mod tidy -go=1.21
go get github.com/docker/distribution@v2.8.2-beta.1
go get github.com/emicklei/go-restful@v2.16.0
go get golang.org/x/crypto@v0.31.0
go get golang.org/x/net@v0.33.0
go get github.com/hashicorp/go-retryablehttp@v0.7.7
go get google.golang.org/grpc@v1.56.3
go get k8s.io/kubernetes@v1.24.17
#replase all in k8s.io v0.24.4 -> v0.24.17

go mod tidy
git diff > patches/001-go_mod.patch
#git apply patches/001-go_mod.patch
```
