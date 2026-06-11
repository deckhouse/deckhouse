# Cluster resource grants — demo

Demonstrates the multitenancy-manager grants feature: per-project availability (allow-list),
default injection / coercion, and the read-only catalog. Object quota is delegated to Kubernetes
`ResourceQuota` and is not part of this feature.

> Requires the multitenancy-manager image from the references redesign (PR #20611, `pr20611`).

## Run

On the master node:

```bash
alias k='sudo /opt/deckhouse/bin/kubectl --kubeconfig=/etc/kubernetes/admin.conf'
cd grants-demo
```

`00-setup.yaml` (project `demo` + availability policy) is normally already applied. To (re)create:

```bash
k apply -f 00-setup.yaml
```

Each scenario file mixes ALLOW and DENY objects; `kubectl apply` creates the allowed ones and
prints a `Forbidden` error for the denied ones — that is the demo.

## What is configured for project `demo`

| Resource              | Allowed                                                         | Default      | Defaulting |
|-----------------------|----------------------------------------------------------------|--------------|------------|
| storageclasses        | `local`                                                        | `local`      | Coerce     |
| clusterissuers        | `selfsigned`                                                   | `selfsigned` | FillEmpty (Certificate) / None (Ingress annotation) |
| loadbalancerclasses   | `internal`                                                     | —            | FillEmpty  |
| clusterroles          | d8:use:role:* + user-authz access (user,priv-user,editor,admin) | —            | None       |

Key difference: the storageclasses PVC path uses `defaulting: Coerce` (the built-in DefaultStorageClass
admission pre-fills `storageClassName`), so a disallowed/omitted storage class is rewritten to `local`.
Issuers / LB / cluster roles have no such pre-filler, so a disallowed explicit value is **rejected**.

## Discovery, status & protection (run, don't apply)

```bash
# Tenant's "what can I use" view (the controller-owned catalog). AVAILABLE is a count, so the table
# stays readable even for cluster roles (dozens of entries):
k -n demo get availableclusterresources

# Full list of available names for one resource (with the default flagged):
k -n demo get availableclusterresource storageclasses -o yaml

# Definition status — which reference paths point at a resource:
k get grantableclusterresourcedefinition storageclasses -o jsonpath='{.status.references}'

# Reference status — bound to a definition, or a typo'd name:
k get grantableclusterresourcereference

# The catalog is read-only (protect webhook) — this is denied:
k -n demo delete availableclusterresource storageclasses
```

## Cleanup

```bash
k -n demo delete pvc,svc,certificate,ingress,rolebinding --all
k delete project demo
```

## Files

- `00-setup.yaml`         — project + availability policy
- `10-storage.yaml`       — disk allowed / coerced (omit & disallowed class → `local`)
- `20-issuer-cert.yaml`   — ClusterIssuer in a Certificate (allow / deny)
- `21-issuer-ingress.yaml`— ClusterIssuer in an Ingress annotation (allow / deny)
- `30-lb-class.yaml`      — loadBalancerClass (allow / deny)
- `40-clusterrole.yaml`   — RoleBinding to a ClusterRole (allow / deny)
