---
title: "Миграция container runtime на containerd v2"
permalink: ru/admin/configuration/platform-scaling/node/migrating.html
lang: ru
---

Вы можете настроить containerd v2 как основной container runtime на уровне всего кластера или для отдельных групп узлов. Этот вариант позволяет использовать cgroups v2, обеспечивает лучшую безопасность и более гибкое управление ресурсами.

## Требования

Миграция на containerd v2 возможна при выполнении следующих условий:

- Узлы соответствуют требованиям, описанным [в общих параметрах кластера](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
- На сервере нет кастомных конфигураций в `/etc/containerd/conf.d` ([пример кастомной конфигурации](/modules/node-manager/faq.html#как-использовать-containerd-с-поддержкой-nvidia-gpu)).

При несоответствии одному из требований, описанных [в общих параметрах кластера](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri), Deckhouse Kubernetes Platform добавляет на узел лейбл `node.deckhouse.io/containerd-v2-unsupported`. Если  на узле есть кастомные конфигурации  в `/etc/containerd/conf.d`, на него добавляется лейбл `node.deckhouse.io/containerd-config`.

При наличии одного из этих лейблов cмена параметра [`spec.cri.type`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) для группы узлов будет недоступна. Узлы, которые не подходят под условия миграции можно посмотреть с помощью команд:

```shell
d8 k get node -l node.deckhouse.io/containerd-v2-unsupported
d8 k get node -l node.deckhouse.io/containerd-config
```

Также администратор может проверить конкретный узел на соответствие требованиям с помощью команд:

```shell
uname -r | cut -d- -f1
stat -f -c %T /sys/fs/cgroup
systemctl --version | awk 'NR==1{print $2}'
modprobe -qn erofs && echo "TRUE" || echo "FALSE"
ls -l /etc/containerd/conf.d
```

## Как включить containerd v2

Включение containerd v2 возможно двумя способами:

1. **Для всего кластера**. Укажите значение `ContainerdV2` в параметре [`defaultCRI`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri) ресурса ClusterConfiguration. Это значение будет применяться ко всем [NodeGroup](/modules/node-manager/cr.html#nodegroup), в которых явно не указан [`spec.cri.type`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type).

   Пример:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterConfiguration
   ...
   defaultCRI: ContainerdV2
   ```

1. **Для конкретной группы узлов**. Укажите `ContainerdV2` в параметре [`spec.cri.type`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) в объекте [NodeGroup](/modules/node-manager/cr.html#nodegroup).

   Пример:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     cri:
       type: ContainerdV2
   ```

При переходе на containerd v2 Deckhouse Kubernetes Platform начнет поочерёдное обновление узлов. Если в группе узлов установлен параметр [spec.disruptions.approvalMode](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) в `Manual`, то для обновления **каждого** узла в такой группе на узел нужно будет установить аннотацию `update.node.deckhouse.io/disruption-approved=`.

Пример команды для добавления аннотации на узел:

```shell
d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
```

Во время миграции будет выполнен drain в соответствии с настройками [spec.disruptions.automatic.drainBeforeApproval](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-drainbeforeapproval).

{% alert level="info" %}
При [определенных условиях](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-drainbeforeapproval) процесс может не произойти, как описано в документации настройки. Директория `/var/lib/containerd` будет очищена, что приведет к повторному скачиванию образов подов, и узел перезагрузится.
{% endalert %}
