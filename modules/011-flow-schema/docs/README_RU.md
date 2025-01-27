---
title: "Модуль flow-schema"
---

Модуль применяет FlowSchema and PriorityLevelConfiguration для предотвращения перегрузки API.

`FlowSchema` устанавливает `PriorityLevel` для `list`-запросов от всех сервис-аккаунтов в пространствах имен Deckhouse (у которых установлен label `heritage: deckhouse`) к следующим apiGroup:
* `v1` (Pod, Secret, ConfigMap, Node и т. д.). Это помогает в случае большого количества основных ресурсов в кластере (например, Secret'ов или подов).
* `apps/v1` (DaemonSet, Deployment, StatefulSet, ReplicaSet и т. д.). Это помогает в случае развертывания большого количества приложений в кластере (например, Deployment'ов).
* `deckhouse.io` (custom resource'ы Deckhouse). Это помогает в случае большого количества различных кастомных ресурсов Deckhouse в кластере.
* `cilium.io` (custom resource'ы cilium). Это помогает в случае большого количества политик cilium в кластере.

Все запросы к API, соответствующие `FlowSchema`, помещаются в одну очередь.
