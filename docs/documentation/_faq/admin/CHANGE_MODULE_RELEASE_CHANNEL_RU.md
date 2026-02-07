---
title: Как сменить канал обновлений для модуля?
lang: ru
---

Модуль может быть встроенным в DKP или подключенным из источника модулей (определяется с помощью [ModuleSource](reference/api/cr.html#modulesource)). Встроенные модули имеют общий с DKP релизный цикл и обновляются вместе с DKP. **Канал обновлений встроенного модуля всегда соответствует каналу обновлений DKP.** Модуль, подключаемый из источника, имеет собственный релизный цикл, который не зависит от релизного цикла DKP. **Канал обновлений модуля, подключенного из источника, может быть изменен.** 

Далее рассматривается процесс смены канала обновлений для модуля, подключенного из источника.

По умолчанию канал обновлений для модулей наследуется от канала обновлений DKP (указывается в параметре [`releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) ModuleConfig `deckhouse`). Подробнее про каналы обновлений — в разделе [«Каналы обновлений»](architecture/module-development/versioning/#каналы-обновлений).

Для модулей из источника канал обновлений задается с помощью [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy), который затем _привязывается_ к модулю через параметр `updatePolicy` в ModuleConfig.

Чтобы сменить канал обновлений у модуля из источника, выполните следующие шаги:

1. Определите политику обновления модуля.

   Создайте [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy), в котором укажите канал обновлений в параметре `releaseChannel`.

   Пример ModuleUpdatePolicy:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModuleUpdatePolicy
   metadata:
     name: my-module-policy
   spec:
     releaseChannel: Alpha
     # При необходимости, укажите режим обновления и окна обновления.
     # update:
     #   mode: AutoPatch
     #   windows: []
   ```

   Убедитесь, что политика создана:

   ```shell
   d8 k get mup my-module-policy
   ```

   Пример ответа:

   ```console
   NAME               RELEASE CHANNEL   UPDATE MODE
   my-module-policy   Alpha             AutoPatch
   ```

1. Свяжите политику обновления с модулем.

   Укажите имя созданной политики обновления в параметре [updatePolicy](reference/api/cr.html#moduleconfig-v1alpha1-spec-updatepolicy) ModuleConfig соответствующего модуля.

   Для редактирования ModuleConfig используйте команду (укажите имя модуля):

   ```shell
   d8 k edit mc my-module
   ```

   Пример ModuleConfig:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: my-module
   spec:
     enabled: true
     # Имя ModuleUpdatePolicy
     updatePolicy: my-module-policy
   ```

При изменении канала обновлений модуля, его установленная версия изменится согласно настроенному режиму обновления.

Чтобы посмотреть текущий канал обновлений модуля и другую информацию о его состоянии в кластере, используйте соответствующий объект [Module](reference/api/cr.html#module).

Пример команды для получения информации о модуле:

```shell
d8 k get module my-module -o yaml
```

Используемая политика обновления будет указана в поле `properties.updatePolicy`, текущий канал обновлений — в поле `properties.releaseChannel`. Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: my-module
  # ...
properties:
  # ...
  releaseChannel: Alpha # Канал обновлений модуля.
  updatePolicy: my-module-policy # Политика обновлений модуля.
  version: v1.16.10  # Версия модуля.
  # ...
```
