---
title: "Модуль descheduler"
---

Модуль запускает в кластере [descheduler](https://github.com/kubernetes-incubator/descheduler/) с набором [стратегий](#стратегии), заданных в custom resource `Descheduler`.

descheduler каждые 15 минут вытесняет Pod'ы, которые удовлетворяют включенным в custom resource `Descheduler` стратегиям. Это приводит к принудительному запуску процесса шедулинга в отношении вытесненных подов.

## Особенности работы descheduler

* При вытеснении подов с нагруженного узла учитывается класс приоритета (ознакомьтесь с модулем [priority-class](../001-priority-class/)).
* Поды с [priorityClassName](../001-priority-class/) `system-cluster-critical` или `system-node-critical` (*критичные* поды) не вытесняются.
* Поды без контроллера или с контроллером DaemonSet не вытесняются.
* Поды с local storage не вытесняются.
* Best-effort-поды вытесняются раньше, чем Burstable и Guaranteed.
* Учитывается [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/): если вытеснение пода приведет к нарушению условий PDB, то под не вытесняется.

## Стратегии

Включить, выключить, настроить стратегии можно в custom resource [`Descheduler`](cr.html).

### HighNodeUtilization

Данная стратегия находит ненагруженные узлы и вытесняет с них поды в надежде что эти поды будут компактно распределены по меньшему количеству узлов. Эта стратегия должна использоваться со стратегией планировщика `MostRequestedPriority`.

### LowNodeUtilization

Данная стратегия находит нагруженные и ненагруженные узлы в кластере по CPU/памяти/подам (в процентах) и при наличии и тех и других вытесняет поды с нагруженных узлов в надежде что поды будут запущены на ненагруженных узлах. Данная стратегия учитывает не реально потребленные ресурсы на узле, а реквесты у подов.

### PodLifeTime

Эта стратегия гарантирует, что поды в состоянии Pending старше 24 часов будут удалены с узлов.

### RemoveDuplicates

Данная стратегия следит за тем, чтобы на одном узле не было запущенно более одного пода одного контроллера (RS, RC, Deploy, Job). Если таких подов два на одном узле, descheduler убивает один под.

К примеру, у нас есть 3 узла (один из них более нагружен), и мы хотим выкатить 6 реплик приложения. Так как один из узлов перегружен, scheduler привяжет к нагруженному узлу 0 или 1 под. Остальные реплики поедут на другие узлы, и в таком случае descheduler будет каждые 15 минут прибивать «лишние» поды на ненагруженных узлах и надеяться, что scheduler привяжет их к этому нагруженному узлу.

### RemovePodsHavingTooManyRestarts

Эта стратегия гарантирует, что поды, имеющие больше 100 перезапусков контейнеров (включая init-контейнеры), будут удалены с узлов.

### RemovePodsViolatingInterPodAntiAffinity

Данная стратегия следит за тем, чтобы все «нарушители» anti-affinity были удалены. В какой ситуации может быть нарушен InterPodAntiAffinity, нам самим придумать не удалось, а в официальной документации по descheduler написано что-то недостаточно убедительное:
> This strategy makes sure that Pods violating interpod anti-affinity are removed from nodes. For example, if there is podA on node and podB and podC (running on same node) have anti-affinity rules which prohibit them to run on the same node, then podA will be evicted from the node so that podB and podC could run. This issue could happen, when the anti-affinity rules for Pods B, C are created when they are already running on node.

### RemovePodsViolatingNodeAffinity

Данная стратегия отвечает за кейс, когда под был привязан к узлу по условиям (`requiredDuringSchedulingIgnoredDuringExecution`), но потом узел перестал им удовлетворять. Тогда descheduler увидит это и сделает все, чтобы под переехал туда, где он будет удовлетворять условиям.

### RemovePodsViolatingNodeTaints

Эта стратегия гарантирует, что поды, нарушающие NoSchedule на узлах, будут удалены. Например, есть под, имеющий toleration и запущенный на узле с соответствующим taint. Если taint на узле будет изменен или удален, под будет вытеснен с узла.

### RemovePodsViolatingTopologySpreadConstraint

Эта стратегия гарантирует, что поды, нарушающие [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/), будут вытеснены с узлов.
