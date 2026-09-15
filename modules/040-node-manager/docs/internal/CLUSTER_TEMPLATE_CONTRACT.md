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

The controller applies mutable provider resources with server-side apply. An existing
`spec.controlPlaneEndpoint` is preserved: some infrastructure providers treat that field as
set-once, while the discovered control-plane addresses can change later.

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

`credentials.yaml` is optional and must render exactly one `v1/Secret`. Both objects must be in
`d8-cloud-instance-manager`; node-controller fills that namespace when it is omitted. A Secret
previously created from `credentials.yaml` is removed when the file disappears or starts rendering
a Secret with another name.

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
`node-controller` field manager, removes Helm release ownership metadata, and keeps the same
object UID. Provider-specific metadata and the keep annotation remain.
