# Patches

## Go module updates (CVE remediation)

`001-go-mod.patch` bumps Go module dependencies to remediate CVEs reported by
Trivy for the cluster-autoscaler binary. The vulnerabilities live in
dependencies linked into the binary (`google.golang.org/grpc`, `x/crypto/ssh`,
`x/net`, `x/text`, and the `k8s.io/*` staging modules), not in
cluster-autoscaler logic, so the fix is a pure `go.mod`/`go.sum` bump — the
gardener source tag is not changed. The patch covers `go.mod` and `go.sum` in
both the main `cluster-autoscaler` module and its nested `apis` module.

All active patches pin `google.golang.org/grpc v1.82.1`. `cel-go` is bumped to
`v0.30.0` on 1.33+ (GO-2026-6094); on 1.31/1.32 it stays at the gardener
upstream version because apiserver 0.31/0.32 is incompatible with cel-go >=
v0.29 (`TwoVarComprehensions` API). For 1.32 that finding is covered by
`known_vulnerabilities.vex`.

- 1.31 (CSE still builds this image from gardener `v1.31.1`): `x/net v0.56.0`,
  `x/text v0.39.0`, `x/crypto v0.53.0`, `x/sys v0.46.0` (the CVE floor was
  `crypto v0.52.0` / `sys v0.44.0`, but those exact pins conflict with
  `net@v0.56.0` and `text@v0.39.0`), `azidentity v1.6.0`, `jwt/v4 v4.5.2`,
  `runc v1.2.9`, `k8s.io/kubernetes v1.31.6` with staging require+replace
  synced to `v0.31.6`. There is no 1.31.x fix for CVE-2025-13281; that finding
  is documented in `known_vulnerabilities.vex`.
- 1.32–1.34: `x/net v0.56.0`, `x/text v0.39.0`, `x/crypto v0.53.0`,
  `x/sys v0.46.0`, plus Kubernetes staging bumps with require+replace sync
  (`v1.32.10` / `v1.33.6` / `v1.34.2`). 1.32 also pins `azidentity v1.6.0`,
  `jwt/v4 v4.5.2` and `runc v1.2.9`.
- 1.35 (also used for the 1.36 image): `k8s.io/kubernetes v1.35.8` (staging
  synced to `v0.35.8`), `x/mod v0.40.0` (CVE-2026-56864/56865), which pulls
  `x/net v0.58.0`, `x/text v0.41.0`, `x/crypto v0.55.0`, `x/sys v0.47.0`.

The runc CVEs (CVE-2024-45310, CVE-2025-31133, CVE-2025-52565,
CVE-2025-52881) only ever affected the **1.31** image: there `k8s.io/mount-utils`
imports `runc/libcontainer/userns`, so the module is linked into the binary and
Trivy reports it. From 1.32 on, `mount-utils` no longer pulls runc in and the
module is absent from the binary, so the `v1.2.9` pin in `1.32/` is a no-op for
the image — it is kept only for module-graph hygiene and parity with `1.31/`.
On 1.33+ `go mod tidy` drops the requirement entirely.

Note that `go mod why -m` answers "is this module needed by the module graph",
which is not the same as "is it linked into the binary": on 1.32 it reports a
chain through the kubemark cloud provider, but that provider is not part of the
build. Always confirm with `go version -m cluster-autoscaler` on the built
binary, since that is exactly what the image scanners read.

Because the patch is generated against a specific gardener tag, it must be
recreated from a clean checkout of that tag; applying a patch made from a
different base fails in CI with `patch does not apply`. Per-version target
versions and the exact recreate commands are documented in each
`<k8s-minor>/README.md`. Note that the `1.35/` patch is also used for the
1.36 image (`werf.inc.yaml` clamps `$maxVersion = "1.35"`).

## Scale from zero

We want to scale a node group from zero but our MCM revision does not support generic MachineClass CRs. 
With this patch we adds an ability to calculate node-group capacity from MachineDeployment annotations.
It makes sense only for calculation node-group capacity from zero, when we have no nodes presented.

## Kruise advanced daemonsets

Cluster autoscaler can't tell the difference between pods created by apps/v1 and apps.kruise.io/v1alpha1 
daemonsets when simulating if a node can be terminated. This patch makes cluster autoscaler check PDB 
instead of checking if an apps/v1 daemonset exists, when it bumps into a pod created by an advanced daemonset.

## Set priorities for to de deleted machines and clean annotation node.machine.sapcloud.io/trigger-deletion-by-mcm
Remove additional cordoning nodes from mcm cloud provider.

New autoscaler works with new version MCM witch select nodes for deleting from annotation `node.machine.sapcloud.io/trigger-deletion-by-mcm`
This annotation does not support by our MCM, and we should set deleting priority with annotation `machinepriority.machine.sapcloud.io`.
We set priority for machines and keep `node.machine.sapcloud.io/trigger-deletion-by-mcm` annotation for calculation replicas,
but we need to clean deleted machines from annotation in refresh function for keeping up to date annotation value to avoid
drizzling replicas count in machine deployment.

### Report-all-machine-creation-errors-to-ca.patch

Report all machine creation errors to Cluster Autoscaler, not only ResourceExhausted

Previously, generateInstanceStatus only reported ErrorInfo to the Cluster Autoscaler when a Machine failed with ResourceExhausted error code (quota/stockout).
All other creation failures (invalid image, wrong credentials, network errors, etc.) returned InstanceStatus without ErrorInfo, making them invisible to CA's error handling.

### Fix-upcoming-nodes-deadlock-for-failed-node-groups.patch

Exclude upcoming nodes for groups without active scale-up requests or are backed off in cluster state handling.

GetUpcomingNodes() counts upcoming nodes based solely on target - registered, without checking whether the scale-up is still actively in progress.
When instance creation fails, handleInstanceCreationErrors removes the scaleUpRequest (by decrementing Increase to zero),
but GetUpcomingNodes() continues to report upcoming nodes because the target size was never decreased.

This patch adds two guards in GetUpcomingNodes() to skip counting upcoming nodes when:

- There is no active scaleUpRequest for the node group (indicating the previous scale-up attempt has failed or timed out)
- The node group is in backoff state

This breaks the deadlock: pods remain unschedulable, ScaleUp() is invoked, and the priority expander can fall back to a working lower-priority node group.
