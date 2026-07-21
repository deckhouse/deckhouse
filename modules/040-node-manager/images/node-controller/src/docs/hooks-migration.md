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
| `yandex-preemptible-cleanup` | `NodeGroup` (schedule-driven, 15-minute ticker) | `yc_delete_preemptible_instances.go` | Rotate the oldest ~10% of preemptible Yandex MCM `Machine`s (node age > 20h) ahead of the cloud's 24h eviction, keeping each NodeGroup ≥ 0.9 ready |
| `bashible-apiserver-host-ip` | `Pod` (`app=bashible-apiserver`, `d8-cloud-instance-manager`) | `change_host_ip.go` | Record the node host IP into `node.deckhouse.io/initial-host-ip`; delete the Pod when the live `status.hostIP` diverges so it is recreated with a certificate valid for the new address |
| `master-node-group` | `NodeGroup` (name `master` only; startup-enqueued) | `create_master_node_group.go` | Ensure the default `master` NodeGroup exists (create-if-not-exists); nodeType `Static` for a Static cluster, else `CloudPermanent`. Never patches an existing object |
| `bashible-apiserver-lock` | `Deployment` (`bashible-apiserver`, `d8-cloud-instance-manager`) | `lock_bashible_apiserver.go` | Toggle the `node.deckhouse.io/bashible-locked` annotation on Secret `bashible-apiserver-context` (+ metric `d8_bashible_apiserver_locked`) while the apiserver Deployment rolls out to a new image |
| `capi-webhook-cert` | `Service` (`capi-webhook-service`, `d8-cloud-instance-manager`; +watch `Secret`/`ValidatingWebhookConfiguration`/`MutatingWebhookConfiguration`) | `generate_capi_webhook_certs.go` | Issue the self-signed CA + serving cert into Secret `capi-webhook-tls` and inject the CA into the `capi-mutating`/`capi-validating-webhook-configuration` caBundles |
| `bashible-apiserver-cert` | `Service` (`bashible-api`, `d8-cloud-instance-manager`; +watch `Secret`/`APIService`) | `gen_bashible_apiserver_certs.go` | Issue the self-signed CA + serving cert into Secret `bashible-api-server-tls` and inject the CA into the `v1alpha1.bashible.deckhouse.io` APIService caBundle |
| `capi-cluster-resources` (kubeconfig, cloud half) | cloud-provider `Secret` (`d8-node-manager-cloud-provider`) | `generate_capi_kubeconfig.go` (**cloud CAPI half only**) | Issue the `capi-controller-manager` client cert and write the `<capiClusterName>-kubeconfig` Secret. The `static` (CAPS) kubeconfig stays owned by the hook — CAPS is not migrated |

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

### yandex-preemptible-cleanup (`internal/controller/preemptible`)

Proactively rotates the oldest preemptible Yandex.Cloud nodes before the cloud
provider force-stops them. Yandex terminates preemptible VMs after at most 24h, so
once a node's age crosses the 20h (24h-4h) window this controller deletes its MCM
`Machine` (`machine.sapcloud.io/v1alpha1`, namespace `d8-cloud-instance-manager`);
MCM then recreates a fresh node.

```
every 15 minutes (ticker)
  ├─ collect preemptible YandexMachineClass names (spec.schedulingPolicy.preemptible)
  │    none → nothing to rotate (MCM-only scope) → return
  ├─ index Nodes by name → {group, creationTimestamp}
  ├─ index Yandex NodeGroups (classReference.kind == YandexInstanceClass) → {nodes, ready}
  ├─ for each Machine:
  │    ├─ terminating (deletionTimestamp)?                 → skip
  │    ├─ spec.class.kind != YandexMachineClass?           → skip
  │    ├─ class not preemptible?                           → skip
  │    ├─ no Node named after the Machine?                 → skip
  │    ├─ node age < 20h?                                  → skip (too young)
  │    └─ NodeGroup ready/nodes < 0.9?                     → skip (protect availability)
  ├─ sort candidates oldest-first; batch = len/10 (min 1)
  └─ delete the oldest `batch` Machines
```

