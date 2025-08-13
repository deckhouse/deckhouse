---
title: "Миграция container runtime на containerd v2"
permalink: ru/admin/configuration/platform-scaling/node/migrating.html
lang: ru
---

Вы можете настроить containerd v2 как основной container runtime на уровне всего кластера или для отдельных групп узлов. Этот вариант позволяет использовать cgroups v2, обеспечивает лучшую безопасность и более гибкое управление ресурсами.

## Требования

Миграция на containerd v2 возможна при выполнении следующих условий:

- Узлы соответствуют требованиям, описанным [в общих параметрах кластера](/installing/configuration.html#clusterconfiguration-defaultcri).
- На сервере нет кастомных конфигураций в `/etc/containerd/conf.d` ([пример кастомной конфигурации](/modules/node-manager/faq.html#как-использовать-containerd-с-поддержкой-nvidia-gpu)).

## Как включить containerd v2

Включение containerd v2 возможно двумя способами:

1. **Для всего кластера**. Укажите значение `ContainerdV2` в параметре [`defaultCRI`](/installing/configuration.html#clusterconfiguration-defaultcri) ресурса ClusterConfiguration. Это значение будет применяться ко всем [NodeGroup](/modules/node-manager/cr.html#nodegroup), в которых явно не указан [`spec.cri.type`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type).

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

При переходе на containerd v2:

- Очищается каталог `/var/lib/containerd`, в котором containerd хранил данные.
- containerd v2 использует отдельную директорию конфигурации: `/etc/containerd/conf2.d` вместо `/etc/containerd/conf.d`.

Это значит, что при включении containerd v2 все предыдущие конфигурации containerd игнорируются, а узел начинает использовать изолированную структуру настроек и каталога данных.
