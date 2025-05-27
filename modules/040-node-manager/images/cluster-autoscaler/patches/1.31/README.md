## Patches

### 001-go-mod.patch

To create this patch run commands:

```shell
cd cluster-autoscaler
go mod edit -go 1.23
go get github.com/golang-jwt/jwt/v4@v4.5.1
go get github.com/opencontainers/runc@v1.1.14
go get golang.org/x/crypto@v0.31.0
go get golang.org/x/net@v0.33.0

go get k8s.io/kubernetes@v1.30.8
go get k8s.io/kubelet@v0.30.8
#replase all in k8s.io  v0.30.1 -> v0.30.8
cd apis
go get golang.org/x/net@v0.33.0
cd ..
go mod tidy
git diff > patches/001-go_mod.patch
#git apply patches/001-go_mod.patch
```

### 002-kruise-ads.patch

TODO: add description

### 003-scale-from-zero.patch

TODO: add description

### 004-set-priorities-for-to-de-deleted-machines-and-clean-annotation.patch

Remove additional cordoning nodes from mcm cloud provider.

New autoscaler works with new version MCM witch select nodes for deleting from annotation `node.machine.sapcloud.io/trigger-deletion-by-mcm`
This annotation does not support by our MCM, and we should set deleting priority with annotation `machinepriority.machine.sapcloud.io`.
We set priority for machines and keep `node.machine.sapcloud.io/trigger-deletion-by-mcm` annotation for calculation replicas,
but we need to clean deleted machines from annotation in refresh function for keeping up to date annotation value to avoid
drizzling replicas count in machine deployment.
