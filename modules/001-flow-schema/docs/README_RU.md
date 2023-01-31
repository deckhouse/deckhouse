---
title: "Модуль flow-schema"
---

Этот модуль применяет [FlowSchema and PriorityLevelConfiguration](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/) для предотвращения перегрузки API.

`FlowSchema` устанавливает `PriorityLevel` для `list` запросов от всех сервис-аккаунтов в неймспейсах Deckhouse (у которых установлен label `heritage: deckhouse`) к:
* `v1` apigroup (Pod'ы, Secret'ы, ConfigMap'ы,  ноды, и т.д.). Это помогает в случае большого количества основных ресурсов в кластере (например, Secret'ов ил Pod'ов).
* `deckhouse.io` apigroup (кастомные ресурсы Deckhouse). Это помогает в случае большого количества различных кастомных ресурсов Deckhouse в кластере.
* `cilium.io` apigroup (кастомные ресурсы cilium). Это помогает в случае большого количества политик cilium в кластере.

Все запросы к API, соответствующие `FlowSchema` помещаются в одну очередь.