**Parity note:** the hook (`yc_delete_preemptible_instances.go`) was schedule-only —
crontab `0/15 * * * *` with purely passive Kubernetes bindings — so the controller
uses a 15-minute ticker raw source rather than a reactive watch (an always-false
`WithEventFilter` drops all primary events while the raw source bypasses the filter);
reactive triggers would rotate nodes more aggressively than the intended cadence. The node lookup keys on the
`Machine` name (MCM names the node after its Machine), matching the hook 1:1. The
readiness guard skips NodeGroups with zero nodes, which also avoids the `0/0` NaN the
hook's raw ratio could produce. Scope is MCM only: it reads
`YandexMachineClass.spec.schedulingPolicy.preemptible`. **CAPI is a deliberate no-op,
not a regression** — the Deckhouse CAPI `YandexMachineTemplate` does not render
`preemptible`, and the upstream `cluster-api-provider-yandex` v0.2.0 `YandexMachineSpec`
has no `schedulingPolicy`/`preemptible` field at all, so CAPI Yandex nodes are never
preemptible and there is nothing to rotate. Enabling CAPI preemptible rotation is a
separate provider feature (see the migration TODO / `preemptible-capi` plan). Baseline
is time-gated (the hook only acts on a real >20h-old preemptible node; a node's
`creationTimestamp` cannot be forged), so parity is covered by deterministic unit
tests rather than a same-session live 2a. RBAC: existing
`machine.sapcloud.io/machines` get/list/watch/delete and
`yandexmachineclasses`/`nodes`/`nodegroups` read grants already cover it — no new grants.

### bashible-apiserver-host-ip (`internal/controller/hostipchange`)

Keeps the `bashible-apiserver` Pod consistent with the host IP of the node it runs
on. bashible-apiserver serves node bootstrap data over a certificate pinned to its
host IP; if the node reappears with a different IP (e.g. a reboot with a new DHCP
lease) the Pod must be recreated so its certificate matches the new address.

```
Pod app=bashible-apiserver in d8-cloud-instance-manager (reactive watch)
  ├─ status.hostIP == ""?                          → skip (not scheduled yet)
  ├─ annotation node.deckhouse.io/initial-host-ip absent?
  │    → merge-patch it = current status.hostIP    (record initial IP)
  └─ initial-host-ip != status.hostIP?             → delete Pod (Deployment recreates)
```

**Parity note:** the hook (`change_host_ip.go`) is a one-liner instantiation of the
shared `go_lib/hooks/change_host_address` library for
`("bashible-apiserver", "d8-cloud-instance-manager")`. That library is still used by
nine other modules (cloud-provider-\*, `002-deckhouse`, `038-registry`) for their own
components, so **only the node-manager bashible-apiserver instance moves here** — the
library and its other callers stay untouched. The controller is reactive: it uses
`For(&Pod{})` with a `WithEventFilter` predicate narrowing to
`app=bashible-apiserver` in `d8-cloud-instance-manager`, reusing the cluster-wide Pod
informer that `bashible-context` already establishes (no extra watch cost). Pods are
`DisableFor` the client cache, so the reconcile `Get` reads the live Pod (fresh
`status.hostIP`) straight from the API. The three branches (skip empty IP / record /
delete on mismatch) mirror the library's `changeHostAddressHandler` 1:1. RBAC: the
existing `pods` grant gained `patch` (record the annotation) alongside the pre-existing
`get`/`list`/`watch`/`delete`. Live baseline verified on deni-yand: faking the
annotation to a stale IP made the old hook delete the Pod, which the Deployment
recreated with `initial-host-ip` re-recorded to the real host IP.

### master-node-group (`internal/controller/masternodegroup`)

