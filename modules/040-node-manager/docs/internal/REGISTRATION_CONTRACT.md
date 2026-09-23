---
title: "Cloud-provider registration contract"
description: Data a cloud-provider module publishes for node-manager and node-controller.
---

Cloud-provider modules publish identical registration data in the legacy
`kube-system/d8-node-manager-cloud-provider` Secret and the provider-specific
`kube-system/d8-node-manager-cloud-provider-<id>` Secret. Values are either plain strings or JSON,
depending on their type. The loader rejects missing, empty, or malformed required data before a
controller starts rendering resources.

Both Secrets must carry the `cloud-provider.deckhouse.io/registration` label. node-controller
validates the contract on the legacy Secret only, but it enumerates InstanceClass kinds by
listing every labelled Secret in `kube-system` and reading `instanceClassKind` and
`instanceClassAPIVersion` out of each one. An unlabelled registration renders resources and then
fails silently elsewhere: no InstanceClass watch is started and the NodeGroup admission cannot
resolve the class. Publish both Secrets, labelled, with the same content.

## Common fields

Every provider must publish:

| Key | Type | Meaning |
|---|---|---|
| `type` | string | Lowercase provider identifier. |
| `region` | string | Provider region; use `default` when the provider has no regions. |
| `zones` | JSON list of strings | Available zones; use `["default"]` when the provider has no zones. An empty list is allowed. |
| `instanceClassKind` | string | Kind of the provider InstanceClass. |
| `instanceClassAPIVersion` | string | Served InstanceClass version that node-controller must use. |
| `<type>` | JSON object | Provider-owned data exposed to templates as `.provider`; the object may be empty. |

The registration must enable at least one node engine: CAPI or MCM.

An empty `zones` list is a state, not a broken registration: discovery may not have found the
zones yet, which is how OpenStack in hybrid mode publishes them until the cloud-data hook has run.
node-controller skips the NodeGroups that need a zone and keeps serving every other path, so a
provider must publish the empty list rather than an invented zone name. Zone names reach the cloud
API, and a made-up one renames the MachineClass and recreates the nodes of the group.

## CAPI fields

A CAPI provider publishes all of these fields together:

| Key | Type |
|---|---|
| `capiClusterName` | string |
| `capiClusterKind` | string |
| `capiClusterAPIVersion` | group/version |
| `capiMachineTemplateKind` | string |
| `capiMachineTemplateAPIVersion` | group/version |

The API versions must include both the API group and version. The files placed in
`d8-cloud-provider-<type>-capi` are described by the cluster-template and machine-template
contracts in this directory.

The legacy `capiClusterAPIGroup` and `capiMachineTemplateAPIGroup` keys are not part of the
contract: the group is derived from the corresponding API version.

## MCM fields

An MCM provider publishes a non-empty `machineClassKind`. A provider that does not support MCM
may omit the key or publish an empty value. A provider that supports both engines publishes both
the CAPI group and a non-empty `machineClassKind`.

`sshPublicKey` is an optional addition used only by providers that need it.

`capiMachineDeploymentSpecPatch` is deprecated: OpenStack, the last in-tree provider to publish
it, declares `failureDomain` in its machine template instead. The field will be dropped once no
out-of-tree provider relies on it, so do not add it to a new provider.

## Validation and versions

node-controller enforces this contract from Deckhouse 1.78: `type`, `region`, `instanceClassKind`,
`instanceClassAPIVersion` and the `<type>` subtree must all be present, the CAPI group must be
published whole or not at all, and a registration that enables neither engine is rejected. Before
1.78 a partial registration was accepted and produced half-rendered resources.

An out-of-tree provider that publishes less than this must be updated before the cluster is
upgraded to 1.78, otherwise the provider resources stop being rendered and the NodeGroups that
depend on them report the error.
