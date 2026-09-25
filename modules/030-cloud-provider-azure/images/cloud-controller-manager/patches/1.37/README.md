### 001-go-mod.patch

Bump go.mod dependencies to fix known CVEs:
- `github.com/google/cel-go` v0.29.2 -> v0.30.0 (GO-2026-6094)
- `go.etcd.io/etcd/client/pkg/v3` v3.7.0 -> v3.7.1 (GO-2026-6107)

### 002-options.patch

This patch add NodeController options to main context object CloudControllerManager from package "k8s.io/cloud-provider/options" witch return flag "node controller".

### 003-fix-disable-api-call-cache.patch

Fix some issues with the disable API call cache feature.
