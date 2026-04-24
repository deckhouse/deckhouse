---
title: "Cluster admin access model"
permalink: en/admin/configuration/access/authorization/cluster-admin-access-model.html
description: "Cluster admin access model for the Deckhouse Kubernetes Platform Cluster"
---

The Deckhouse Kubernetes Platform supports the presence of multiple kubeconfig files on master nodes (this feature is implemented by the [`control-plane-manager`](/modules/control-plane-manager/) module). Understanding their purpose is important for secure cluster administration.

## Kubeconfig files on master nodes

The following kubeconfig files are located on the master nodes:

| File | Identity | Purpose |
| --- | --- | --- |
| `/etc/kubernetes/admin.conf` | `kubernetes-admin` (`kubeadm:cluster-admins` group) | Machine kubeconfig for kubeadm internals (join, renewal). With the [`user-authz`](/modules/user-authz/) module enabled, RBAC uses `user-authz:cluster-admin` plus an additional ClusterRole. With `user-authz` disabled, the group is bound to the built-in `cluster-admin` role |
| `/etc/kubernetes/super-admin.conf` | `kubernetes-super-admin` (`system:masters` group) | Break-glass emergency credential. Bypasses RBAC entirely. Restrict access to this file to trusted recovery scenarios |
| `/etc/kubernetes/controller-manager.conf` | `system:kube-controller-manager` | Used by kube-controller-manager |
| `/etc/kubernetes/scheduler.conf` | `system:kube-scheduler` | Used by kube-scheduler |

## RBAC-based admin access

Starting from Kubernetes 1.29, kubeadm generates `admin.conf` with the `kubeadm:cluster-admins` group instead of `system:masters`. This provides RBAC-controlled admin access that can be revoked by removing the `kubeadm:cluster-admins` ClusterRoleBinding(s).

If the [`user-authz`](/modules/user-authz/) module is **disabled**, Deckhouse binds the `kubeadm:cluster-admins` group to the built-in wildcard ClusterRole `cluster-admin` (same effective model as a plain kubeadm cluster without extra RBAC).

If `user-authz`is **enabled**, the group is bound to `user-authz:cluster-admin`, and a second ClusterRoleBinding adds ClusterRole `d8:control-plane-manager:admin-kubeconfig-supplement` (rules beyond the high-level role, e.g. for certificates and cluster machinery). Together they replace a single wildcard `cluster-admin` for this identity. For full unrestricted access, use `super-admin.conf`.

## Recommended admin access

If the [user-authn](/modules/user-authn/) module is enabled, use personalized OIDC-based kubeconfig obtained through the kubeconfig generator. This provides individual accountability and audit trail.

If `user-authn` is disabled, administrators can explicitly use the admin kubeconfig on a master node:

```bash
d8 k --kubeconfig=/etc/kubernetes/admin.conf <command>
```

## Root kubeconfig symlink

By default, the [`control-plane-manager`](/modules/control-plane-manager/) module creates a symlink `/root/.kube/config` → `/etc/kubernetes/admin.conf` on master nodes, allowing root to run `d8 k` without specifying `--kubeconfig`.

When the `user-authz` module is enabled, you can disable this symlink by setting [`rootKubeconfigSymlink: false`](modules/control-plane-manager/configuration.html#parameters-rootkubeconfigsymlink) in the `control-plane-manager` module configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 2
  enabled: true
  settings:
    rootKubeconfigSymlink: false
```

If `user-authz` is disabled, CPM does not apply this setting and keeps the default symlink behavior.

When the symlink is disabled (and `user-authz` is enabled), the symlink is removed when it pointed to `admin.conf`. Use personalized credentials or pass `--kubeconfig` explicitly.

## Security hardening

The CPM module automatically restricts file permissions on `admin.conf` and `super-admin.conf` to `0600` (owner read/write only) during every reconciliation cycle. This prevents unauthorized users from reading these sensitive credentials.

## Break-glass access

In emergency situations (RBAC misconfiguration, webhook failures), use `super-admin.conf`:

```bash
d8 k --kubeconfig=/etc/kubernetes/super-admin.conf <command>
```

This credential bypasses all RBAC checks. Use it only as a last resort and restrict who can read the file.
