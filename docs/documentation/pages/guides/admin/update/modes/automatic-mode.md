---
title: Автоматический режим обновлений
permalink: ru/update/automatic-mode/
lang: ru
---

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` Deckhouse Kubernetes Platform будет каждую минуту проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse Kubernetes Platform скачает его в кластер и создаст кастомный ресурс [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease).

После появления кастомного ресурса *DeckhouseRelease* в кластере Deckhouse Kubernetes Platform выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update) (по умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

{% alert %}
Patch-релизы, например, обновление на версию `1.30.2` при установленной версии `1.30.1`, устанавливаются без учета режима и окон обновления, то есть при появлении на канале обновления patch-релиза он всегда будет установлен.
{% endalert %}

**Настройка автоматического режима обновления**

Если в автоматическом режиме окна обновлений не заданы, Deckhouse Kubernetes Platform обновится сразу, как только новый релиз станет доступен.

Patch-версии (например, обновления с `1.26.1` до `1.26.2`) устанавливаются без подтверждения и без учета окон обновлений.

{% alert %}
Также можно настраивать окна disruption-обновлений узлов в custom resource [NodeGroup](../040-node-manager/cr.html#nodegroup) (параметр `disruptions.automatic.windows`).
{% endalert %}