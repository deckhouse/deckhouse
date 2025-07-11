---
title: "The neuvector module: usage examples"
---

## Security Rules

```yaml
target:
  policymode: Protect  # Discover, Monitor, Protect
  selector:
    name: group-name
    criteria:
      - key: service
        value: service-name
        op: "="
      - key: domain
        value: namespace
        op: "="
```

- `Protect`: Learn and log all activities without enforcement.
- `Monitor`: Log policy violations but allow traffic.
- `Discover`: Actively block policy violations.

## Network Rules

```yaml
ingress:
  - action: allow
    name: allow-web-traffic
    selector:
      name: frontend-group
      criteria:
        - key: app
          value: frontend
          op: "="
    ports: "tcp/80,tcp/443"
    applications: ["HTTP", "SSL"]
    priority: 0
```

## Process Rules

```yaml
process:
  - action: allow
    name: nginx-process
    path: /usr/sbin/nginx
    allow_update: false
```

## File Access Rules

```yaml
file:
  - behavior: block_access
    filter: /etc/shadow
    recursive: false
    app: ["nginx"]
```

## Process Profiles

```yaml
process_profile:
  baseline: zero-drift
  mode: Protect
```

## Policy Organization

1. Use NvGroupDefinition for reusable groups:

   ```yaml
   # Define once.
   apiVersion: neuvector.com/v1
   kind: NvGroupDefinition
   metadata:
     name: web-tier
   spec:
     selector:
       name: nv.web-tier.production
   ```

1. Leverage namespace-scoped rules for application policies:

   ```yaml
   # Application-specific in namespace.
   apiVersion: neuvector.com/v1
   kind: NvSecurityRule
   metadata:
     name: app-security
     namespace: production
   ```

1. Use cluster-scoped rules for global policies:

   ```yaml
   # Global baseline security.
   apiVersion: neuvector.com/v1
   kind: NvClusterSecurityRule
   metadata:
     name: baseline-security
   ```

## Migration Strategy

```bash
# Export from staging.
kubectl get nvsecurityrule -o yaml > staging-policies.yaml

# Modify for production.
target:
  policymode: Protect  # Was: Monitor

# Apply to production.
kubectl apply -f production-policies.yaml
```

## RBAC Integration

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: neuvector-policy-manager
rules:
- apiGroups: ["neuvector.com"]
  resources: ["nvsecurityrules", "nvclustersecurityrules"]
  verbs: ["get", "list", "create", "update", "delete"]
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: dev-team-policies
  namespace: development
subjects:
- kind: User
  name: dev-team-lead
roleRef:
  kind: ClusterRole
  name: neuvector-policy-manager
```

## GitOps Workflow

```yaml
# .github/workflows/security-policies.yml
name: Deploy Security Policies
on:
  push:
    paths: ['policies/**']
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Deploy policies
        run: kubectl apply -f policies/
```

## CI/CD Pipeline Integration

```bash
# !/bin/bash
# validate-policies.sh
set -e

echo "Validating NeuVector policies..."
for policy in policies/*.yaml; do
  kubectl apply --dry-run=client -f "$policy"
done

echo "Applying policies to staging..."
kubectl apply -f policies/ --namespace=staging

echo "Running security tests..."
./run-security-tests.sh

echo "Promoting to production..."
kubectl apply -f policies/ --namespace=production
```
