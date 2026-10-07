## Patches

### 001-go-mod.patch

Bumps Go module dependencies to remediate CVEs reported by Trivy for the
cluster-autoscaler binary. CSE on release-1.77 still builds this image
(`ee/cse/candi/version_map.yml` sets `clusterAutoscalerPatch: '1'`), so the
gardener tag stays `v1.31.1`.

Applied to both `cluster-autoscaler/go.mod` and `cluster-autoscaler/apis/go.mod`:

- `google.golang.org/grpc`: `v1.65.0` -> `v1.83.2` (CVE-2026-84303, CVE-2026-84304, CVE-2026-84445)
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace`, `.../otlptrace/otlptracegrpc`
  and `go.opentelemetry.io/otel/sdk`: -> `v1.45.0` (CVE-2026-81870 / GO-2026-6505);
  `otel`, `otel/metric` and `otel/trace` follow to `v1.45.0`
- `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`:
  `replace` to `v0.53.0`. `google.golang.org/api` (pulled to `v0.264.0` by the
  bumps above) requires `v0.61.0`, which removed `UnaryClientInterceptor`, still
  called by `k8s.io/apiserver` and `k8s.io/cri-client` of this minor
- `github.com/google/cel-go`: left at `v0.20.1` (outside GO-2026-6094 affected range; apiserver 0.31 incompatible with cel-go >= v0.29)
- `golang.org/x/net`: `v0.38.0` -> `v0.58.0`
- `golang.org/x/text`: `v0.23.0` -> `v0.42.0`
- `golang.org/x/crypto`: `v0.36.0` -> `v0.57.0` (x/crypto/ssh CVE-2026-56854,
  CVE-2026-56855, CVE-2026-78662)
- `golang.org/x/sys`: `v0.31.0` -> `v0.48.0`
- `azidentity`: `v1.5.2` -> `v1.6.0`; `jwt/v4`: `v4.5.0` -> `v4.5.2`
- `go` directive: `1.23.0` -> `1.26.0`
- `github.com/opencontainers/runc`: `v1.1.13` -> `v1.2.9` (CVE-2024-45310,
  CVE-2025-31133, CVE-2025-52565, CVE-2025-52881). This is the only version
  where runc ends up in the binary: `k8s.io/mount-utils` imports
  `runc/libcontainer/userns`. From 1.32 on, `mount-utils` no longer pulls runc
  in, so the module disappears from the image (verify with
  `go version -m cluster-autoscaler`)
- `k8s.io/kubernetes`: `v1.31.1` -> `v1.31.6` (no 1.31.x fix for
  CVE-2025-13281; see `known_vulnerabilities.vex`)

To recreate this patch, check out the clean tag and re-apply the bumps:

```shell
git clone <SOURCE_REPO>/gardener/autoscaler.git
cd autoscaler && git checkout v1.31.1
cd cluster-autoscaler
go get google.golang.org/grpc@v1.83.2 \
  go.opentelemetry.io/otel/sdk@v1.45.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.45.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.45.0 \
  golang.org/x/net@v0.58.0 \
  golang.org/x/text@v0.42.0 \
  golang.org/x/crypto@v0.57.0 \
  golang.org/x/sys@v0.48.0 \
  github.com/Azure/azure-sdk-for-go/sdk/azidentity@v1.6.0 \
  github.com/golang-jwt/jwt/v4@v4.5.2 \
  github.com/opencontainers/runc@v1.2.9 \
  k8s.io/kubernetes@v1.31.6
go mod edit -replace=go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc=go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.53.0
cd apis
go get google.golang.org/grpc@v1.82.1 golang.org/x/net@v0.56.0 golang.org/x/text@v0.39.0
cd ..
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
