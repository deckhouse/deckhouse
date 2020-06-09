---
title: "Модуль network-policy-engine"
---

Данный модуль выкатывает в namespace `d8-system` daemonset с [kube-router](https://github.com/cloudnativelabs/kube-router) только в режиме поддержки [network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) после чего в Kubernetes кластере включается полная поддержка Network Policies.

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения добавьте в CM `deckhouse`:

```yaml
data:
  networkPolicyEngineEnabled: "true"
```

### Примеры network policies

Разрешить подам обращаться к внешним ресурсам и обращение внутри namespace, но запрещает обращаться снаружи к подам в namespace:
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

Больше примеров можно посмотреть [тут](https://github.com/ahmetb/kubernetes-network-policy-recipes).
