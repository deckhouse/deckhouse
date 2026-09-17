---
title: "CAPI cluster-template contract v1"
description: How a cloud-provider module describes its infrastructure cluster and credentials for node-controller.
---

This is the single source of truth for `capi/cluster.yaml` and the optional
`capi/credentials.yaml` shipped by a cloud-provider module. **If something is not in this
document, it is not in the contract.** Bump `version` only for an incompatible change; document
an added field with the Deckhouse release in which it became available.

The machine-template contract uses the same data model and sandbox; see
[`MACHINE_TEMPLATE_CONTRACT.md`](MACHINE_TEMPLATE_CONTRACT.md).

## What node-controller does with the files

The provider module puts the files into the `d8-cloud-provider-<type>-capi` Secret in
`kube-system`. The ClusterReconciler reads and validates both templates before applying either
object. It also creates the generic CAPI `Cluster`, `DeckhouseControlPlane`, and
`MachineHealthCheck` resources.

The controller applies mutable provider resources with server-side apply. A live
`spec.controlPlaneEndpoint` is preserved when the template renders none: most infrastructure
providers fill that field in themselves, and applying an object without it would wipe what the
provider discovered. A template that does render the endpoint owns it, and the rendered value
wins on every apply: it comes from the apiserver addresses node-controller watches, so replacing
a master updates it.

## File shape

Both files use the same strict envelope:

```yaml
version: v1
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: OpenStackCluster
  metadata:
    name: {{ .cluster.name | quote }}
    namespace: {{ .cluster.namespace | quote }}
  spec: {}
```

Unknown envelope fields, an unsupported version, an empty template, or a malformed template make
the reconcile fail.

`cluster.yaml` must render exactly one Kubernetes object. Its `apiVersion`, `kind`, and
`metadata.name` must match `capiClusterAPIVersion`, `capiClusterKind`, and `capiClusterName` from
the provider registration Secret.

`credentials.yaml` is optional and must render exactly one `v1/Secret` named
`capi-user-credentials`. The fixed name is part of the migration contract: the before-Helm hook
must identify and protect the existing Secret before the old manifest is removed. Both objects must be in
`d8-cloud-instance-manager`; node-controller fills that namespace when it is omitted. A Secret
previously created from `credentials.yaml` is removed when the file disappears.

## Render context

### `cluster.yaml`

| Root | Type | Since | What it is |
|---|---|---|---|
| `.provider` | map | v1 | Provider-owned subtree from `d8-node-manager-cloud-provider`. |
| `.cluster` | map | v1 | Common cluster facts listed below. |
| `.prefix` | string | v1 | Effective cluster prefix. It may be empty. |
| `.controlPlane.endpoints` | list | v1 | Discovered API endpoints as `{host, port}`. The `.controlPlane` root is absent when no endpoint is available. |

### `credentials.yaml`

This file sees only `.provider` and `.cluster`. It cannot read `.prefix` or `.controlPlane`.

### `.cluster` keys

| Key | Type | Since | What it is |
|---|---|---|---|
| `.cluster.name` | string | v1 | CAPI Cluster name. |
| `.cluster.namespace` | string | v1 | Namespace of CAPI resources. |
| `.cluster.uuid` | string | v1 | Deckhouse cluster UUID. |
| `.cluster.podSubnet` | string | v1 | Pod network CIDR. |

There is no `.Values`, `.Chart`, `.Files`, or `.Release`. The provider publishes the values it
needs through its subtree in `d8-node-manager-cloud-provider`; node-controller supplies the
common cluster facts.

## Template rules

1. Render one YAML document per file. Put credentials in `credentials.yaml`, not beside the
   infrastructure object in a multi-document `cluster.yaml`.
2. Treat absent values as errors unless they are genuinely optional. The renderer uses
   `missingkey=error`; use `get` or `hasKey` for optional map keys.
3. Keep the result deterministic. The sandbox excludes clocks, random values, host environment,
   network access, cryptographic generators, and Helm-only functions such as `include`, `tpl`, and
   `lookup`.
4. Keep provider-specific labels, such as the provider controller's `app` label, in the template.
   node-controller adds the common `heritage`, `module`, and `helm.sh/resource-policy: keep`
   metadata.

## Helm ownership handover

During upgrade, the before-Helm migration hook adds `helm.sh/resource-policy: keep` to existing
resources before the old Helm templates disappear. ClusterReconciler applies the object under the
`node-controller` field manager and keeps the same object UID. Provider-specific metadata, the
keep annotation and Helm's own ownership metadata all remain.

Keeping Helm's metadata is not what makes a rollback work: addon-operator upgrades releases with
`TakeOwnership`, so Helm adopts a live object by kind and name and never reads that metadata.
It is kept because nothing needs it removed, and because it truthfully describes an object Helm
may own again.

The keep annotation is stamped on every apply and never removed. Helm skips an annotated object
both on prune and on uninstall, and node-controller does not delete these objects either, so
disabling the module leaves them in the cluster — `capi-user-credentials` among them, with live
cloud credentials in it. Prune compares against the previous release manifest, so the annotation
only matters for the single upgrade that moves an object out of the chart: drop it together with
`hooks/set_keep_policy_on_capi_resources.go`, which carries the same removal note.