Ensures the default `master` NodeGroup metadata object exists. The object is metadata
only: during bootstrap the master Node is registered directly by kubeadm via bashible,
so it is not on the cluster's critical path.

```
Reconcile (request name must be "master", else no-op)
  ├─ master NodeGroup exists?  → do nothing (preserve user edits)
  └─ not found → read clusterType from Secret kube-system/d8-cluster-configuration
                 build default spec (nodeType Static if clusterType==Static, else
                 CloudPermanent) → Create
```

**Parity note:** replaces the `OnStartup{Order:6}` hook `create_master_node_group.go`,
which was `CreateIfNotExists` for the `master` NodeGroup. The controller keeps the same
build-if-absent / never-patch semantics, so user changes are preserved. The object is
built as an **unstructured** value with exactly the hook's fields — a typed NodeGroup
would marshal empty `cri`/`cloudInstances` structs that fail admission validation for a
master NodeGroup. `clusterType` comes from the same Secret `d8-cluster-configuration`
(`cluster-configuration.yaml`, base64-unwrapped) that `derived_status` already reads, so
no new input source. The primary is `NodeGroup` with a `WithEventFilter` predicate
narrowing to name `master` (recreate it if a user deletes it, ignore all other
NodeGroups); a startup raw source enqueues `master` once so it is created on a fresh
cluster where the object does not exist yet and the primary watch would never fire. The
`Create` passes through node-controller's own validating webhook, which is up by the time
the controller reconciles (no chicken-egg — the hook ran on its own queue only because
the addon-operator startup phase raced the warming webhook; a running controller implies
a running webhook). RBAC: the existing `nodegroups` grant gained `create` alongside
`get`/`list`/`watch`/`update`/`patch`.

### bashible-apiserver-lock (`internal/controller/bashiblelock`)

Locks the bashible-apiserver context while its Deployment rolls out to a new image, so
old apiserver Pods do not serve updated context (referencing step templates / image
digests they do not yet have) to nodes. The bashible-apiserver reads the annotation
`node.deckhouse.io/bashible-locked` on Secret `bashible-apiserver-context`
(`images/bashible-apiserver/.../template/context.go` `secretEventHandler.lockApplied`):
`"true"` sets `updateLocked` and freezes context publishing.

```
Reconcile (request must be Deployment bashible-apiserver / d8-cloud-instance-manager)
  ├─ Deployment not found  → no-op (nothing to lock on)
  ├─ rollout complete?     → UNLOCK: remove annotation, metric d8_bashible_apiserver_locked=0
  └─ rollout in progress   → LOCK:   set annotation "true", metric=1

rollout complete :=
  Status.ObservedGeneration >= Generation   (reject stale status right after the spec bump)
  AND UpdatedReplicas == Replicas
  AND AvailableReplicas == Replicas
```

**Parity note:** replaces the `OnBeforeHelm{Order:20}` hook `lock_bashible_apiserver.go`.
The hook compared the *live* Deployment image to the **helm values digest**
`global.modulesImages.digests.nodeManager.bashibleApiserver` (the target image), so it
could lock *before* helm applied. The controller has no access to the values digest and
only observes the Deployment after helm patched it, so it locks on **rollout status**
instead of a digest comparison. The window difference is milliseconds (helm patches the
Deployment `spec` and the `images_digests.json` ConfigMap in the same apply; the
controller reacts to the generation bump immediately), and the race the lock guards
against — an old apiserver Pod serving new context — is still closed. The
`ObservedGeneration` guard rejects the stale-status window right after the image bump,
where the replica counts still reflect the previous generation and would falsely read
"complete". The annotation is written with an idempotent merge patch (a `null` value
removes it); a missing Secret is ignored, matching the hook's `WithIgnoreMissingObject`.
The metric `d8_bashible_apiserver_locked` feeds the `D8BashibleApiserverLocked` alert
(fires when `== 1` for 15m). RBAC: added cluster-wide `apps/deployments`
`get`/`list`/`watch`; the Secret patch is already covered by the namespaced
`bashible-context` Role in `d8-cloud-instance-manager`.

