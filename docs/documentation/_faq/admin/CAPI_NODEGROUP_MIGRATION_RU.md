---
title: Как мигрировать группы узлов на Cluster API (CAPI)?
subsystems:
  - cluster_infrastructure
lang: ru
---

Deckhouse Kubernetes Platform переводит управление узлами типа CloudEphemeral с Machine Controller Manager (MCM) на Cluster API (CAPI).

На данный момент CAPI поддерживается для следующих облачных провайдеров:

- [Yandex Cloud](/modules/cloud-provider-yandex/);
- [OpenStack](/modules/cloud-provider-openstack/).

После появления поддержки CAPI существующие группы узлов типа [CloudEphemeral](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#добавление-cloudephemeral-узлов-в-облачном-кластере) продолжают использовать MCM (`status.engine: MCM`). Новые группы узлов по умолчанию создаются с использованием CAPI (`status.engine: CAPI`).

Чтобы проверить, какой механизм управления используется для группы узлов, выполните команду:

```shell
d8 k get nodegroup -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
```

## Как выполнить миграцию

Чтобы перевести группу узлов на CAPI, пересоздайте ресурс [NodeGroup](/modules/node-manager/cr.html#nodegroup): удалите существующий ресурс и создайте его заново с той же конфигурацией. После этого новые CloudEphemeral-узлы будут создаваться и управляться через CAPI.

{% alert level="warning" %}
Пересоздание NodeGroup приводит к пересозданию всех узлов этой группы. Планируйте миграцию заранее и при необходимости выполняйте её в окно обслуживания.
{% endalert %}

## Принудительное создание NodeGroup с MCM

При необходимости можно принудительно создать группу узлов под управлением MCM. Для этого до создания NodeGroup (или перед её пересозданием) задайте аннотацию `node.deckhouse.io/use-mcm`:

```shell
d8 k annotate nodegroup <NODE_GROUP_NAME> node.deckhouse.io/use-mcm=""
```

Например:

```shell
d8 k annotate nodegroup worker node.deckhouse.io/use-mcm=""
```

{% alert level="warning" %}
Аннотация `node.deckhouse.io/use-mcm` — временный обходной путь, использовать её не рекомендуется. Предпочтительна миграция групп узлов на CAPI.
{% endalert %}
