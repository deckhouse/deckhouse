---
title: "Модуль deckhouse: примеры конфигурации"
---

## Пример конфигурации модуля

Ниже представлен простой пример конфигурации модуля:

```yaml
deckhouse: |
  logLevel: Debug
  bundle: Minimal
  releaseChannel: RockSolid
```

Также можно настроить дополнительные параметры.

## Настройка режима обновления

Если в автоматическом режиме окна обновлений не заданы, Deckhouse обновится сразу, как только новый релиз станет доступен.

Patch-версии (например, обновления с `1.26.1` до `1.26.2`) устанавливаются без подтверждения и без учета окон обновлений.

> Вы также можете настраивать окна disruption-обновлений узлов в custom resource'ах [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup) (параметр `disruptions.automatic.windows`).

### Конфигурация окон обновлений

Сконфигурировать время, когда Deckhouse будет устанавливать обновления, можно, указав следующие параметры в конфигурации модуля:

```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    windows: 
      - from: "8:00"
        to: "15:00"
      - from: "20:00"
        to: "23:00"
```

Здесь обновления будут устанавливаться каждый день с 8:00 до 15:00 и с 20:00 до 23:00.

Также можно настроить обновления в определенные дни, например, по вторникам и субботам с 13:00 до 18:30:

```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    windows: 
      - from: "13:00"
        to: "18:30"
        days:
          - Tue
          - Sat
```

### Ручное подтверждение обновлений

При необходимости возможно включить ручное подтверждение обновлений. Сделать это можно следующим образом:

```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    mode: Manual
```

В этом режиме необходимо подтверждать каждое минорное обновление Deckhouse (без учёта patch-версий).

Пример подтверждения обновления на версию `v1.26.0`:

```shell
kubectl patch DeckhouseRelease v1-26-0 --type=merge -p='{"approved": true}'
```

### Ручное подтверждение потенциально опасных (disruptive) обновлений

При необходимости возможно включить ручное подтверждение потенциально опасных (disruptive) обновлений (которые меняют значения по-умолчанию или поведение некоторых модулей). Сделать это можно следующим образом:

```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    disruptionApprovalMode: Manual
```

В этом режиме необходимо подтверждать каждое минорное потенциально опасное(disruptive) обновление Deckhouse (без учёта patch-версий) с помощью аннотации:

```shell
kubectl annotate DeckhouseRelease v1-36-0 release.deckhouse.io/disruption-approved=true
```

## Сбор информации для отладки

Мы всегда рады помочь пользователям с расследованием сложных проблем. Пожалуйста, выполните следующие шаги, чтобы мы смогли вам помочь:

1. Выполните следующую команду, чтобы собрать необходимые данные:

   ```sh
   kubectl -n d8-system exec deploy/deckhouse \
     -- deckhouse-controller collect-debug-info \
     > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

2. Отправьте получившийся архив [команде Deckhouse](https://github.com/deckhouse/deckhouse/issues/new/choose) для дальнейшего расследования.

Данные, которые будут собраны:
* состояние очереди Deckhouse
* Deckhouse values (без каких-либо конфиденциальных данных)
* список включенных модулей
* манифесты controller'ов и pod'ов manifests из всех пространств имен Deckhouse
* состояние `nodes`
* состояние `nodegroups`
* состояние `machines`
* все объекты `deckhousereleases`
* `events` из всех пространств имен
* логи Deckhouse
* логи machine controller manager
* логи cloud controller manager
* все горящие уведомления в Prometheus
* метрики terraform-state-exporter
