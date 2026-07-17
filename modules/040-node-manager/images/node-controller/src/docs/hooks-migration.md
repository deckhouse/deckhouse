# Migration of Conversion and Validation Hooks from Python/Bash to Go

## Overview

This document describes how hooks from `modules/040-node-manager/hooks/` were rewritten in Go:

1. **node_group.py** — Conversion Webhook (Python)
2. **node_group** — Validation Webhook (Bash)

## Part 1: Conversion Hooks (node_group.py)

## Comparison: Python Hook vs Go Conversion

### Python Hook (shell-operator)

```python
# node_group.py
kubernetesCustomResourceConversion:
  - name: alpha1_to_alpha2
    crdName: nodegroups.deckhouse.io
    conversions:
    - fromVersion: deckhouse.io/v1alpha1
      toVersion: deckhouse.io/v1alpha2
```

**How it works:**
1. Shell-operator registers a Conversion Webhook with the API Server
2. API Server calls the webhook over HTTP during conversion
3. Python code processes the object and returns the result

**Problems:**
- HTTP call on every conversion
- Separate process for handling
- JSON/YAML parsing
- No type safety

### Go Conversion (controller-runtime)

```go
// api/deckhouse.io/v1alpha1/nodegroup_conversion.go
func (src *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*v1.NodeGroup)
    // ... conversion
    return nil
}
```

**How it works:**
1. Webhook server is embedded in the controller
2. API Server calls the webhook over HTTP
3. Go code processes a typed object

**Advantages:**
- Compile-time type safety
- Single process (controller)
- No JSON parsing (schema is used)
- Testability

## Function Mapping

| Python function | Go function | File |
|----------------|------------|------|
| `alpha1_to_alpha2()` | `ConvertTo()` + `Convert_v1alpha1_*` | `v1alpha1/nodegroup_conversion.go`, `v1alpha1/conversion.go` |
| `alpha2_to_alpha1()` | `ConvertFrom()` + `Convert_v1_*` | `v1alpha1/nodegroup_conversion.go`, `v1alpha1/conversion.go` |
| `alpha2_to_v1()` | `ConvertTo()` | `v1alpha2/nodegroup_conversion.go` |
| `v1_to_alpha2()` | `ConvertFrom()` | `v1alpha2/nodegroup_conversion.go` |

## Detailed Logic Comparison

### 1. alpha1_to_alpha2: docker → cri.docker

**Python:**
```python
def alpha1_to_alpha2(self, o):
    obj.apiVersion = "deckhouse.io/v1alpha2"
    if "docker" in obj.spec:
        if "cri" not in obj.spec:
            obj.spec.cri = {}
        obj.spec.cri.docker = obj.spec.docker
        del obj.spec.docker
    if "kubernetesVersion" in obj.spec:
        del obj.spec.kubernetesVersion
    if "static" in obj:
        del obj.static
```

**Go (v1alpha1/conversion.go):**
```go
func Convert_v1alpha1_NodeGroupSpec_To_v1_NodeGroupSpec(in *NodeGroupSpec, out *v1.NodeGroupSpec, s conversion.Scope) error {
    // Docker field in v1alpha1 maps to CRI.Docker in v1
    if in.Docker != nil {
        if out.CRI == nil {
            out.CRI = &v1.CRISpec{}
        }
        if out.CRI.Type == "" {
            out.CRI.Type = v1.CRITypeDocker
        }
        out.CRI.Docker = &v1.DockerSpec{
            MaxConcurrentDownloads: in.Docker.MaxConcurrentDownloads,
            Manage:                 in.Docker.Manage,
        }
    }
    // kubernetesVersion and static are simply not converted (lost)
    return nil
}
```

### 2. alpha2_to_v1: nodeType mapping + cluster config

