# Cluster object grants — demo

Demonstrates the multitenancy-manager grants feature: per-project availability (allow-list),
object quotas, default injection / coercion, and the read-only catalog.

> Requires the multitenancy-manager image with `coerceToDefault` (PR #20520, current `pr20520`).

## Run

On the master node:

```bash
alias k='sudo /opt/deckhouse/bin/kubectl --kubeconfig=/etc/kubernetes/admin.conf'
cd grants-demo
```

`00-setup.yaml` (project `demo` + grant + object quota) is normally already applied. To (re)create:

```bash
k apply -f 00-setup.yaml
```

Each scenario file mixes ALLOW and DENY objects; `kubectl apply` creates the allowed ones and
prints a `Forbidden` error for the denied ones — that is the demo.

## What is configured for project `demo`

| Resource              | Allowed            | Default     | Quota                         | coerceToDefault |
|-----------------------|--------------------|-------------|-------------------------------|-----------------|
| storageclasses        | `local`            | `local`     | 5Gi total, 3 PVCs             | yes             |
| clusterissuers        | `selfsigned`       | `selfsigned`| —                             | no              |
| loadbalancerclasses   | `internal`         | —           | 1 LoadBalancer service        | no              |
| clusterroles          | `d8:use:*`, `user-authz:*` | —   | —                             | no              |

Key difference: storage has `coerceToDefault: true` (the built-in DefaultStorageClass admission
pre-fills `storageClassName`), so a disallowed/omitted storage class is silently rewritten to `local`.
Issuers / LB / cluster roles have no such defaulter, so a disallowed explicit value is **rejected**.

## Discovery & protection (run, don't apply)

```bash
# Tenant's "what can I use" view (the controller-owned catalog). The AVAILABLE column is a count,
# so the table stays readable even for cluster roles (dozens of entries):
k -n demo get availableresources

# Full list of available names for one resource:
k -n demo get availableresource clusterroles -o jsonpath='{.status.availableSummary}'
# or the structured form (with the default flagged):
k -n demo get availableresource storageclasses -o yaml

# Object-quota usage vs limit:
k -n demo get grantquota objects -o yaml

# The catalog is read-only (protected webhook) — this is denied:
k -n demo delete availableresource storageclasses
```

## Cleanup

```bash
k -n demo delete pvc,svc,certificate,ingress,rolebinding --all
k delete project demo
```

## Files

- `00-setup.yaml`         — project + grant + object quota
- `10-storage.yaml`       — disk allowed / coerced (omit & disallowed class → `local`)
- `11-storage-quota.yaml` — disk size & count quota (denied)
- `20-issuer-cert.yaml`   — ClusterIssuer in a Certificate (allow / deny)
- `21-issuer-ingress.yaml`— ClusterIssuer in an Ingress annotation (allow / deny)
- `30-lb-class.yaml`      — loadBalancerClass (allow / deny)
- `31-lb-quota.yaml`      — load balancer count quota (2nd denied)
- `40-clusterrole.yaml`   — RoleBinding to a ClusterRole (allow / deny)
