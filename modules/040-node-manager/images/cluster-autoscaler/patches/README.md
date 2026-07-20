# Patches

## Go module updates (CVE remediation)

`001-go-mod.patch` bumps Go module dependencies to remediate CVEs reported by
Trivy for the cluster-autoscaler binary. The vulnerabilities live in
indirect/build dependencies that are linked into the binary (`x/crypto/ssh`,
`x/net`, and the `k8s.io/*` staging modules), not in cluster-autoscaler logic,
so the fix is a pure `go.mod`/`go.sum` bump — the gardener source tag is not
changed. The patch touches both `cluster-autoscaler/go.mod` and
`cluster-autoscaler/apis/go.mod`.

Typical bumps: `golang.org/x/net` -> `v0.55.0`, `golang.org/x/sys` -> `v0.45.0`,
`golang.org/x/crypto` -> `v0.51.0`, and `k8s.io/kubernetes` (plus all `k8s.io/*`
require/replace directives) to the latest fix patch of the matching minor
(for example `v1.32.10` / `v1.33.6` / `v1.34.2`).

Because the patch is generated against a specific gardener tag, it must be
recreated from a clean checkout of that tag; applying a patch made from a
different base fails in CI with `patch does not apply`. Per-version target
versions and the exact recreate commands are documented in each
`<k8s-minor>/README.md`. Note that the `1.34/` patch is also used for the
1.35 and 1.36 images (`werf.inc.yaml` clamps `$maxVersion = "1.34"`).

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