### capi-webhook-cert (`internal/controller/capiwebhookcert`)

Issues the serving certificate for the `capi-controller-manager` admission webhook and
injects its CA into the two CAPI webhook configurations. Replaces the OnBeforeHelm
`tls_certificate.RegisterInternalTLSHook` `generate_capi_webhook_certs.go`.

```
Reconcile (fixed key; any watched object funnels here)
  ├─ Service capi-webhook-service absent → no-op (CAPI disabled)
  ├─ ensureSecret(capi-webhook-tls):
  │     stored CA+leaf still valid (both > 6mo left AND leaf SANs == desired) → reuse
  │     else generate self-signed CA + leaf (ecdsa P256, 10y) → write kubernetes.io/tls Secret
  ├─ inject CA into every webhook of capi-validating/​capi-mutating-webhook-configuration
  │     (skip a config that does not exist yet; patch only when a caBundle differs)
  └─ RequeueAfter 12h (bound renewal latency; the hook re-checked every OnBeforeHelm run)

desired SANs = capi-webhook-service.d8-cloud-instance-manager{,.svc}{,.<clusterDomain>}
  clusterDomain read from Secret kube-system/d8-cluster-configuration (default cluster.local)
```

**Parity note.** The hook generated the CA + leaf into helm values
(`nodeManager.internal.capiControllerManagerWebhookCert`); helm then wrote the
`capi-webhook-tls` Secret (`secret-tls.yaml`) and stamped every `clientConfig.caBundle`
in `webhook.yaml`. nc cannot feed helm values, so it **owns the Secret directly** and
**patches the caBundle** into the live webhook configurations. helm no longer renders a
`caBundle` field in `webhook.yaml` at all (and `secret-tls.yaml` is removed): omitting the
field keeps it out of helm's fieldset, so node-controller is the sole author and a converge
never resets it — there is no caBundle flap. The controller still watches both
configurations and re-injects within seconds if the field is ever cleared externally. The
reuse check mirrors the hook's `isOutdatedCA` +
`isIrrelevantCert`, so a valid cert is left untouched (zero-disruption on rollout). CAPI
enablement is gated on the `capi-webhook-service` Service rather than on the webhook
configurations: the configs carry a werf dependency on the `capi-controller-manager`
Deployment being ready, which in turn mounts `capi-webhook-tls` — gating Secret creation
on the configs would deadlock, while the Service is created early with no such dependency.
The crypto is raw `crypto/x509` (node-controller does not import `go_lib/certificate`); the
leaf carries `ServerAuth` so the API server accepts it when dialing the webhook Service.
RBAC: added cluster-wide `admissionregistration.k8s.io`
`validating`/`mutatingwebhookconfigurations` `get`/`list`/`watch`/`update`/`patch`; the
Secret write is covered by the namespaced secrets Role in `d8-cloud-instance-manager`, and
the Service + `d8-cluster-configuration` reads by the existing cluster-wide
`services`/`secrets` rules.

### bashible-apiserver-cert (`internal/controller/bashibleapiservercert`)

Issues the serving certificate for the aggregated `bashible-apiserver` and injects its CA
into the `v1alpha1.bashible.deckhouse.io` APIService. Replaces the OnBeforeHelm
`gen_bashible_apiserver_certs.go`.

```
Reconcile (fixed key; any watched object funnels here)
  ├─ Service bashible-api absent → no-op (bashible-apiserver not deployed)
  ├─ ensureSecret(bashible-api-server-tls):
  │     stored CA+leaf still valid (both > 6mo left AND leaf SANs == desired) → reuse
  │     else generate self-signed CA + leaf (ecdsa P256, 10y) → write Opaque Secret
  │        (keys ca.crt / apiserver.crt / apiserver.key)
  ├─ inject CA into APIService v1alpha1.bashible.deckhouse.io spec.caBundle
  │     (skip if the APIService does not exist yet; patch only when the caBundle differs)
  └─ RequeueAfter 12h (bound renewal latency; the hook re-checked every OnBeforeHelm run)

desired SANs = 127.0.0.1 (IP) + bashible-api.d8-cloud-instance-manager.svc (DNS), fixed
```

