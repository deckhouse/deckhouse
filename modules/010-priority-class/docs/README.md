---
title: "Модуль priority-class"
---

Модуль создает в кластере набор [priority class'ов](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass) и проставляет их компонентам установленным Deckhouse и приложениям в кластере.

[Priority Class](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption) — это функционал scheduler'а, который позволяет учитывать приоритет пода (из его принадлежности к классу) при шедулинге.

К примеру, при выкате в кластер подов с `priorityClassName: production-low`, если в кластере не будет доступных ресурсов для данного пода, то kubernetes начнет evict'ить поды с наименьшим приоритетом в кластере.
Т.е. сначала будут выгнаны все поды с `priorityClassName: develop`, потом с `cluster-low` и так далее.

При выставлении priority class очень важно понимать к какому типу относится приложение и в каком окружении оно будет работать. Любой установленный `priorityClassName` не уменьшит приоритета пода, т.к. если `priority-class` у пода не установлен, шедулер считает его самым низким — `develop`.