**Python:**
```python
def alpha2_to_v1(self, o):
    obj.apiVersion = "deckhouse.io/v1"
    
    # Read cluster config from Secret
    provider_config = get_from_secret("d8-provider-cluster-configuration")
    
    ng_name = obj.metadata.name
    ng_type = obj.spec.nodeType
    
    if ng_type == "Cloud":
        ng_type = "CloudEphemeral"
    elif ng_type == "Hybrid":
        # Determine CloudPermanent vs CloudStatic
        found_in_permanent = False
        if ng_name == "master":
            found_in_permanent = True
        else:
            for ng in provider_config["nodeGroups"]:
                if ng["name"] == ng_name:
                    found_in_permanent = True
                    break
        ng_type = "CloudPermanent" if found_in_permanent else "CloudStatic"
    
    obj.spec.nodeType = ng_type
```

**Go (v1alpha2/nodegroup_conversion.go):**
```go
func (src *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*v1.NodeGroup)
    
    switch src.Spec.NodeType {
    case NodeTypeCloud:
        dst.Spec.NodeType = v1.NodeTypeCloudEphemeral
    case NodeTypeStatic:
        dst.Spec.NodeType = v1.NodeTypeStatic
    case NodeTypeHybrid:
        // Default to CloudStatic
        // CloudPermanent detection requires cluster config lookup
        // which is handled by ConversionWebhook with cluster config access
        dst.Spec.NodeType = v1.NodeTypeCloudStatic
    }
    
    return nil
}
```

### 3. Problem: Hybrid → CloudPermanent requires cluster config

**In the Python hook:**
```python
# Hook has access to Secret via snapshot
includeSnapshotsFrom: ["cluster_config"]

# In code:
provider_config = base64.decode(self._snapshots["cluster_config"][0]["filterResult"])
```

**In Go there are two options:**

**Option A: Conversion Webhook with cluster config access**
```go
// api/deckhouse.io/v1alpha2/nodegroup_webhook.go

type NodeGroupWebhook struct {
    Client client.Client
}

func (w *NodeGroupWebhook) ConvertTo(src *NodeGroup, dst *v1.NodeGroup) error {
    if src.Spec.NodeType == NodeTypeHybrid {
        // Read Secret
        secret := &corev1.Secret{}
        w.Client.Get(ctx, types.NamespacedName{
            Namespace: "kube-system",
            Name:      "d8-provider-cluster-configuration",
        }, secret)
        
        providerConfig := parseConfig(secret.Data["cloud-provider-cluster-configuration.yaml"])
        
        if isCloudPermanent(src.Name, providerConfig) {
            dst.Spec.NodeType = v1.NodeTypeCloudPermanent
        } else {
            dst.Spec.NodeType = v1.NodeTypeCloudStatic
        }
    }
    return nil
}
```

**Option B: Use an annotation to pass information**
```yaml
# When creating a NodeGroup, an external component adds an annotation
metadata:
  annotations:
    node.deckhouse.io/permanent-node-group: "true"
```

```go
func (src *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
    if src.Spec.NodeType == NodeTypeHybrid {
        if src.Annotations["node.deckhouse.io/permanent-node-group"] == "true" {
            dst.Spec.NodeType = v1.NodeTypeCloudPermanent
        } else {
            dst.Spec.NodeType = v1.NodeTypeCloudStatic
        }
    }
    return nil
}
```

### 4. v1_to_alpha2: Reverse mapping

**Python:**
```python
def v1_to_alpha2(self, o):
    obj.apiVersion = "deckhouse.io/v1alpha2"
    
    ng_type = obj.spec.nodeType
    if ng_type == "CloudEphemeral":
        ng_type = "Cloud"
    elif ng_type in ["CloudPermanent", "CloudStatic"]:
        ng_type = "Hybrid"
    
    obj.spec.nodeType = ng_type
```

**Go (v1alpha2/nodegroup_conversion.go):**
```go
func (dst *NodeGroup) ConvertFrom(srcRaw conversion.Hub) error {
    src := srcRaw.(*v1.NodeGroup)
    
    switch src.Spec.NodeType {
    case v1.NodeTypeCloudEphemeral:
        dst.Spec.NodeType = NodeTypeCloud
    case v1.NodeTypeStatic:
        dst.Spec.NodeType = NodeTypeStatic
    case v1.NodeTypeCloudPermanent, v1.NodeTypeCloudStatic:
        dst.Spec.NodeType = NodeTypeHybrid
    }
    
    return nil
}
```

## Conversion Chain

### Python (shell-operator)

