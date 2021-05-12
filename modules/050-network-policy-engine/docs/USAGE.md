---
title: "The network-policy-engine module: usage"
---

## Examples of network policies
### Deny external traffic to pods in the namespace, but allow traffic to external resources from within the namespace

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

More examples are available [here](https://github.com/ahmetb/kubernetes-network-policy-recipes).
