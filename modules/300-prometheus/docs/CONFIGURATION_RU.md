---
title: "Prometheus-мониторинг: настройки"
type:
  - instruction
search: prometheus
---

Модуль не требует обязательной конфигурации (все работает из коробки).

<!-- SCHEMA -->

## Аутентификация

По умолчанию используется модуль [user-authn](/products/kubernetes-platform/documentation/v1/modules/user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем и пользователем `admin`.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values prometheus -o json | jq '.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-monitoring delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.

## Примечание

* `retentionSize` для `main` и `longterm` **рассчитывается автоматически, возможности задать значение нет!**
  * Алгоритм расчета:
    * `pvc_size * 0.85` — если PVC существует;
    * `10 GiB` — если PVC нет и StorageClass поддерживает ресайз;
    * `25 GiB` — если PVC нет и StorageClass не поддерживает ресайз.
  * Если используется `local-storage` и требуется изменить `retentionSize`, необходимо вручную изменить размер PV и PVC в нужную сторону. **Внимание!** Для расчета берется значение из `.status.capacity.storage` PVC, поскольку оно отражает реальный размер PV в случае ручного ресайза.
* `40 GiB` — размер PersistentVolumeClaim создаваемого по умолчанию.
* Размер дисков Prometheus можно изменить стандартным для Kubernetes способом (если в StorageClass это разрешено), отредактировав в PersistentVolumeClaim поле `.spec.resources.requests.storage`.