```
v1alpha1 → v1alpha2 → v1 (hub)
    │          │
    └── alpha1_to_alpha2()
               └── alpha2_to_v1()
```

Shell-operator implements **direct conversions** between adjacent versions.

### Go (controller-runtime)

```
v1alpha1 ─────────────────► v1 (hub)
              ConvertTo()

v1alpha2 ─────────────────► v1 (hub)
              ConvertTo()
```

Controller-runtime implements **hub-and-spoke**: each version converts directly to the hub.

## Fields Lost During Conversion

### v1alpha1 → v1 (lost)

| Field | Reason |
|-------|--------|
| `spec.kubernetesVersion` | Deprecated, managed by cluster |
| `spec.static.internalNetworkCIDRs` | No equivalent in v1 |

### v1 → v1alpha1 (lost)

| Field | Reason |
|-------|--------|
| `spec.gpu` | New in v1 |
| `spec.fencing` | New in v1 |
| `spec.update` | New in v1 |
| `spec.staticInstances` | New in v1 |
| `spec.cri.containerdV2` | New in v1 (downgrade to containerd) |
| `spec.cri.notManaged` | New in v1 |
| `spec.kubelet.resourceReservation` | New in v1 |
| `spec.kubelet.topologyManager` | New in v1 |

## Testing

```go
// api/deckhouse.io/v1alpha1/conversion_test.go

func TestConvertTo(t *testing.T) {
    src := &v1alpha1.NodeGroup{
        Spec: v1alpha1.NodeGroupSpec{
            NodeType: v1alpha1.NodeTypeCloud,
            Docker: &v1alpha1.DockerSpec{
                MaxConcurrentDownloads: ptr.To(5),
            },
        },
    }
    
    dst := &v1.NodeGroup{}
    err := src.ConvertTo(dst)
    
    assert.NoError(t, err)
    assert.Equal(t, v1.NodeTypeCloudEphemeral, dst.Spec.NodeType)
    assert.Equal(t, v1.CRITypeDocker, dst.Spec.CRI.Type)
    assert.Equal(t, 5, *dst.Spec.CRI.Docker.MaxConcurrentDownloads)
}
```

## Summary

| Aspect | Python Hook | Go Conversion |
|--------|-------------|---------------|
| Type safety | No | Yes |
| Testability | Hard | Easy |
| Performance | HTTP + JSON | Direct call |
| Cluster config access | Via snapshots | Via webhook with client |
| Complexity | Low | Medium |

## Part 2: Validation Webhook (node_group bash hook)

### Original Hook (Bash)

```bash
# modules/040-node-manager/hooks/node_group

kubernetesValidating:
- name: nodegroup-policy.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["nodegroups"]
```

### What the Hook Reads

| Snapshot | Resource | What it extracts |
|----------|----------|------------------|
| `endpoints` | Endpoints/kubernetes | API server endpoint count |
| `cluster_config` | Secret/d8-cluster-configuration | defaultCRI, clusterPrefixLen, clusterType, podSubnetNodeCIDRPrefix |
| `provider_cluster_config` | Secret/d8-provider-cluster-configuration | Available zones |
| `deckhouse_config` | ModuleConfig/global | customTolerationKeys |
| `nodes_with_containerd_custom_conf` | Nodes with label `containerd-config=custom` | Nodes with custom containerd |
| `nodes_without_containerd_support` | Nodes with label `containerd-v2-unsupported` | Nodes without containerd v2 support |

### All Validations

| # | Validation | When | Go implementation |
|---|-----------|------|-------------------|
| 1 | `clusterPrefix + ngName <= 42` | CREATE, Cloud | `NodeGroupValidator.Handle()` |
| 2 | `maxPerZone >= minPerZone` | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 3 | `maxPods` vs subnet size | CREATE/UPDATE | Warning in `NodeGroupValidator` |
| 4 | Zone exists in provider | CREATE/UPDATE | `loadProviderClusterConfig()` |
| 5 | `cri.type != Docker` | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 6 | CRI change on master < 3 endpoints | UPDATE | Warning, `getKubernetesEndpointsCount()` |
| 7 | Taints in customTolerationKeys | CREATE/UPDATE | `loadCustomTolerationKeys()` |
| 8 | RollingUpdate only for CloudEphemeral | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 9 | `nodeType` is immutable | UPDATE | `NodeGroupValidator.Handle()` |
| 10 | No duplicate taints | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 11 | topologyManager requires resourceReservation | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 12 | CRI change blocked by custom containerd | UPDATE | `getNodesWithCustomContainerd()` |
| 13 | ContainerdV2 blocked by unsupported nodes | UPDATE | `getNodesWithoutContainerdV2Support()` |
| 14 | memorySwap requires cgroup v2 | UPDATE | `getNodesWithoutContainerdV2Support()` |

