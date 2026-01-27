# Node Controller

Standalone Kubernetes controller for managing NodeGroup resources in Deckhouse.

## Architecture

This controller follows the same patterns as `caps-controller-manager`:

- **Scope Pattern**: Uses `scope` package for encapsulating reconciliation context
- **Event Recorder**: Sends Kubernetes events for important operations
- **Conversion Webhooks**: Supports multiple API versions with automatic conversion
- **controller-runtime**: Built on the standard Kubernetes controller framework

## API Versions

| Version | Status | nodeType values |
|---------|--------|-----------------|
| v1 | Hub (storage) | CloudEphemeral, CloudPermanent, CloudStatic, Static |
| v1alpha2 | Spoke | Cloud, Static, Hybrid (+ NotManaged CRI) |
| v1alpha1 | Spoke | Cloud, Static, Hybrid |

### Conversion Mapping

```
v1alpha1/v1alpha2 → v1:
  Cloud  → CloudEphemeral
  Static → Static
  Hybrid → CloudStatic

v1 → v1alpha1/v1alpha2:
  CloudEphemeral → Cloud
  CloudPermanent → Hybrid
  CloudStatic    → Hybrid
  Static         → Static
```

## Project Structure

```
node-controller/
├── api/deckhouse.io/
│   ├── v1/                      # Hub version (storage)
│   │   ├── groupversion_info.go # API group registration
│   │   ├── nodegroup_types.go   # Type definitions
│   │   ├── nodegroup_conversion.go # Hub() marker
│   │   └── nodegroup_webhook.go # Validation/Defaulting
│   ├── v1alpha1/                # Spoke version
│   │   ├── doc.go               # +k8s:conversion-gen marker
│   │   ├── groupversion_info.go
│   │   ├── nodegroup_types.go
│   │   ├── nodegroup_conversion.go # ConvertTo/ConvertFrom
│   │   └── conversion.go        # Custom field mappings
│   └── v1alpha2/                # Spoke version (+ NotManaged)
│       └── ...
├── cmd/
│   └── main.go                  # Entry point
├── internal/
│   ├── controller/
│   │   └── nodegroup_controller.go # Reconciler
│   ├── scope/
│   │   ├── scope.go             # Base scope
│   │   └── nodegroup_scope.go   # NodeGroup scope
│   └── event/
│       └── recorder.go          # Event recorder
├── config/
│   ├── rbac/rbac.yaml
│   ├── manager/deployment.yaml
│   └── webhook/webhook-configuration.yaml
├── hack/
│   └── boilerplate.go.txt
├── go.mod
├── Makefile
├── Dockerfile
└── PROJECT
```

## Building

```bash
# Generate DeepCopy methods
make generate

# Build binary
make build

# Build Docker image
make docker-build IMG=your-registry/node-controller:tag

# Run locally
make run
```

## Key Patterns (from caps-controller-manager)

### Scope Pattern

```go
// Create scope for NodeGroup operations
nodeGroupScope, err := scope.NewNodeGroupScope(baseScope, nodeGroup, ctx)
defer nodeGroupScope.Close(ctx)

// Load related objects
nodeGroupScope.LoadNodes(ctx)

// Update status
nodeGroupScope.UpdateStatus()
```

### Event Recording

```go
recorder.SendNormalEvent(node, nodeGroup.Name, "NodeUpdated", "Node configuration updated")
recorder.SendWarningEvent(nodeGroup, nodeGroup.Name, "NodeFailed", "Node failed to update")
```

### Conversion Functions

```go
// In spoke version (v1alpha1)
func (src *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*v1.NodeGroup)
    switch src.Spec.NodeType {
    case NodeTypeCloud:
        dst.Spec.NodeType = v1.NodeTypeCloudEphemeral
    }
    return nil
}
```

## License

Apache License 2.0
