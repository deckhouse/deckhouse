---
title: Как сделать так, чтобы внешний модуль обновлялся только на патч-версии?
lang: ru
---

По умолчанию, если у модуля нет собственной политики обновления, режим обновления и окна наследуются из настроек DKP.

Если для DKP задан режим `AutoPatch`, внешний модуль тоже будет автоматически получать только патч-версии в рамках текущей минорной версии. Переход на новую минорную версию потребует ручного подтверждения. В этом случае отдельно ничего настраивать не нужно.

Подробнее о режимах обновления DKP — в разделе [«Настройка обновлений»](admin/configuration/update/configuration.html#режимы-обновления).

Если нужно управлять режимом обновления модуля независимо от DKP, создайте [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy) с `update.mode: AutoPatch` и привяжите её к модулю через параметр `updatePolicy` в ModuleConfig:

1. Создайте политику обновления.

   Пример ModuleUpdatePolicy:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModuleUpdatePolicy
   metadata:
     name: my-update-policy
   spec:
     releaseChannel: Stable
     update:
       mode: AutoPatch
   ```

   Убедитесь, что политика создана:

   ```shell
   d8 k get mup my-update-policy
   ```

1. Свяжите политику с модулем.

   Укажите имя политики в параметре [updatePolicy](reference/api/cr.html#moduleconfig-v1alpha1-spec-updatepolicy) ModuleConfig модуля:

   ```shell
   d8 k edit mc module
   ```

   Пример ModuleConfig:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: module
   spec:
     enabled: true
     updatePolicy: my-update-policy
   ```

В режиме `AutoPatch` патч-версии модуля (например, с `v1.16.1` на `v1.16.2`) применяются автоматически с учётом окон обновлений, если они заданы. Для перехода на новую минорную версию (например, с `v1.16.*` на `v1.17.*`) подтвердите соответствующий [ModuleRelease](reference/api/cr.html#modulerelease):

```shell
d8 k annotate mr module-v1.17.0 modules.deckhouse.io/approved="true"
```

Или с помощью [`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/) CLI:

```shell
d8 system module approve module v1.17.0
```
