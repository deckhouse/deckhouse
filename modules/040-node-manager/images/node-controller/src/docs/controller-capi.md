# capi

**Package:** `internal/controller/capi`
**Replaces helm templates:** `_capi_machine_deployment.tpl`, `_static_or_hybrid_machine_deployment.tpl`, `static-cluster.yaml` (v1beta2 parts)
**Replaces hook:** `capi_set_replicas` (MCM replica scaling)

The package registers six **independent** controllers. They do not call each other —
each has its own primary resource, watches and reconcile loop. All share the
`BaseWithReader` base (cached `Client` + uncached `APIReader`). Documented one by one below.

| Registered name | Primary resource | File |
|-----------------|------------------|------|
| `capi-machine-deployment` | `NodeGroup` | `machinedeployment.go` |
| `capi-cluster-resources` | `Secret` (cloud-provider) | `cluster.go` |
| `capi-api-version` | `MachineDeployment` (CAPI) | `apiversion.go` |
| `capi-control-plane` | `DeckhouseControlPlane` | `controlplane.go` |
| `capi-finalizer-cleanup` | `Cluster` (CAPI) | `finalizer.go` |
| `capi-md-metrics` | `MachineDeployment` (CAPI) | `metrics.go` |

---

# capi-machine-deployment

| | |
|---|---|
| **Primary** | `NodeGroup` |
| **Watches** | MCM `MachineDeployment` (v1alpha1) + CAPI `MachineDeployment` (v1beta2), both mapped to a NodeGroup by the `node-group` label |
| **Reads** | cloud-provider Secret `d8-node-manager-cloud-provider`, `d8-cluster-configuration`, ConfigMap `d8-cluster-uuid` (see [Data source keys](#data-source-keys)) |
| **Output** | one or more `MachineDeployment` per NodeGroup (CAPI or MCM) |

Reconciles the desired set of MachineDeployments (or MCM replica counts) for one NodeGroup.

```
NodeGroup changed (or MD re-enqueues by node-group label)
  │
  ├─ NodeGroup not found? → done
  │
  ├─ nodeType == CloudEphemeral:
  │   ├─ status.engine == CAPI  → reconcileCloudMDs
  │   ├─ status.engine == MCM   → reconcileMCMReplicas
  │   └─ else                   → skip (engine not set)
  │
  └─ nodeType == Static/CloudStatic (with staticInstances):
      └─ reconcileStaticMD
```

### reconcileCloudMDs (CAPI engine)

Creates/updates one `MachineDeployment` (cluster.x-k8s.io/v1beta2) **per zone**:

- **MD name:** `{instancePrefix}-{ng.name}-{hash}` (prefix omitted when empty),
  where `hash = sha256(clusterUUID + zone)[:8]` — **stable, excludes the instance-class checksum**.
- **Template / bootstrap name:** `{ng.name}-{hash2}`,
  where `hash2 = sha256(clusterUUID + zone + instanceClassChecksum)[:8]` — **includes the checksum**, so a class change rolls a new template.
- `spec.clusterName` = `capiClusterName`; `spec.template.spec.infrastructureRef` kind/apiGroup from the cloud-provider secret.
- `spec.template.spec.bootstrap.dataSecretName` = template name.
- `spec.template.spec.deletion`: `nodeDrainTimeoutSeconds` from `ng.spec.nodeDrainTimeoutSecond` (default 600); deletion/volume-detach timeouts 600.
- `spec.rollout.strategy`: `maxSurge`/`maxUnavailable` from `cloudInstances` (defaults 1/0).
- Annotations: autoscaler min/max size, plus capacity labels/taints (`serializeNodeGroupLabels` / `serializeNodeGroupTaints`).
- `spec.replicas` = `calculateReplicas(current, minPerZone, maxPerZone)` — clamps the
  current replica count into `[min, max]`. The desired count is changed by editing NodeGroup `min/max`, not by patching the MD.
- Applied with server-side apply (`FieldOwner("node-controller")`, `ForceOwnership`).

### reconcileStaticMD (static engine)

Creates one `MachineDeployment` named `{ng.name}`:

- `spec.clusterName: static`; `infrastructureRef` → `StaticMachineTemplate`.
- `bootstrap.dataSecretName: manual-bootstrap-for-{ng.name}`.
- `spec.replicas` = `ng.spec.staticInstances.count`.
- `spec.rollout.strategy`: maxSurge 1, maxUnavailable 0.
- selector/template labels: `cluster-name=static`, `deployment-name={ng.name}`.

### reconcileMCMReplicas (MCM engine)

Lists MCM `MachineDeployment`s by `node-group` label and patches `spec.replicas`
to `calculateReplicas(current, min, max)` only when it differs
(`FieldOwner("capi-set-replicas")`). This replaces the legacy `capi_set_replicas` hook.

### Helpers

| Func | Behavior |
|------|----------|
| `getMinMax(ng)` | static → `(count, count)`; cloud → `(minPerZone, maxPerZone)`; else `(0, 0)` |
| `calculateReplicas(cur, min, max)` | `min>=max`→max; `cur==0`→min; `cur<=min`→min; `cur>max`→max; else cur |
| `sha256Hash(s)` | `sha256(s)` hex, truncated to 8 chars |
| `serializeNodeGroupLabels(ng)` | merges NodeTemplate labels + `node.deckhouse.io/group`, `node.deckhouse.io/type`, `node-role.kubernetes.io/{name}` |
| `serializeNodeGroupTaints(ng)` | sorted `key=value:effect` list |

---

# capi-cluster-resources

| | |
|---|---|
| **Primary** | `Secret` `d8-node-manager-cloud-provider` (kube-system) |
| **Watches** | that Secret (event filter) + all NodeGroups (any NodeGroup change re-enqueues the Secret request) |
| **Reads** | the cloud-provider Secret, `d8-cluster-configuration` (cluster network) |
| **Output** | top-level CAPI `Cluster` + `MachineHealthCheck` |

Ensures the `Cluster` and `MachineHealthCheck` objects exist. Uses `Create`-if-not-exists
(never overwrites a running cluster).

- **ensureCloudCluster:** when `capiClusterName`/`capiClusterKind` are set, creates a
  `Cluster` (infrastructureRef from the secret, controlPlaneRef → `{name}-control-plane`
  `DeckhouseControlPlane`) and a `MachineHealthCheck` (`{name}-machine-health-check`,
  nodeStartup 1200s, Ready=Unknown/False timeout 300s).
- **ensureStaticCluster:** when at least one NodeGroup has `staticInstances`, creates a
  `Cluster` named `static` (infrastructureRef → `StaticCluster`, controlPlaneRef →
  `static-control-plane`) and `static-machine-health-check`
  (Ready=Unknown timeout 2147483647s — effectively never).

Cluster network (`pods`/`services`/`serviceDomain`) comes from `d8-cluster-configuration`.

---

# capi-api-version

| | |
|---|---|
| **Primary** | CAPI `MachineDeployment` (v1beta2) |
| **Watches** | CAPI `Machine` (mapped to MDs by `node-group` label) + CAPI `Cluster` (enqueued under synthetic request name `cluster:{name}`) |
| **Reads** | the object being reconciled |
| **Output** | patches `infrastructureRef.apiGroup` on MD/Machine/Cluster |

Backfills `infrastructureRef.apiGroup` on objects that only carry `kind` (a v1beta1→v1beta2
contract requirement). Dispatches by request name:

- **`cluster:<name>`** → `reconcileCluster`: sets `spec.infrastructureRef.apiGroup` and
  `spec.controlPlaneRef.apiGroup` to `infrastructure.cluster.x-k8s.io` when missing.
- **otherwise** → patches the MachineDeployment's `infrastructureRef.apiGroup`, then lists
  the MD's Machines and patches each `infrastructureRef.apiGroup`.

Known kinds are resolved through static maps (`machineTemplateAPIGroups`, `machineAPIGroups`,
`clusterInfraAPIGroups`, `controlPlaneAPIGroups`) covering all six infra providers
(Deckhouse, Dynamix, HuaweiCloud, Static, VCD, Zvirt). Unknown kinds are logged and skipped.

---

# capi-control-plane

| | |
|---|---|
| **Primary** | `DeckhouseControlPlane` (infrastructure.cluster.x-k8s.io/v1alpha1) |
| **Watches** | — (primary only) |
| **Output** | status patch on the DeckhouseControlPlane |

Marks the externally-managed control plane as ready so CAPI proceeds. On every reconcile it
status-patches the object with `initialized: true`, `ready: true`,
`externalManagedControlPlane: true`, and `initialization.controlPlaneInitialized: true`.

---

# capi-finalizer-cleanup

| | |
|---|---|
| **Primary** | CAPI `Cluster` (v1beta2) |
| **Watches** | — (primary only) |
| **Output** | removes a finalizer from the Cluster |

Removes the `deckhouse.io/capi-controller-manager` finalizer from Clusters so deletion is not
blocked when capi-controller-manager is unavailable. No-op when the finalizer is absent.

---

# capi-md-metrics

| | |
|---|---|
| **Primary** | CAPI `MachineDeployment` (v1beta2) |
| **Watches** | — (primary only) |
| **Output** | Prometheus gauges (label `machine_deployment_name`) |

Exports per-MachineDeployment gauges into the controller-runtime metrics registry.
On NotFound it clears the series for that MD.

| Metric | Source |
|--------|--------|
| `d8_caps_md_desired` | `spec.replicas` |
| `d8_caps_md_replicas` | `status.replicas` |
| `d8_caps_md_ready` | `status.readyReplicas` |
| `d8_caps_md_unavailable` | `status.replicas - status.availableReplicas` (when positive) |
| `d8_caps_md_phase` | `phaseToFloat(status.phase)`: Running=1, ScalingUp=2, ScalingDown=3, Failed=4, else 5 |

---

## Data source keys

Referenced by `capi-machine-deployment` and `capi-cluster-resources`:

| Value | Resource | Key |
|-------|----------|-----|
| `clusterUUID` | ConfigMap `d8-cluster-uuid` (kube-system) | `cluster-uuid` |
| `instancePrefix` | Secret `d8-cluster-configuration` (kube-system) | `cluster-configuration.yaml` → `cloud.prefix` |
| `capiClusterName`, `capiClusterKind`, `capiClusterAPIVersion` | Secret `d8-node-manager-cloud-provider` (kube-system) | same keys |
| `capiMachineTemplateKind`, `capiMachineTemplateAPIVersion` | same Secret | same keys |
| `zones` | Secret `d8-node-manager-cloud-provider` or NodeGroup | `zones` / `spec.cloudInstances.zones` |
| `podSubnetCIDR`, `serviceSubnetCIDR`, `clusterDomain` | Secret `d8-cluster-configuration` | `cluster-configuration.yaml` |
| `instanceClassChecksum` | infrastructure MachineTemplate annotation | `checksum/instance-class` |

## Files

- `machinedeployment.go` — MachineDeployment reconciler (cloud/static/MCM), data readers, hash/serialize helpers
- `cluster.go` — Cluster + MachineHealthCheck reconciler (cloud + static)
- `apiversion.go` — `infrastructureRef.apiGroup` backfill for MD/Machine/Cluster
- `controlplane.go` — DeckhouseControlPlane status patcher
- `finalizer.go` — capi-controller-manager finalizer cleanup
- `metrics.go` — Prometheus gauges for MachineDeployments
- `common.go` — `BaseWithReader`, shared constants, `newUnstructured`
