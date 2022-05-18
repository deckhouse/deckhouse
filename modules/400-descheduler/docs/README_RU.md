---
title: "Модуль descheduler"
---

Модуль запускает в кластере [descheduler](https://github.com/kubernetes-incubator/descheduler/) с набором **предопределенных** [стратегий](#стратегии) в политике.

descheduler каждые 15 минут вытесняет Pod'ы, которые удовлетворяют включенным в [конфигурации модуля](configuration.html) стратегиям. Это приводит к принудительному запуску процесса шедулинга в отношении вытесненных Pod'ов.

## Особенности работы descheduler

* При вытеснении Pod'ов с нагруженного узла учитывается `priorityClass` (ознакомьтесь с модулем [priority-class](../001-priority-class/));
* Pod'ы с [priorityClassName](../001-priority-class/) `system-cluster-critical` или `system-node-critical` (*критичные* Pod'ы) не вытесняются;
* Pod'ы без контроллера или с контроллером DaemonSet не вытесняются;
* Pod'ы с local storage не вытесняются;
* Best-effort Pod'ы вытесняются раньше, чем Burstable и Guaranteed;
* Учитывается [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/): если вытеснение Pod'а приведет к нарушению условий PDB, то Pod не вытесняется.

## Стратегии

Включить или выключить использование стратегии можно в [конфигурации модуля](configuration.html).

По умолчанию **включены** следующие стратегии:
* [RemovePodsViolatingInterPodAntiAffinity](#removepodsviolatinginterpodantiaffinity)
* [RemovePodsViolatingNodeAffinity](#removepodsviolatingnodeaffinity)

### HighNodeUtilization

Данная стратегия находит узлы, которые недостаточно используются, и удаляет модули в надежде, что эти модули будут
компактно распределены по меньшему количеству узлов. Эта стратегия должна использоваться со стратегией
планировщика `MostRequestedPriority`.

Пороги, по которым узел определяется как малонагруженный в настоящий момент предопределены и их нельзя изменять:
* Параметры определения малонагруженных узлов:
  * CPU — 50%
  * memory — 50%

### LowNodeUtilization

Данная стратегия находит нагруженные и не нагруженные узлы в кластере по cpu/memory/Pod'ам (в процентах) и, при наличии и тех и других, вытесняет Pod'ы с нагруженных узлов. Данная стратегия учитывает не реально потребленные ресурсы на узле, а requests у Pod'ов.

Пороги, по которым узел определяется как малонагруженный или перегруженный, в настоящий момент предопределены и их нельзя изменять:
* Параметры определения малонагруженных узлов:
  * CPU — 40%
  * memory — 50%
  * Pod'ы — 40%
* Параметры определения перегруженных узлов:
  * CPU — 80%
  * memory — 90%
  * Pod'ы — 80%

### PodLifeTime

Эта стратегия гарантирует, что Pod'ы в состоянии Pending старше 24 часов, будут удалены с узлов.

### RemoveDuplicates

Данная стратегия следит за тем, чтобы на одном узле не было запущенно более одного Pod'а одного контроллера (RS, RC, Deploy, Job). Если таких Pod'ов два на одном узле, descheduler убивает один Pod.

К примеру, у нас есть 3 узла (один из них более нагружен), и мы хотим выкатить 6 реплик приложения. Так как один из узлов перегружен, то scheduler привяжет к нагруженному узлу 0 или 1 Pod. Остальные реплики поедут на другие узлы, и в таком случае descheduler будет каждые 15 минут прибивать "лишние" Pod'ы на ненагруженных узлах и надеяться, что scheduler привяжет их к этому нагруженному узлу.

### RemovePodsHavingTooManyRestarts

Эта стратегия гарантирует, что Pod'ы, имеющие больше 100 перезапусков контейнеров (включая init-контейнеры), будут удалены с узлов.

### RemovePodsViolatingInterPodAntiAffinity

Данная стратегия следит за тем, чтобы все "нарушители" anti-affinity были удалены. В какой ситуации может быть нарушен InterPodAntiAffinity нам самим придумать не удалось, а в официальной документации по descheduler написано что-то недостаточно убедительное:
> This strategy makes sure that Pods violating interpod anti-affinity are removed from nodes. For example, if there is podA on node and podB and podC (running on same node) have anti-affinity rules which prohibit them to run on the same node, then podA will be evicted from the node so that podB and podC could run. This issue could happen, when the anti-affinity rules for Pods B, C are created when they are already running on node.

### RemovePodsViolatingNodeAffinity

Данная стратегия отвечает за кейс, когда Pod был привязан к узлу по условиям (`requiredDuringSchedulingIgnoredDuringExecution`), но потом узел перестал им удовлетворять. Тогда descheduler увидит это и сделает все, что бы Pod переехал туда, где он будет удовлетворять условиям.

### RemovePodsViolatingNodeTaints

Эта стратегия гарантирует, что Pod'ы, нарушающие NoSchedule на узлах, будут удалены. Например, есть Pod, имеющий toleration и запущенный на узле с соответствующим taint. Если taint на узле будет изменён или удален, Pod будет вытеснен с узла.

### RemovePodsViolatingTopologySpreadConstraint

Эта стратегия гарантирует, что Pod'ы, нарушающие [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/), будут вытеснены с узлов.