### Go Implementation

```go
// internal/webhook/validator.go

type NodeGroupValidator struct {
    Client  client.Client
    decoder *admission.Decoder
}

func (v *NodeGroupValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
    // Load cluster config
    clusterConfig, _ := v.loadClusterConfig(ctx)
    providerConfig, _ := v.loadProviderClusterConfig(ctx)
    
    // Validation 1: prefix + name <= 42
    if req.Operation == "CREATE" && clusterConfig.ClusterType == "Cloud" {
        if 63-clusterConfig.ClusterPrefixLen-1-len(ng.Name)-21 < 0 {
            return admission.Denied("...")
        }
    }
    
    // Validation 2: maxPerZone >= minPerZone
    if ng.Spec.CloudInstances != nil {
        if ng.Spec.CloudInstances.MaxPerZone < ng.Spec.CloudInstances.MinPerZone {
            return admission.Denied("...")
        }
    }
    
    // ... remaining validations
    
    return admission.Allowed("")
}
```

### Webhook Registration

```go
// cmd/main.go

mgr.GetWebhookServer().Register("/validate-nodegroup-policy", &webhook.Admission{
    Handler: &validator.NodeGroupValidator{
        Client: mgr.GetClient(),
    },
})
```

### Comparison: Bash vs Go

| Aspect | Bash Hook | Go Validator |
|--------|-----------|--------------|
| Type safety | No (JSON/jq) | Yes (Go structs) |
| Cluster access | Via snapshots | Via client.Client |
| Errors | Runtime | Compile-time |
| Testability | Hard | Unit tests |
| Performance | New process | In-memory |

## Project Files

```
node-controller/
├── api/deckhouse.io/
│   ├── v1/
│   │   ├── nodegroup_webhook.go      # Simple defaulting/validation
│   │   └── nodegroup_conversion.go   # Hub() marker
│   ├── v1alpha1/
│   │   ├── nodegroup_conversion.go   # ConvertTo/ConvertFrom
│   │   └── conversion.go             # docker → cri.docker
│   └── v1alpha2/
│       └── nodegroup_conversion.go   # ConvertTo/ConvertFrom
│
├── internal/
│   ├── webhook/
│   │   └── validator.go              # Custom validator with cluster access
│   ├── controller/
│   │   └── nodegroup_controller.go   # Reconciliation logic
│   └── ...
│
└── cmd/
    └── main.go                       # Registers both webhooks
```

## Webhook Configuration

```yaml
# config/webhook/webhook-configuration.yaml

apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: node-controller-validating
webhooks:
  # Simple validation (from nodegroup_webhook.go)
  - name: vnodegroup.deckhouse.io
    rules:
      - apiGroups: ["deckhouse.io"]
        resources: ["nodegroups"]
        operations: ["CREATE", "UPDATE", "DELETE"]
    clientConfig:
      service:
        path: /validate-deckhouse-io-v1-nodegroup

  # Policy validation (from validator.go)
  - name: nodegroup-policy.deckhouse.io
    rules:
      - apiGroups: ["deckhouse.io"]
        resources: ["nodegroups"]
        operations: ["CREATE", "UPDATE"]
    clientConfig:
      service:
        path: /validate-nodegroup-policy
```

## Part 3: Reconcile Hooks (k8s → k8s controllers)

Besides the conversion/validation webhooks above, several shell-operator Go hooks
were pure `k8s → k8s` reconcilers (watch objects, patch/delete other objects). These
move to controller-runtime controllers registered via `register.RegisterController`
and blank-imported in `internal/register/controllers/controllers.go`. Each keeps the
same trigger and effect; the reactive watch replaces the hook's converge cadence.

