---
title: "Модуль basic-auth: примеры"
---

## Пример конфигурации

```yaml
basicAuthEnabled: "true"
basicAuth: |
  locations:
  - location: "/"
    whitelist:
      - 1.1.1.1
    users:
      username: "password"
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```

## Пример использования

Просто добавьте подобную аннотацию к Ingress-ресурсу:

```yaml
nginx.ingress.kubernetes.io/auth-url: "http://basic-auth.kube-basic-auth.svc.cluster.local/"
```
