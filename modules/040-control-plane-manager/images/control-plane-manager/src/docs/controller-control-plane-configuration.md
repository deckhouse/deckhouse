# control-plane-configuration

**Name:** `control-plane-configuration-controller`  
**Primary resource:** `ControlPlaneNode` (by node name)

## Purpose

Build and keep `ControlPlaneNode.spec` in sync with:

- control-plane node labels
- `d8-control-plane-manager-config` secret
- `d8-pki` secret

## Watched Resources

| Resource | Trigger | Mapping |
|---|---|---|
| `ControlPlaneNode` | create/update(generation)/delete | self |
| `Secret` (`d8-control-plane-manager-config`, `d8-pki`) | create/update | enqueue all control-plane and arbiter nodes |
| `Node` | label changes | enqueue `ControlPlaneNode` with same name |

## Reconciliation Logic

1. Read `Node` by request name.
2. If node is gone or no longer control-plane/arbiter: delete matching `ControlPlaneNode`.
3. Read both secrets from `kube-system`.
4. Build desired `ControlPlaneNode.spec`:
- `spec.caChecksum` from `d8-pki`
- per-component `config` and `pki` checksums from config secret
5. Create `ControlPlaneNode` if absent.
6. Patch `ControlPlaneNode.spec` only when it differs.

## Checksum Composition

All checksums are `SHA256` hex over concatenated secret values in deterministic sorted key order.

- `spec.components.<component>.checksums.config`:
- source: `d8-control-plane-manager-config`
- key set per component (missing keys are skipped):
- `etcd`: `etcd.yaml.tpl`
- `kube-apiserver`: `kube-apiserver.yaml.tpl`, `extra-file-admission-control-config.yaml`, `extra-file-audit-policy.yaml`, `extra-file-audit-webhook-config.yaml`, `extra-file-authentication-config.yaml`, `extra-file-authn-webhook-config.yaml`, `extra-file-authorization-config.yaml`, `extra-file-event-rate-limit-config.yaml`, `extra-file-oidc-ca.crt`, `extra-file-secret-encryption-config.yaml`, `extra-file-webhook-config.yaml`
- `kube-controller-manager`: `kube-controller-manager.yaml.tpl`
- `kube-scheduler`: `kube-scheduler.yaml.tpl`, `extra-file-scheduler-config.yaml`

- `spec.components.<component>.checksums.pki`:
- source: `d8-control-plane-manager-config`
- `etcd`: `encryption-algorithm`
- `kube-apiserver`: `cert-sans`, `encryption-algorithm`
- `kube-controller-manager`, `kube-scheduler`: no PKI checksum (empty)

- `spec.caChecksum`:
- source: `d8-pki`
- key set: all keys from secret
- hash input uses key values only (keys are used only for deterministic ordering)

## Logic Basis

- Membership basis: node labels (`node-role.kubernetes.io/control-plane`, `node.deckhouse.io/etcd-arbiter`).
- Desired checksums basis: deterministic checksum functions over secret data.
- Arbiter rule: only etcd + CA checksums are set.
