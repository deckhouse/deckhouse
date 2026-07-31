---
title: "Cloud provider — Basis Dynamix: FAQ"
---

## Как настроить LoadBalancer?

Для настройки Service типа LoadBalancer добавьте в манифест Service следующие аннотации:

```yaml
metadata:
  annotations:
    dynamix.cpi.flant.com/internal-network-name: <internal_name>
    dynamix.cpi.flant.com/external-network-name: <external_name>
```

Обе аннотации обязательны:

- `dynamix.cpi.flant.com/internal-network-name` — имя внутренней сети в Basis Dynamix;
- `dynamix.cpi.flant.com/external-network-name` — имя внешней сети в Basis Dynamix.

Термины «внутренняя сеть» и «внешняя сеть» используются в контексте Basis Dynamix. Внешняя сеть не обязательно должна быть публичной и может использовать серые IP-адреса.

Если одна из аннотаций не указана, cloud-controller-manager завершит обработку Service с ошибкой.
