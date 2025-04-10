## Patches

### 001-go-mod.patch

To create this patch run commands:

```shell
cd cluster-autoscaler
go mod edit -go 1.23
go get github.com/golang-jwt/jwt/v4@v4.5.2
go get github.com/opencontainers/runc@v1.1.14
go get golang.org/x/crypto@v0.31.0
go get golang.org/x/net@v0.36.0
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity@v1.6.0
go get k8s.io/kubernetes@v1.29.14
go get k8s.io/kubelet@v0.29.14
#replase all in k8s.io  v0.29.* -> v0.29.14
go mod tidy
git diff > patches/001-go_mod.patch
#git apply patches/001-go_mod.patch
```

### 002-kruise-ads.patch

TODO: add description

### 003-scale-from-zero.patch

TODO: add description
