---
title: "Модуль descheduler"
---

## Описание
### Что это такое

Шедулинг в Kubernetes — это процесс, который привязывает pending-поды на узлы. Он выполняется компонентом kube-scheduler. Kube-scheduler на основе политик пода и состояния узла определяет, на какой узел привязать под. Решение принимается в момент, когда появился под в статусе pending. Поскольку Кubernetes-кластер очень динамичный и его состояние меняется с течением времени, по разным причинам может появиться потребность в переносе уже запущенного пода на другой узел:
* Некоторые узлы кластера недогружены или перегружены.
* Первоначальные условия при шедулинге уже не соответствуют действительности (добавлены/удалены taint/labels, pod/node affinity).
* Часть узлов удалены из кластера и поды с них переехали на другие узлы.
* Новые узлы добавлены в кластер.

Descheduler находит поды по политикам и эвиктит "лишние" поды. Тогда kube-scheduler двигает эти поды по новым условиям.

### Как это работает

Данный модуль добавляет в кластер Deployment с [descheduler](https://github.com/kubernetes-incubator/descheduler), который выполняется раз в 15 минут, находит по политикам из [config-map](templates/config-map.yaml) и эвиктит найденные поды.

У descheduler есть 9 стратегий:
* RemoveDuplicates (**выключена по умолчанию**)
* LowNodeUtilization (**выключена по умолчанию**)
* HighNodeUtilization (**выключена по умолчанию**)
* RemovePodsViolatingInterPodAntiAffinity (**включена по умолчанию**)
* RemovePodsViolatingNodeAffinity (**включена по умолчанию**)
* RemovePodsViolatingNodeTaints (**выключена по умолчанию**)
* RemovePodsViolatingTopologySpreadConstraint (**выключена по умолчанию**)
* RemovePodsHavingTooManyRestarts (**выключена по умолчанию**)
* PodLifeTime (**выключена по умолчанию**)

#### RemoveDuplicates

Данная стратегия следит за тем, чтобы на одном узле не было запущенно более одного пода одного контроллера (rs, rc, deploy, job). Если таких подов 2 на одном узле, descheduler убивает один под.

К примеру, у нас есть 3 узла (один из них более нагружен), и мы хотим выкатить 6 реплик приложения. Так как один из узлов перегружен, то scheduler привяжет к нагруженному узлу 0 или 1 под. Остальные реплики поедут на другие узлы, и в таком случае descheduler будет каждые 15 минут прибивать "лишние" поды на ненагруженных узлах и надеяться, что scheduler привяжет их к этому нагруженному узлу.

#### LowNodeUtilization

Данная стратегия находит нагруженные и не нагруженные узлы в кластере по cpu/memory/pods (в процентах) и, при наличии и тех и других, эвиктит поды с нагруженных узлов. Данная стратегия учитывает не реально потребленные ресурсы на узле, а requests подов.
Пороги, по которым узел определяется как малонагруженный или перегруженный, в настоящий момент предопределены и их нельзя изменять:
* Параметры определения малонагруженных узлов:
  * cpu — 40%
  * memory — 50%
  * pods — 40%
* Параметры определения перегруженных узлов:
  * cpu — 80%
  * memory — 90%
  * pods — 80%

#### HighNodeUtilization

Данная стратегия находит узлы, которые недостаточно используются, и удаляет модули в надежде, что эти модули будут компактно распределены по меньшему количеству узлов. Эта стратегия должна использоваться со стратегией планировщика `MostRequestedPriority`
Пороги, по которым узел определяется как малонагруженный в настоящий момент предопределены и их нельзя изменять:
* Параметры определения малонагруженных узлов:
  * cpu — 50%
  * memory — 50%

#### RemovePodsViolatingInterPodAntiAffinity

Данная стратегия следит за тем, чтобы все "нарушители" anti-affinity были удалены. В какой ситуации может быть нарушен InterPodAntiAffinity нам самим придумать не удалось, а в официальной документации по descheduler написано что-то совершенно неубедительное:
> This strategy makes sure that pods violating interpod anti-affinity are removed from nodes. For example, if there is podA on node and podB and podC(running on same node) have antiaffinity rules which prohibit them to run on the same node, then podA will be evicted from the node so that podB and podC could run. This issue could happen, when the anti-affinity rules for pods B,C are created when they are already running on node.

#### RemovePodsViolatingNodeAffinity

Данная стратегия отвечает за кейс, когда под был привязан к узлу по условиям (`requiredDuringSchedulingIgnoredDuringExecution`), но потом узел перестал им удовлетворять. Тогда descheduler увидит это и сделает все, что бы под переехал туда, где он будет удовлетворять условиям.

#### RemovePodsViolatingNodeTaints
Эта стратегия гарантирует, что поды, нарушающие NoSchedule на узлах, будут удалены. Например, есть под имеющий toleration и запущенный на узле с соответствующим taint. Если taint на узле будет изменён или удален, под будет выгнан с узла.

#### RemovePodsViolatingTopologySpreadConstraint
Эта стратегия гарантирует, что поды, нарушающие [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/), будут вытеснены с узлов.

#### RemovePodsHavingTooManyRestarts
Эта стратегия гарантирует, что поды, имеющие больше 100 перезапусков контейнеров (включая init-контейнеры), будут удалены с узлов.

#### PodLifeTime
Эта стратегия гарантирует, что поды в состоянии Pending старше 24 часов, будут удалены с узлов.

### Известные особенности

* Критикал поды (с priorityClassName `system-cluster-critical` или `system-node-critical`) не эвиктятся.
* При эвикте подов с нагруженного узла учитывается priorityClass.
* Поды без контроллера или с контроллером DaemonSet не эвиктятся.
* Поды с local storage не эвиктятся.
* Best effort поды эвиктсятся раньше, чем Burstable и Guaranteed.
* Descheduler использует Evict API и поэтому учитывается [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/) и если он нарушает его условия, то он не эвиктит под.

### Пример конфигурации

```yaml
descheduler: |
  removePodsViolatingNodeAffinity: false
  removeDuplicates: true
  lowNodeUtilization: true
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```
