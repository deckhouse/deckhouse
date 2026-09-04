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

## Почему появился StorageClass для storage endpoint из другой локации?

StorageClass'ы формируются из storage endpoints, которые `cloud-data-discoverer` получает через эндпоинт `/restmachine/cloudapi/sep/listAvailableSepAndPools`. Этот эндпоинт ограничивает выдачу аккаунтом и не сообщает локацию storage endpoint, поэтому дискаверер не может отфильтровать результат по параметру `location` из `DynamixClusterConfiguration`.

Если аккаунт присутствует более чем в одной локации, для storage endpoints из других локаций тоже будут созданы StorageClass'ы, а `PersistentVolumeClaim` на таком классе не поднимется. Используйте только те StorageClass'ы, которые относятся к локации, указанной в `DynamixClusterConfiguration`.
