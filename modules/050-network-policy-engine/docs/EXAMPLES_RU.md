---
title: "Модуль network-policy-engine: примеры"
---

## Примеры network policies

### Запретить обращаться снаружи к подам внутри namespace, но разрешить им обращаться внутри namespace и к внешним ресурсам

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

Больше примеров можно посмотреть [в этом репозитории](https://github.com/ahmetb/kubernetes-network-policy-recipes).
