---
title: "Модуль user-authn: FAQ"
---

## Как отключить аутентификацию для списка доверенных адресов?

Использовать аннотации:
```yaml
nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
nginx.ingress.kubernetes.io/satisfy: "any"
```

[Подробнее](usage.html#настройка-ограничений-на-основе-cidr)...