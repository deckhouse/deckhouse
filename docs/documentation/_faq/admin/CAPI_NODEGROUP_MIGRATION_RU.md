---
title: Как мигрировать группы узлов на Cluster API (CAPI)?
subsystems:
  - cluster_infrastructure
lang: ru
---

Deckhouse Kubernetes Platform переводит управление узлами типа CloudEphemeral с Machine Controller Manager (MCM) на Cluster API (CAPI).

Сейчас CAPI поддерживается для следующих облачных провайдеров:

- [Yandex Cloud](/modules/cloud-provider-yandex/);
- [OpenStack](/modules/cloud-provider-openstack/).

После появления поддержки CAPI у провайдера существующие ресурсы [NodeGroup](/modules/node-manager/cr.html#nodegroup) типа CloudEphemeral остаются на MCM (`status.engine: MCM`). Новые NodeGroup по умолчанию создаются с CAPI (`status.engine: CAPI`).

Проверить, каким механизмом управляется группа узлов:

```shell
d8 k get nodegroup -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
```

## Как выполнить миграцию

Чтобы перевести группу узлов на CAPI, пересоздайте [NodeGroup](/modules/node-manager/cr.html#nodegroup): удалите существующую группу и создайте её заново с той же конфигурацией. Новые узлы будут управляться через CAPI.

{% alert level="warning" %}
Пересоздание NodeGroup приводит к пересозданию узлов этой группы. Планируйте миграцию заранее и при необходимости выполняйте её в окно обслуживания.
{% endalert %}

## Принудительное использование MCM при проблемах

Если CAPI работает некорректно, можно принудительно использовать MCM для NodeGroup — задайте аннотацию `node.deckhouse.io/use-mcm` до создания группы (или при её пересоздании):

```shell
d8 k annotate nodegroup <NODE_GROUP_NAME> node.deckhouse.io/use-mcm=""
```

{% alert level="warning" %}
Аннотация `node.deckhouse.io/use-mcm` — временный обходной путь, использовать её не рекомендуется. Предпочтительна миграция групп узлов на CAPI.
{% endalert %}
