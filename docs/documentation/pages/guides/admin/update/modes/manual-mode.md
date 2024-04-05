---
title: Ручной режим обновлений
permalink: ru/update/modes/manual-mode/
lang: ru
---

1. Включите ручное подтверждение обновлений, как показано на примере ниже:

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
         mode: Manual
   ```

2. В этом режиме подтвердите каждое минорное обновление Deckhouse Kubernetes Platform (без учета patch-версий), как показано на примере подтверждения обновления на версию `v1.43.2`:

   ```shell
   kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
   ```

**Срочное ручное обновление**

Обновление без окна обновлений позволяет выполнить обновление модуля вне определенного для этого времени. Это необходимо в случае срочного обновления. 

> Применение обновлений без соблюдения определенного для этого времени может вызвать проблемы стабильности системы или конфликты с работающими приложениями. Поэтому используйте только в случае действительной необходимости.

Установите в соответствующем ресурсе [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`, как показано на примерах ниже:

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

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