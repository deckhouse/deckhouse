---
title: Как применить обновление DKP или модуля, минуя окна обновлений, canary-release и ручной режим обновлений?
subsystems:
  - deckhouse
lang: ru
---

Чтобы применить обновление DKP немедленно, установите в соответствующем ресурсе [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

В этом случае будут проигнорированы окна обновления, [настройки canary-release](../user/network/canary-deployment.html) и [режим ручного обновления кластера](../admin/configuration/update/configuration.html#ручное-подтверждение-обновлений). Обновление применится сразу после установки аннотации.

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

```shell
d8 k annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
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

#### Как принудительно обновить модуль

Чтобы немедленно применить обновление конкретного модуля, установите аннотацию `modules.deckhouse.io/apply-now: "true"` в соответствующем ресурсе [ModuleRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulerelease).

В этом случае обновление модуля будет выполнено сразу после установки аннотации, даже если не выполняются требования для обновления (например, версия Kubernetes, зависимости или другие условия).

Пример ресурса с установленной аннотацией:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleRelease
metadata:
  name: console-0.9.3
  annotations:
    modules.deckhouse.io/apply-now: "true"
...
```