### Migrated controllers

| Controller name | Primary | Replaces hook | Effect |
|-----------------|---------|---------------|--------|
| `node-csi-taint` | `Node` (+watch `CSINode`) | `remove_csi_taints.go` | Remove `node.deckhouse.io/csi-not-bootstrapped` taint once `CSINode.spec.drivers` is non-empty |
| `node-spot-termination` | `Node` | `handle_spot_instance_deletion.go` | Delete the `Instance` of a drained spot node marked `termination-in-progress` |
| `node-kubelet-csr-approver` | `CertificateSigningRequest` | `kubelet_csr_approver.go` | Auto-approve validated `kubernetes.io/kubelet-serving` CSRs |
| `node-nodeuser-error-cleanup` | `NodeUser` (+watch `Node` deletes) | `clear_nodeuser_errors.go` | Drop `NodeUser.status.errors` entries keyed by nodes that no longer exist |
| `node-machineset-revision-trim` | MCM `MachineSet` | `trim_machine_set_revision_history.go` | Cap the `deployment.kubernetes.io/revision-history` annotation to the first revision |
| `node-instanceclass-ng-usage` | `NodeGroup` | `set_instance_class_ng_usage.go` | Record which NodeGroups consume each cloud `InstanceClass` in `status.nodeGroupConsumers` |
| `capi-crd-migration` (CAPS path) | `CustomResourceDefinition` (+watch caps `Secret`/`Service`) | `sshcredentials_crd_cabundle_injection.go` | Keep the `sshcredentials.deckhouse.io` conversion-webhook CA in sync with the CAPS webhook TLS secret |
| `node-group-configuration-metrics` | `NodeGroupConfiguration` | `metrics_node_group_configurations.go` | Export `d8_node_group_configurations_total{node_group}` — NGC count aggregated by targeted NodeGroup |
| `chaos-monkey` | `NodeGroup` (schedule-driven, one-minute ticker) | `chaos_monkey.go` | Periodically drain-and-delete one random MCM `Machine` of every chaos-enabled, ready NodeGroup |

### node-csi-taint (`internal/controller/csitaint`)

A freshly bootstrapped node carries the `csi-not-bootstrapped` taint (`NoSchedule`)
until its CSI driver registers, observed via the node's `CSINode` object. A CSINode is
named after its node, so the secondary watch maps a registration event to the Node of
the same name.

```
Node / CSINode changed
  ├─ Node has no csi-not-bootstrapped taint?  → skip
  ├─ CSINode not found?                       → still bootstrapping, keep taint
  ├─ CSINode.spec.drivers empty?              → driver not registered, keep taint
  └─ driver registered → strip only the csi-not-bootstrapped taint
```

**Parity note:** the hook's Node binding was passive (`ExecuteHookOnEvents=false`,
`ExecuteHookOnSynchronization=false`) — it removed the taint only on the `OnBeforeHelm`
converge or a `CSINode` filter-result change (minutes). The controller watches Node
reactively (seconds). End state identical; only latency improves.
RBAC: `nodes` get/list/watch/patch, `storage.k8s.io/csinodes` get/list/watch.

### node-spot-termination (`internal/controller/spottermination`)

A reclaimed spot/preemptible VM is labeled `node.deckhouse.io/termination-in-progress`
by the provider. Once the node is also drained (`update.node.deckhouse.io/drained`
annotation), the matching `Instance` CR is deleted so machine-controller-manager tears
the machine/VM down.

```
Node changed
  ├─ label termination-in-progress != "true"? → skip
  ├─ no drained annotation?                    → skip
  └─ both present → delete Instance named after the node (NotFound = ok)
```

