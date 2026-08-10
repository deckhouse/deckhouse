---
title: Как мигрировать группы узлов на Cluster API (CAPI)?
subsystems:
  - cluster_infrastructure
lang: ru
---

Deckhouse Kubernetes Platform переводит управление узлами типа CloudEphemeral с Machine Controller Manager (MCM) на Cluster API (CAPI).

На данный момент миграция с MCM на CAPI поддерживается для следующих облачных провайдеров:

- [Yandex Cloud](/modules/cloud-provider-yandex/);
- [OpenStack](/modules/cloud-provider-openstack/).

После появления поддержки CAPI существующие группы узлов типа [CloudEphemeral](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#добавление-cloudephemeral-узлов-в-облачном-кластере) продолжают использовать MCM (`status.engine: MCM`). Новые группы узлов по умолчанию создаются с использованием CAPI (`status.engine: CAPI`).

Проверить, какой механизм управления используется для группы узлов можно с помощью команды:

```shell
d8 k get nodegroup -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
```

Чтобы перевести группу узлов с MCM на CAPI:

1. Создайте новую [NodeGroup](/modules/node-manager/cr.html#nodegroup) типа [CloudEphemeral](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#добавление-cloudephemeral-узлов-в-облачном-кластере) с требуемой конфигурацией. Не указывайте аннотацию `node.deckhouse.io/use-mcm` — иначе группа останется на MCM.
1. Убедитесь, что новая группа создана под управлением CAPI ([`status.engine: CAPI`](/modules/node-manager/cr.html#nodegroup-v1-status-engine)):

   ```shell
   d8 k get nodegroup <NODE_GROUP_NAME> -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
   ```

1. Дождитесь, пока узлы новой группы перейдут в состояние `Ready`:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<NODE_GROUP_NAME>
   ```

1. Перенесите рабочие нагрузки со старой группы на новую.

   Например, обновите у приложений [`nodeSelector`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) и [`tolerations`](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) так, чтобы поды планировались на узлы новой группы, или используйте другие механизмы размещения, принятые в вашей инфраструктуре. Подробнее о выделении узлов под нагрузки — в разделе [«Выделение узлов под специфические нагрузки»](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#выделение-узлов-под-специфические-нагрузки).

1. Убедитесь, что перенесённые рабочие нагрузки успешно запущены на узлах новой группы и на узлах старой группы не осталось нужных подов.
1. Удалите старую NodeGroup.

{% alert level="warning" %}
При удалении NodeGroup удаляются все входящие в неё узлы. Перед удалением старой группы убедитесь, что необходимые рабочие нагрузки перенесены на узлы новой группы и работают корректно.
{% endalert %}

При необходимости можно принудительно создать группу узлов под управлением MCM. Для этого до создания NodeGroup (или перед её пересозданием) задайте аннотацию `node.deckhouse.io/use-mcm`:

```shell
d8 k annotate nodegroup <NODE_GROUP_NAME> node.deckhouse.io/use-mcm="true"
```

Например:

```shell
d8 k annotate nodegroup worker node.deckhouse.io/use-mcm="true"
```

{% alert level="warning" %}
Аннотация `node.deckhouse.io/use-mcm` — временный обходной путь, использовать её не рекомендуется. Предпочтительна миграция групп узлов на CAPI.
{% endalert %}
