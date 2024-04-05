---
title: Автоматический режим обновлений
permalink: ru/update/modes/automatic-mode/
lang: ru
---

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` Deckhouse Kubernetes Platform будет каждую минуту проверять данные о релизе на канале обновлений.
<!-- каким образом?-->

{% alert %}
Patch-релизы, например, обновление на версию `1.30.2` при установленной версии `1.30.1`, устанавливаются без учета режима и окон обновления, то есть при появлении на канале обновления patch-релиза - он всегда будет установлен.
{% endalert %}

При автоматическом режиме обновления происходит следующее:

1. При появлении нового релиза DKP скачает его в кластер и создаст кастомный ресурс [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease).

2. После появления кастомного ресурса *DeckhouseRelease* в кластере DKP выполняет обновление на соответствующую версию согласно установленным [параметрам обновлений](modules/002-deckhouse/configuration.html#parameters-update). По умолчанию — автоматически, в любое время.

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

Если в автоматическом режиме окна обновлений не заданы, Deckhouse Kubernetes Platform обновится сразу, как только новый релиз станет доступен.

Чтобы включить подтверждение потенциально опасных (disruptive) обновлений, добавьте параметр `disruptionApprovalMode: Manual` в *ModuleConfig*:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
```

В В этом режиме можно подтвердить каждое минорное потенциально опасное обновление Deckhouse Kubernetes Platform (DKP) на соответствующем ресурсе [*DeckhouseRelease*](cr.html#deckhouserelease).

Пример подтверждения минорного обновления DKP на версию `v1.43.2`:

   ```shell
   kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
   ```

1. При необходимости, выполните обновление модуля без определенного времени.

   > Применение обновлений без соблюдения определенного для этого времени может вызвать проблемы стабильности системы или конфликты с работающими приложениями.

1. Установите в соответствующем ресурсе [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

   Пример команды пропуска окон обновлений для версии `v1.56.2`:

   ```shell
   kubectl annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
   ```

   Пример ресурса с установленной аннотацией пропуска окон обновлений:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: DeckhouseRelease
   metadata:
     annotations:
       release.deckhouse.io/apply-now: "true"
   ...
   ```

