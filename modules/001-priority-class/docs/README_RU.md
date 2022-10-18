---
title: "Модуль priority-class"
---

Модуль создает в кластере набор [priority class'ов](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass) и проставляет их компонентам установленным Deckhouse и приложениям в кластере.

[Priority Class](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption) — это функционал планировщика (scheduler'а), который позволяет учитывать приоритет Pod'а (из его принадлежности к классу) при планировании.

К примеру, при выкате в кластер Pod'ов с `priorityClassName: production-low`, если в кластере не будет доступных ресурсов для данного Pod'а, то Kubernetes начнет вытеснять Pod'ы с наименьшим приоритетом в кластере.
Т.е. сначала будут выгнаны все Pod'ы с `priorityClassName: develop`, потом — с `cluster-low`, и так далее.

При выставлении priority class очень важно понимать, к какому типу относится приложение и в каком окружении оно будет работать. Любой установленный `priorityClassName` не уменьшит приоритета Pod'а, т.к. если `priority-class` у Pod'а не установлен, планировщик считает его самым низким — `develop`.
