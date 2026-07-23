## Patches

### 001-go-mod.patch

Bumps Go module dependencies to remediate CVEs reported by Trivy for the
cluster-autoscaler binary. The vulnerabilities live in indirect/build
dependencies that are linked into the binary (x/crypto/ssh, x/net, k8s
staging modules), not in cluster-autoscaler logic, so the fix is a pure
`go.mod`/`go.sum` bump. The gardener tag stays `v1.32.3`.

Applied to both `cluster-autoscaler/go.mod` and `cluster-autoscaler/apis/go.mod`:

- `go` directive: `1.23.0` (+ `toolchain go1.23.2`) -> `1.25.0`
- `golang.org/x/net`: `v0.38.0` -> `v0.55.0` (HTML parser / HTTP2 / idna CVEs)
- `golang.org/x/sys`: `v0.31.0` -> `v0.45.0`
- `golang.org/x/crypto`: `v0.36.0` -> `v0.51.0` (x/crypto/ssh CVEs)
- `k8s.io/kubernetes`: `v1.32.0` -> `v1.32.10`, and all `k8s.io/*` staging
  modules (require + replace) synced to `v0.32.10` (kube-controller-manager
  SSRF, CVE-2025-13281)

To recreate this patch, check out the clean tag and re-apply the bumps:

```shell
git clone <SOURCE_REPO>/gardener/autoscaler.git
cd autoscaler && git checkout v1.32.3
cd cluster-autoscaler
go get golang.org/x/crypto@v0.51.0
go get golang.org/x/net@v0.55.0
go get golang.org/x/sys@v0.45.0
go get k8s.io/kubernetes@v1.32.10
# sync every k8s.io/* require and replace directive to v0.32.10
cd apis && go get golang.org/x/net@v0.55.0 && cd ..
go mod tidy && (cd apis && go mod tidy)
cd ..
git diff -- cluster-autoscaler/go.mod cluster-autoscaler/go.sum \
            cluster-autoscaler/apis/go.mod cluster-autoscaler/apis/go.sum \
  > 001-go-mod.patch
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

### 005-report-all-machine-creation-errors-to-ca.patch

Report all machine creation errors to Cluster Autoscaler, not only ResourceExhausted

Previously, generateInstanceStatus only reported ErrorInfo to the Cluster Autoscaler when a Machine failed with ResourceExhausted error code (quota/stockout).
All other creation failures (invalid image, wrong credentials, network errors, etc.) returned InstanceStatus without ErrorInfo, making them invisible to CA's error handling.

### 006-fix-upcoming-nodes-deadlock-for-failed-node-groups.patch

Exclude upcoming nodes for groups without active scale-up requests or are backed off in cluster state handling.
 
GetUpcomingNodes() counts upcoming nodes based solely on target - registered, without checking whether the scale-up is still actively in progress.
When instance creation fails, handleInstanceCreationErrors removes the scaleUpRequest (by decrementing Increase to zero), 
but GetUpcomingNodes() continues to report upcoming nodes because the target size was never decreased.

This patch adds two guards in GetUpcomingNodes() to skip counting upcoming nodes when:

- There is no active scaleUpRequest for the node group (indicating the previous scale-up attempt has failed or timed out)
- The node group is in backoff state

This breaks the deadlock: pods remain unschedulable, ScaleUp() is invoked, and the priority expander can fall back to a working lower-priority node group.
