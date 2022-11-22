---
title: "Prometheus-мониторинг: настройки"
type:
  - instruction
search: prometheus
---

{% include module-bundle.liquid %}

Модуль не требует обязательной конфигурации (всё работает из коробки).

## Аутентификация

По умолчанию используется модуль [user-authn](/documentation/v1/modules/150-user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, то модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module values prometheus -o json | jq '.prometheus.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить секрет:

```shell
kubectl -n d8-monitoring delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.

## Параметры

<!-- SCHEMA -->

## Примечание

* `retentionSize` для `main` и `longterm` **рассчитывается автоматически, возможности задать значение нет!**
  * Алгоритм расчета:
    * `pvc_size * 0.8` — если PVC существует.
    * `10 GiB` — если PVC нет и StorageClass поддерживает ресайз.
    * `25 GiB` — если PVC нет и StorageClass не поддерживает ресайз.
  * Если используется `local-storage` и требуется изменить `retentionSize`, то необходимо вручную изменить размер PV и PVC в нужную сторону. **Внимание!** Для расчета берется значение из `.status.capacity.storage` PVC, поскольку оно отражает рельный размер PV в случае ручного ресайза.
* Размер дисков prometheus можно изменить стандартным для kubernetes способом (если в StorageClass это разрешено), отредактировав в PersistentVolumeClaim поле `.spec.resources.requests.storage`.