**Parity note.** The hook generated the CA + leaf into helm values
(`nodeManager.internal.bashibleApiServer{CA,Crt,Key}`); helm then wrote the
`bashible-api-server-tls` Secret (`api-server-tls-secret.yaml`) and stamped the APIService
`spec.caBundle` (`apiservice.yaml`). nc cannot feed helm values, so it **owns the Secret
directly** and **patches the caBundle** into the live APIService. helm no longer renders a
`caBundle` field in `apiservice.yaml` at all (and `api-server-tls-secret.yaml` is removed):
omitting the field keeps it out of helm's fieldset, so node-controller is the sole author
and a converge never resets it — there is no caBundle flap. The controller still watches the
Secret and the APIService and re-injects within seconds if the field is ever cleared
externally. The reuse check keeps a valid cert untouched
(zero-disruption on rollout). There is no enablement gate — bashible-apiserver is a core
component and always deployed — so the reconcile simply anchors on the always-present
Service `bashible-api`; nc does not depend on bashible-apiserver being up, so owning its
serving Secret introduces no bootstrap deadlock (on a fresh cluster the bashible-apiserver
pod mount-retries until nc has created the Secret). The crypto is raw `crypto/x509`
(node-controller does not import `go_lib/certificate`); the leaf carries **no ExtKeyUsage**,
mirroring the hook — cfssl mapped `signing`/`key encipherment` to KeyUsage bits and silently
ignored the unknown `requestheader-client`, so the produced serving cert has no EKU
extension and is valid for any purpose (the kube-aggregator accepts it as it dials the
Service as a TLS client). The APIService is patched via `unstructured` because the
kube-aggregator apiregistration types are not part of node-controller's scheme. RBAC: added
cluster-wide `apiregistration.k8s.io` `apiservices` `get`/`list`/`watch`/`update`/`patch`;
the Secret write is covered by the namespaced secrets Role in `d8-cloud-instance-manager`.
**Blast radius:** the injected caBundle governs the node bootstrap path (bashible.sh reaches
the aggregated API through kube-apiserver). Only the very first migration converge flaps the
caBundle once — helm removes the previously templated caBundle before nc re-injects; steady
-state converges no longer touch the field (it is no longer in helm's fieldset), so there is
no recurring flap.

### capi-controller-manager kubeconfig, cloud half (`internal/controller/capi`, `kubeconfig.go`)

Issues the client certificate for `capi-controller-manager` and writes the
`<capiClusterName>-kubeconfig` Secret it mounts to reach the management API server.
Replaces the **cloud CAPI half** of `generate_capi_kubeconfig.go`. It is folded into the
existing `capi-cluster-resources` `ClusterReconciler` (primary: the cloud-provider Secret
`d8-node-manager-cloud-provider`), right after the cloud `Cluster`/`MachineHealthCheck` are
ensured — reusing the same `capiClusterName != ""` gate.

```
ensureCloudCluster (clusterName from cloud-provider Secret, non-empty)
  ├─ create Cluster + MachineHealthCheck (existing)
  └─ ensureKubeconfigSecret(clusterName):
        existing <clusterName>-kubeconfig cert still has > 90d left → no-op
        else issue client cert via CSR (CN capi-controller-manager,
             org d8:node-manager:capi-controller-manager:manager-role,
             kube-apiserver-client signer, ClientAuth, 180d, self-approved)
        build kubeconfig (Host + CA from the manager rest config) → write Secret
             (type cluster.x-k8s.io/secret, key value, label cluster.x-k8s.io/cluster-name)
Reconcile returns RequeueAfter 12h → rotate the 180d cert before expiry
```

**Split note (CAPS stays a hook).** The hook had two branches: the cloud CAPI kubeconfig
(`<capiClusterName>-kubeconfig`) and the `static` CAPS kubeconfig (`static-kubeconfig`).
Only the **cloud** branch is migrated. CAPS is not migrated to node-controller, so the
`static` branch **remains in `generate_capi_kubeconfig.go`** (now CAPS-only, gated on
`capsControllerManagerEnabled`). nc's `ensureStaticCluster` still creates the static
`Cluster`/`MachineHealthCheck` (that predates this migration) but deliberately does **not**
write the static kubeconfig.

**Design.** The cert is minted with the same CSR issue/self-approve/wait/delete flow as the
`apiproxycert` controller (kube-apiserver-client signer), using a typed clientset built from
the manager rest config in `BaseWithReader.Setup`. The kubeconfig's server URL and CA come
from that same rest config (CA loaded from `CAFile` when `CAData` is empty, as in-cluster).
The Secret name/type/label/key are byte-identical to the hook's `internal/kubeconfig`
helper, and no Helm template reads `<cluster>-kubeconfig` (the only runtime consumer is the
`capi-controller-manager` Deployment mounting it), so ownership can move cleanly from helm
patch-collector to nc. The reconcile is a no-op while the stored cert has more than half its
180-day lifetime left, so migration does not roll the existing kubeconfig. RBAC already
covers everything: CSR create/approve on the kube-apiserver-client signer (apiproxycert) and
the namespaced secrets Role in `d8-cloud-instance-manager` (bashible-context) — no new rules.

## Part 4: Hooks subsumed by the bashible context (deleted, no new controller)

Some shell-operator hooks only read a source object and wrote a value into
`nodeManager.internal.*` for Helm to render into the bashible `input.yaml`. After the
bashible cutover, `input.yaml` is written by the node-controller
(`internal/controller/nodegroup/bashiblecontext`), which reads the same source objects
directly. When such a hook's value is **no longer read by any Helm template** (only the
bashible context path remained), the hook is pure duplication and is deleted outright —
no controller is added, because `bashiblecontext` already produces the field.

