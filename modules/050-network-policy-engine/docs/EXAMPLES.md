---
title: "The network-policy-engine module: examples"
---

## Examples of network policies

### Deny external traffic to Pods in the namespace, but allow traffic to external resources from within the namespace

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: delete
spec:
  podSelector: {}
  egress:
  - {}
  ingress:
  - from:
    - podSelector: {}
  policyTypes:
  - Ingress
  - Egress
```

More examples are available in [this repository](https://github.com/ahmetb/kubernetes-network-policy-recipes).
