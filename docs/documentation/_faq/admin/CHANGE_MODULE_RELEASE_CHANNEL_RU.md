---
title: Как сменить канал обновления для модуля из источника?
lang: ru
---

Смена канала обновления возможна для модулей из источника (т.к. их релизный цикл не привязан к релизному циклу DKP). Подробнее про каналы обновлений — в разделе [«Каналы обновлений»](reference/release-channels.html).

Для модулей из источника канал обновления задается через ресурс ModuleUpdatePolicy, определяющий политику обновления, который затем привязывается к модулю через поле `spec.updatePolicy` в ModuleConfig.

Чтобы сменить канал обновления для используемого в кластере модуля из источника, выполните следующие шаги:

1. Создайте или измените ресурс [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy) с нужным каналом (канал укажите в поле `spec.releaseChannel`).

   Пример манифеста ресурса:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModuleUpdatePolicy
   metadata:
     name: my-module-policy
   spec:
     releaseChannel: Alpha  # Канал обновления, который должен быть установлен для модуля.
     update:
       mode: AutoPatch
       windows: []  # Опционально: окна обновления.
   ```

1. Убедитесь, что политика создана, используя команду:

   ```shell
   d8 k get mup
   ```

   Пример ответа:

   ```console
   NAME               RELEASE CHANNEL   UPDATE MODE
   my-module-policy   Alpha             AutoPatch
   ```

1. Укажите имя созданной политики в [ModuleConfig](reference/api/cr.html#moduleconfig) модуля, для которого меняется канал:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: my-module
   spec:
     enabled: true
     updatePolicy: my-module-policy  # Имя ModuleUpdatePolicy
   ```

При изменении канала обновления модуль автоматически начнет получать версии из нового канала. Если в новом канале уже есть более новая версия, она будет установлена согласно настроенному режиму обновления.

Чтобы убедиться в том, что для модуля используется желаемый канал обновления, используйте команду:

```shell
d8 k get module my-module -o yaml
```

Используемая политика обновления указывается в поле `properties.updatePolicy`. Текущий канал обновления модуля указывается в параметре `properties.releaseChannel`. Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: my-module
  resourceVersion: "4095616"
  uid: d69fff12-54c4-4949-b82e-7d92f7ecf17a
properties:
  ...
  namespace: my-namespace
  releaseChannel: Alpha # Канал обновления модуля.
  requirements:
    deckhouse: '>= 1.71'
  source: deckhouse
  stage: General Availability
  updatePolicy: my-module-policy # Политика обновления модуля.
  version: v1.16.10
  weight: 900
  ...
```