**Fix relative to the hook:** the hook hardcoded `DeleteInBackground("deckhouse.io/v1alpha1", "Instance", ...)`.
Instance graduated to `v1alpha2` (v1alpha1 `served: false` since PR #18795, 2026-05-07),
so the hook's delete silently failed (`apiVersion 'deckhouse.io/v1alpha1' ... is not
supported by cluster`) and looped in the shell-operator retry queue — the Instance was
never removed (orphan risk on real spot reclamation). The controller deletes via the
typed `v1alpha2` client, targeting the served version through the scheme's RESTMapper.
The migration is parity of the *intended* behavior plus a regression fix.
RBAC: `nodes` get/list/watch, `deckhouse.io/instances` delete (version-agnostic).

### node-kubelet-csr-approver (`internal/controller/kubeletcsrapprover`)

When a kubelet rotates its serving certificate it submits a CSR signed by
`kubernetes.io/kubelet-serving` that no built-in approver handles. The controller
validates and approves it via the `approval` subresource.

```
CSR changed
  ├─ status.certificate already set?          → issued, skip
  ├─ already Approved or Denied?              → skip
  ├─ request PEM does not parse?              → skip (do not approve)
  ├─ signer == kubernetes.io/kubelet-serving? → validate (org system:nodes, CN
  │     system:node: prefix, exact serving usages, IP|DNS SAN, no email/URI SAN,
  │     username == CN); fails → skip
  └─ approve (append Approved condition, message "autoapproved by Deckhouse")
```

**Parity note (approve-on-parse quirk):** the hook approved *any* CSR whose PEM
parses; only `kubelet-serving` CSRs got the extra validation above. The hook ran
under the `deckhouse` ServiceAccount (bound to `cluster-admin`), so it could
approve every signer. The controller preserves this branch logic but runs under
the scoped node-controller ServiceAccount, whose `signers` `approve` grant is
limited to `kubernetes.io/kube-apiserver-client` (apiproxycert) and
`kubernetes.io/kubelet-serving` — so in practice it approves a strictly narrower
set than the hook, and RBAC blocks approval of any other signer.
RBAC: `certificatesigningrequests` get/list/watch, `.../approval` update,
`signers` `approve` on `kubernetes.io/kubelet-serving` (added) and
`kubernetes.io/kube-apiserver-client` (pre-existing).

### node-nodeuser-error-cleanup (`internal/controller/nodeusercleanup`)

`NodeUser.status.errors` is a map keyed by node name holding per-node provisioning
errors. When a node is removed its entry can linger. The controller drops entries
whose node no longer exists among the nodes carrying the `node.deckhouse.io/group`
label.

```
NodeUser changed / Node deleted (re-enqueues every NodeUser)
  ├─ status.errors empty?                     → skip
  ├─ compute stale = error keys not among labeled group nodes
  ├─ no stale keys?                           → skip
  └─ JSON merge patch status.errors {key: null, ...} on the status subresource
```

**Parity note:** the hook ran on a 30-minute `Schedule` plus a passive Node/NodeUser
Synchronization binding (`ExecuteHookOnEvents=false`), so a stale entry lingered until
the next cron tick or an operator restart. The controller reconciles a NodeUser
reactively on its own changes and re-checks every NodeUser on a Node deletion (only
deletions can strand an entry; create/update never turns an existing entry stale), so
stale entries clear promptly. The node set (`node.deckhouse.io/group` label), the stale
computation and the null-valued merge patch on `/status` match the hook exactly.
RBAC: `nodeusers` get/list/watch, `nodeusers/status` patch, `nodes` list (pre-existing).

### node-machineset-revision-trim (`internal/controller/machinesetrevision`)

The machine-controller-manager records every rollout revision of a MachineDeployment in
its child MachineSet's `deployment.kubernetes.io/revision-history` annotation as an
ever-growing comma-separated list. The controller collapses it to the first revision once
it exceeds a small length bound, so the annotation cannot grow without limit.

```
MachineSet changed (machine.sapcloud.io/v1alpha1, ns d8-cloud-instance-manager)
  ├─ namespace != d8-cloud-instance-manager?  → skip (hook NamespaceSelector parity)
  ├─ revision-history length <= 16?           → skip
  ├─ nothing before the first comma to trim?  → skip (unchanged value)
  └─ merge-patch revision-history = first revision (other annotations preserved)
```

**Parity note:** the MachineSet type is not in the node-controller scheme, so the primary
object is an `unstructured.Unstructured` with the MCM GVK (the same pattern the CAPI/MCM
controllers use for MachineDeployment). The two guards (`len > 16` and "trimming actually
changes the value", i.e. there is a comma) and the single-annotation merge patch match the
hook exactly; a value longer than 16 chars but without a comma is left untouched. The
hook's MachineSet binding was event-driven, so the reactive watch keeps identical latency.
RBAC: `machine.sapcloud.io/machinesets` get/list/watch/patch (added).

### node-instanceclass-ng-usage (`internal/controller/instanceclassusage`)

Records the reverse reference of each cloud `InstanceClass`: the list of NodeGroups that
consume it, written to `InstanceClass.status.nodeGroupConsumers`. The validating webhook
refuses to delete an InstanceClass whose consumer list is non-empty, so this protects an
in-use class from deletion.

```
NodeGroup changed
  ├─ read active kind from Secret kube-system/d8-node-manager-cloud-provider[instanceClassKind]
  ├─ kind empty (no cloud provider)?          → skip
  ├─ build icName -> [ngName] from every CloudEphemeral NodeGroup whose
  │    spec.cloudInstances.classReference.kind == active kind
  └─ for each InstanceClass of the active kind:
       desired = sorted consumers (or [] if unused)
       status.nodeGroupConsumers already equal? → skip
       merge-patch status.nodeGroupConsumers = desired (NotFound = ok)
```

**Parity note:** the hook had three bindings but only the `NodeGroup` one was active
(its `InstanceClass` and cloud-provider-`Secret` bindings were passive,
`ExecuteHookOnEvents=false`), so the primary `NodeGroup` watch reproduces every trigger;
each reconcile recomputes the consumer lists for all InstanceClasses of the active kind.
InstanceClass CRDs have **no status subresource** (verified for both `deckhouse.io/v1alpha1`
and `deckhouse.io/v1`), so `status.nodeGroupConsumers` is a plain field patched on the main
resource — not via `Status().Patch()`. All provider kinds serve `deckhouse.io/v1alpha1`
(v1-only kinds also serve v1alpha1 via conversion), so a single version lists and patches
every kind; the controller drops the hook's `kindToVersion` map and dynamic-kind
`BindingAction` (UpdateKind/Disable) machinery, which was needed only by shell-operator's
kind-bound snapshot mechanism. RBAC: the InstanceClass resources gain `patch` on top of the
existing get/list/watch; NodeGroup list and the cloud-provider Secret read already exist.

### sshcredentials CA injection (`internal/controller/crdmigration`, CAPS path)

The `sshcredentials.deckhouse.io` CRD conversion is served by the CAPS (Cluster API Provider
Static) webhook; its CA must be injected into the CRD's `spec.conversion.webhook.clientConfig`
so the API server trusts the conversion webhook. This is the same "inject CA into a CRD
conversion webhook" pattern the `crdmigration` controller already runs for CAPI CRDs and the
deckhouse CRDs (nodegroups/instances), so it was added there as a third mapping rather than a
new controller.

```
CRD sshcredentials.deckhouse.io / caps Secret / caps Service changed
  ├─ caps-controller-manager-webhook-service missing?   → requeue (gate parity)
  ├─ caps-controller-manager-webhook-tls missing/empty? → requeue
  ├─ CRD absent?                                         → skip (no-op)
  ├─ conversion already points at caps service with same CA? → skip (idempotent)
  └─ patch spec.conversion = Webhook{ service caps-controller-manager-webhook-service
       (/convert, :443), caBundle = secret ca.crt, conversionReviewVersions [v1] }
```

**Parity note:** the hook had a passive CRD binding plus active `Secret`
(`caps-controller-manager-webhook-tls`) and `Service`
(`caps-controller-manager-webhook-service`) bindings and an `OnAfterAll` trigger, so it
restored the CA only on a converge (verified live: a perturbed caBundle was restored only
after a Deckhouse restart re-ran the hook). With the CRD as the controller's **primary**
object, the caBundle is restored the instant the CRD is perturbed — end state identical,
latency improved. The webhook Service existence gate mirrors the hook's `webhook-service`
binding (no point pointing the CRD at a missing service). The shared `patchConversionWebhook`
/`isConversionWebhookCurrent` helpers were parameterised by service name; the CAPI and
deckhouse-CRD paths are unchanged. RBAC: cluster-wide `secrets`, `services` and
`customresourcedefinitions` access already exists — no new grants.

### node-group-configuration-metrics (`internal/controller/ngconfigmetrics`)

Exports `d8_node_group_configurations_total{node_group}`: the number of
`NodeGroupConfiguration` objects that target each NodeGroup. A configuration without
`spec.nodeGroups` targets all groups and is counted under `node_group="*"`.

```
NodeGroupConfiguration changed
  ├─ list all NodeGroupConfigurations
  ├─ for each: targets = spec.nodeGroups (absent → ["*"]; explicit [] → none)
  │    countByNodeGroup[target]++
  ├─ gauge.Reset()            (drops series for deleted/retargeted configs)
  └─ set d8_node_group_configurations_total{node_group=target} = count
```

**Parity note:** the hook's only binding was the `NodeGroupConfiguration` watch
(`ExecuteHookOnSynchronization=true`); on every event it called
`MetricsCollector.Expire("node_group_configurations")` and re-emitted one series per
`node_group`. The controller reproduces this exactly: a `NodeGroupConfiguration` primary
watch, a full recompute via `List`, and a `GaugeVec.Reset()` before setting the current
counts (the direct equivalent of the group `Expire`). The default-to-`*` rule fires only
when `spec.nodeGroups` is absent — an explicit empty list yields no series, matching the
hook's `NestedStringSlice` `ok` check. Verified live on `deni-static-master-0`: three `*`
NGCs gave `node_group="*" => 3`, and adding a `parity-test-ng`-targeting NGC produced
`node_group="parity-test-ng" => 1` alongside it. RBAC: `nodegroupconfigurations`
get/list/watch (read-only, added).

### chaos-monkey (`internal/controller/chaosmonkey`)

Periodically kills one random node of every NodeGroup whose `spec.chaos.mode` is
`DrainAndDelete`, by deleting the node's MCM `Machine`
(`machine.sapcloud.io/v1alpha1`, namespace `d8-cloud-instance-manager`); MCM then
drains the node, deletes the VM and recreates it.

```
every minute (ticker)
  ├─ list Machines; if ANY is already flagged victim → skip this tick (global gate)
  ├─ list chaos-enabled, ready NodeGroups (isReadyForChaos)
  ├─ for each NG with mode == DrainAndDelete:
  │    ├─ periodMinutes = chaos.period in minutes (default 6h); ≤0 → skip
  │    ├─ probability gate: rand % periodMinutes != 0 → skip
  │    ├─ pick a random node of the NG → its Machine
  │    └─ annotate Machine chaos-monkey-victim, then delete it
```

**Parity note:** the hook (`chaos_monkey.go`) was schedule-only — crontab
`* * * * *` with purely passive Kubernetes bindings — so the controller uses a
one-minute ticker raw source rather than a reactive watch. NodeGroup/Node/Machine
events must NOT trigger it (an always-false `WithEventFilter` drops all primary
events; raw sources bypass the filter), or the per-minute probability gate would
fire more often and make chaos more aggressive. `isReadyForChaos` mirrors the hook:
a `CloudEphemeral` group is ready when `status.desired > 1 && desired == ready`, any
other group uses `status.nodes` instead. The victim gate keys on the `Machine` label
`chaos-monkey-victim` while the flag is written as an annotation (asymmetry kept 1:1
with the hook). The random seed is `time.Now().UnixNano()`, overridable by
`D8_TEST_RANDOM_SEED` for tests. A sub-minute period is skipped instead of panicking
on a zero modulus (the whole controller binary would crash, unlike the isolated hook).
Scope is MCM only, matching the hook 1:1; CAPI-backed NodeGroups
(`cluster.x-k8s.io`) are not handled yet (migration TODO). Baseline verified live on
`deni-yand-main` (Yandex + MCM, NG `mcm-test`): `period: 1m` made the hook annotate a
`Machine` with `chaos-monkey-victim` and set its `deletionTimestamp` at 15:46:00Z.
RBAC: existing `machine.sapcloud.io/machines` get/list/watch/update/patch/delete and
`nodes`/`nodegroups` read grants already cover it — no new grants.
