# static-provider-id

**Name:** `static-provider-id`
**Primary resource:** `Node`
**Replaces hook:** `set_provider_id_on_static_nodes.go`

## Purpose

Sets `spec.providerID = "static://"` on Static nodes that don't yet have
a providerID and don't have the uninitialized taint. This is required for
proper node lifecycle management in clusters with static (non-cloud) nodes.

## Watched Resources

| Resource | Trigger |
|----------|---------|
| `Node` | Any change (primary resource) |

No additional watches — reacts only to Node changes.

## Reconciliation Logic

```
Node changed
  │
  ├─ Node not found? → done
  │
  ├─ node.deckhouse.io/type != "Static"? → skip
  │
  ├─ spec.providerID already set? → skip
  │
  ├─ Has taint "node.cloudprovider.kubernetes.io/uninitialized"? → skip
  │   (node is still being initialized by cloud provider)
  │
  └─ Patch spec.providerID = "static://" (MergePatch)
```

## Key Constants

| Key | Value |
|-----|-------|
| Node type label | `node.deckhouse.io/type` |
| Expected type | `Static` |
| Uninitialized taint | `node.cloudprovider.kubernetes.io/uninitialized` |
| Provider ID value | `static://` |

## Files

- `controller.go` — reconciler, single file