| Deleted hook | Value it wrote | bashiblecontext reader | Remaining Helm consumer |
|--------------|----------------|------------------------|-------------------------|
| `control_plane_arguments.go` | `internal.nodeStatusUpdateFrequency`, `internal.allowedKubeletFeatureGates` | `readControlPlaneArguments` (`bashiblecontext/sources.go`) | none |

### control_plane_arguments

The hook read Secret `kube-system/d8-control-plane-manager-control-plane-arguments`
(`arguments.json`, `featureGates.json`) and set two values:
`nodeStatusUpdateFrequency = round(nodeMonitorGracePeriod / 4)` and
`allowedKubeletFeatureGates`. Both fed only the bashible `input.yaml`.

`bashiblecontext.readControlPlaneArguments` (`sources.go`) now reads the **same** Secret
directly with the **same** `round(nodeMonitorGracePeriod / 4)` formula and places both
fields into `input.yaml` (`context.go`); bashible-apiserver unmarshals them from the
mounted context Secret (`pkg/template/context_builder.go`). A grep over the module's
`templates/` for `nodeStatusUpdateFrequency`/`allowedKubeletFeatureGates` returns
nothing — no Helm template consumes the values — so the hook's `input.Values.Set` had no
reader left. Removed: the hook, its test, and both fields from `openapi/values.yaml`.
Unlike the other value-feeding V-hooks (`discover_cloud_provider`, `order_bootstrap_token`,
`discover_kubernetes_ca`, `get_packages_proxy_token`, `discover_apiserver_endpoints`),
whose values are still read by the per-NodeGroup bootstrap Secrets in
`node-group/*.tpl`, `control_plane_arguments` fed the bashible context only — which the
controller already owns — so it can be dropped without the bootstrap-secret migration.
