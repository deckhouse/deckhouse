## Patches

These patches are applied to gardener `v1.36.0`. The same build is used for
the k8s 1.36 and 1.37 images (see `oss.yaml`).

Since `v1.36.0` gardener registers cloud providers through
`cloudprovider/router`, and the mcm provider is compiled in only with the `mcm`
build tag (`router_mcm.go`). `werf.inc.yaml` therefore builds 1.36+ with
`-tags mcm`; `router_all.go` is still built with that tag, so `clusterapi`
stays in the binary too. Without the tag the binary has no `mcm` provider and
`--cloud-provider=mcm` fails at startup.

### 001-go-mod.patch

Bumps Go module dependencies to remediate CVEs reported by Trivy for the
cluster-autoscaler binary. The vulnerabilities live in dependencies linked
into the binary, not in cluster-autoscaler logic, so the fix is a pure
`go.mod`/`go.sum` bump. The gardener tag stays `v1.36.0`.

Changes in `cluster-autoscaler/go.mod`:

- `google.golang.org/grpc`: `v1.79.3` -> `v1.83.2` (CVE-2026-84303, CVE-2026-84304,
  CVE-2026-84445, GHSA-hrxh-6v49-42gf)
- `go.opentelemetry.io/otel/sdk`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace`
  and `.../otlptrace/otlptracegrpc`: `v1.40.0` -> `v1.45.0` (CVE-2026-81870 / GO-2026-6505,
  CVE-2026-39883); `otel`, `otel/metric` and `otel/trace` follow to `v1.45.0`
- `github.com/google/cel-go`: `v0.26.1` -> `v0.30.0` (GO-2026-6094 / GHSA-gcjh-h69q-9w9g)
- `golang.org/x/mod`: `v0.35.0` -> `v0.41.0` (CVE-2026-56864, CVE-2026-56865)
- `golang.org/x/crypto`: `v0.52.0` -> `v0.57.0` (CVE-2026-56854, CVE-2026-56855, CVE-2026-78662)
- `golang.org/x/net`: `v0.55.0` -> `v0.58.0` (CVE-2026-46600)
- `golang.org/x/text`: `v0.37.0` -> `v0.42.0` (CVE-2026-56852)
- `golang.org/x/sys`: `v0.45.0` -> `v0.48.0`

`k8s.io/kubernetes` stays at `v1.36.2`: Trivy reports nothing against it.
In `cluster-autoscaler/apis/go.mod` `x/mod`, `x/net`, `x/text` and `x/sys`
are synced to the same versions.

After the bump Trivy reports only GO-2026-5932, a deprecation notice for
`golang.org/x/crypto/openpgp` with no fixed version. That package is not
linked into the binary (`go list -deps -tags mcm .` does not list it).

To recreate this patch, check out the clean tag and re-apply the bumps:

```shell
git clone <SOURCE_REPO>/gardener/autoscaler.git
cd autoscaler && git checkout v1.36.0
cd cluster-autoscaler
go get google.golang.org/grpc@v1.83.2 \
  go.opentelemetry.io/otel/sdk@v1.45.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.45.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.45.0 \
  github.com/google/cel-go@v0.30.0 \
  golang.org/x/sys@v0.48.0 \
  golang.org/x/net@v0.58.0 \
  golang.org/x/text@v0.42.0 \
  golang.org/x/crypto@v0.57.0 \
  golang.org/x/mod@v0.41.0
cd apis
go get golang.org/x/net@v0.58.0 \
  golang.org/x/text@v0.42.0 \
  golang.org/x/mod@v0.41.0 \
  golang.org/x/sys@v0.48.0
go mod tidy
cd ..
go mod tidy
cd ..
git diff -- cluster-autoscaler/go.mod cluster-autoscaler/go.sum \
            cluster-autoscaler/apis/go.mod cluster-autoscaler/apis/go.sum \
  > 001-go-mod.patch
```

### 002-kruise-ads.patch

Makes cluster-autoscaler check PDBs for pods owned by `apps.kruise.io`
advanced DaemonSets instead of looking up an `apps/v1` DaemonSet. See the
top-level `README.md`.

### 003-scale-from-zero.patch

Calculates node group capacity from MachineDeployment annotations so a node
group can scale from zero. See the top-level `README.md`.

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

### No 006-fix-upcoming-nodes-deadlock-for-failed-node-groups.patch

Not needed since `v1.36.0`: upstream `GetUpcomingNodes()` already skips node
groups without an active scale-up request and backed-off node groups (it came
in with gardener's "Sync upstream v1.36.0").

### 007-scale-up-timeout-for-long-unregistered-nodes.patch

Makes a scale-up whose machines never become nodes time out, so the node group
is backed off and the priority expander can fall back to another group.

In `v1.36.0`, `ClusterStateRegistry.updateScaleRequests` decides that a
scale-up is complete with `calculateUpcomingNodesInNodeGroup`, which is
`CurrentTarget - (Ready + Unready + Suspended + LongUnregistered)`
(upstream kubernetes/autoscaler#9860). Instances that are still unregistered
after `MaxNodeProvisionTime` move to `LongUnregistered` and stop counting as
upcoming, and this check runs before the scale-up timeout check. Both
deadlines expire about 15 minutes after the scale-up, a few seconds apart, so
the request is usually closed as a success: no `ScaleUpTimedOut` event, no backoff, and every 15
minutes CA scales up the same broken group again.

The patch keeps long-unregistered instances pending in `updateScaleRequests`
only:

```go
upcoming, ok := csr.getUpcomingNodesInNodeGroup(nodeGroupName)
// Long-unregistered nodes aren't upcoming, but they haven't fulfilled the request.
pending := upcoming + len(csr.perNodeGroupReadiness[nodeGroupName].LongUnregistered)
if !ok || pending <= 0 {
```

The request now reaches the timeout branch, which registers a failed scale-up
with the `timeout` error code and backs the node group off.
`calculateUpcomingNodesInNodeGroup` and `GetUpcomingNodes()` are unchanged, so
long-unregistered instances are still not injected into the cluster snapshot
as upcoming nodes. The patch also adds `TestLongUnregisteredScaleUpBacksOff`
(fails without the fix) and `TestScaleUpCompletionWithDeletedNodes` to
`clusterstate_test.go`.

This is a backport of the upstream fix:

- issue: https://github.com/kubernetes-sigs/cluster-autoscaler/issues/150
- PR: https://github.com/kubernetes-sigs/cluster-autoscaler/pull/157

The upstream code takes a `context.Context` and uses structured logging; the
patch adapts it to the `v1.36.0` API. 1.35 is not affected: there a scale-up
is complete only when registered nodes reach the target size, so
long-unregistered instances still hit the timeout. Drop this patch once the fix
is in a gardener autoscaler release that `oss.yaml` points to.
