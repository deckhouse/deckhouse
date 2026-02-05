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
