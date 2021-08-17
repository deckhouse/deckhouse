---
title: "Модуль descheduler"
---

## Описание
### Что это такое

Шедулинг в Kubernetes — это процесс, который привязывает Pod'ы в Pending к узлам. Он выполняется компонентом kube-scheduler. Kube-scheduler на основе политик Pod'а и состояния узла определяет, на какой узел привязать Pod. Решение принимается в момент, когда появился Pod в статусе Pending. Поскольку Кubernetes-кластер — очень динамичный и его состояние меняется с течением времени, по разным причинам может появиться потребность в переносе уже запущенного Pod'а на другой узел:
* Некоторые узлы кластера недогружены или перегружены.
* Первоначальные условия при шедулинге уже не соответствуют действительности (добавлены/удалены taint/labels, pod/node affinity).
* Часть узлов удалены из кластера и Pod'ы с них переехали на другие узлы.
* Новые узлы добавлены в кластер.

Descheduler находит Pod'ы по политикам и вытесняет "лишние" Pod'ы. Тогда kube-scheduler двигает эти Pod'ы по новым условиям.

### Как это работает

Данный модуль добавляет в кластер Deployment с [descheduler](https://github.com/kubernetes-incubator/descheduler), который выполняется раз в 15 минут, находит по политикам из [config-map](templates/config-map.yaml) и вытесняет найденные Pod'ы.

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

Данная стратегия следит за тем, чтобы на одном узле не было запущенно более одного Pod'а одного контроллера (RS, RC, Deploy, Job). Если таких Pod'ов два на одном узле, descheduler убивает один Pod.

К примеру, у нас есть 3 узла (один из них более нагружен), и мы хотим выкатить 6 реплик приложения. Так как один из узлов перегружен, то scheduler привяжет к нагруженному узлу 0 или 1 Pod. Остальные реплики поедут на другие узлы, и в таком случае descheduler будет каждые 15 минут прибивать "лишние" Pod'ы на ненагруженных узлах и надеяться, что scheduler привяжет их к этому нагруженному узлу.

#### LowNodeUtilization

Данная стратегия находит нагруженные и не нагруженные узлы в кластере по cpu/memory/Pod'ам (в процентах) и, при наличии и тех и других, вытесняет Pod'ы с нагруженных узлов. Данная стратегия учитывает не реально потребленные ресурсы на узле, а requests у Pod'ов.
Пороги, по которым узел определяется как малонагруженный или перегруженный, в настоящий момент предопределены и их нельзя изменять:
* Параметры определения малонагруженных узлов:
  * cpu — 40%
  * memory — 50%
  * Pod'ы — 40%
* Параметры определения перегруженных узлов:
  * cpu — 80%
  * memory — 90%
  * Pod'ы — 80%

#### HighNodeUtilization

Данная стратегия находит узлы, которые недостаточно используются, и удаляет модули в надежде, что эти модули будут компактно распределены по меньшему количеству узлов. Эта стратегия должна использоваться со стратегией планировщика `MostRequestedPriority`
Пороги, по которым узел определяется как малонагруженный в настоящий момент предопределены и их нельзя изменять:
* Параметры определения малонагруженных узлов:
  * cpu — 50%
  * memory — 50%

#### RemovePodsViolatingInterPodAntiAffinity

Данная стратегия следит за тем, чтобы все "нарушители" anti-affinity были удалены. В какой ситуации может быть нарушен InterPodAntiAffinity нам самим придумать не удалось, а в официальной документации по descheduler написано что-то недостаточно убедительное:
> This strategy makes sure that Pods violating interpod anti-affinity are removed from nodes. For example, if there is podA on node and podB and podC (running on same node) have anti-affinity rules which prohibit them to run on the same node, then podA will be evicted from the node so that podB and podC could run. This issue could happen, when the anti-affinity rules for Pods B, C are created when they are already running on node.

#### RemovePodsViolatingNodeAffinity

Данная стратегия отвечает за кейс, когда Pod был привязан к узлу по условиям (`requiredDuringSchedulingIgnoredDuringExecution`), но потом узел перестал им удовлетворять. Тогда descheduler увидит это и сделает все, что бы Pod переехал туда, где он будет удовлетворять условиям.

#### RemovePodsViolatingNodeTaints
Эта стратегия гарантирует, что Pod'ы, нарушающие NoSchedule на узлах, будут удалены. Например, есть Pod, имеющий toleration и запущенный на узле с соответствующим taint. Если taint на узле будет изменён или удален, Pod будет вытеснен с узла.

#### RemovePodsViolatingTopologySpreadConstraint
Эта стратегия гарантирует, что Pod'ы, нарушающие [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/), будут вытеснены с узлов.

#### RemovePodsHavingTooManyRestarts
Эта стратегия гарантирует, что Pod'ы, имеющие больше 100 перезапусков контейнеров (включая init-контейнеры), будут удалены с узлов.

#### PodLifeTime
Эта стратегия гарантирует, что Pod'ы в состоянии Pending старше 24 часов, будут удалены с узлов.

### Известные особенности

* Критичные Pod'ы (с priorityClassName `system-cluster-critical` или `system-node-critical`) не вытесняются.
* При вытеснении Pod'ов с нагруженного узла учитывается priorityClass.
* Pod'ы без контроллера или с контроллером DaemonSet не вытесняются.
* Pod'ы с local storage не вытесняются.
* Best-effort Pod'ы вытесняются раньше, чем Burstable и Guaranteed.
* Descheduler использует Evict API и поэтому учитывается [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/) и если он нарушает его условия, то он не вытесняет Pod.

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
