---
title: "Модуль descheduler"
description: "Модуль descheduler Deckhouse Kubernetes Platform. Каждые 15 минут анализирует состояние кластера и выполняет вытеснение подов, соответствующих условиям, описанным в активных стратегиях."
---

Каждые 15 минут модуль анализирует состояние кластера и выполняет вытеснение подов, соответствующих условиям, описанным в активных [стратегиях](#стратегии). Вытесненные поды вновь проходят процесс планирования с учетом текущего состояния кластера. Это позволяет перераспределить рабочие нагрузки в соответствие с выбранной стратегией.

Модуль основан на проекте [descheduler](https://github.com/kubernetes-sigs/descheduler).

## Особенности работы модуля

* Модуль может учитывать класс приоритета пода (параметр [spec.priorityClassThreshold](cr.html#descheduler-v1alpha2-spec-priorityclassthreshold)), ограничивая работу только подами, у которых класс приоритета ниже заданного порога;
* Модуль не вытесняет под в следующих случаях:
  * под находится в пространстве имен `d8-*` или `kube-system`;
  * под имеет `priorityClassName` `system-cluster-critical` или `system-node-critical`;
  * под связан с локальным хранилищем;
  * под связан с [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/);
  * вытеснение пода нарушит [Pod Disruption Budget (PDB)](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/);
  * нет доступных узлов для запуска вытесненного пода.
* Поды с классом приоритета `Best effort` вытесняются раньше, чем поды с классами `Burstable` и `Guaranteed`.

Для фильтрации подов и узлов модуль использует механизм `labelSelector` Kubernetes:

* `podLabelSelector` — ограничивает поды по меткам;
* `namespaceLabelSelector` — фильтрует поды по пространствам имен.
* `nodeLabelSelector` — выбирает узлы по меткам.

## Стратегии

### HighNodeUtilization

{% alert level="info" %}
Концентрирует нагрузку на меньшем числе узлов. Требует настройку планировщика и включение автоматического масштабирования.

Чтобы использовать `HighNodeUtilization`, необходимо явно указать профиль планировщика [high-node-utilization](../control-plane-manager/faq.html#профили-планировщика) для каждого пода (этот профиль не может быть установлен как профиль по умолчанию).
{% endalert %}

Стратегия определяет *недостаточно нагруженные узлы* и вытесняет с них поды, чтобы распределить их компактнее, на меньшем числе узлов.

**Недостаточно нагруженный узел** — узел, использование ресурсов которого меньше **всех** пороговых значений, заданных в секции параметров [strategies.highNodeUtilization.thresholds](cr.html#descheduler-v1alpha2-spec-strategies-highnodeutilization-thresholds).

Стратегия включается параметром [spec.strategies.highNodeUtilization.enabled](cr.html#descheduler-v1alpha2-spec-strategies-highnodeutilization-enabled).

{% alert level="warning" %}
В GKE нельзя настроить конфигурацию планировщика по умолчанию, но можно использовать стратегию `optimize-utilization` или развернуть второй пользовательский планировщик.
{% endalert %}

{% alert level="warning" %}
Использование ресурсов узла учитывает [extended-ресурсы](https://kubernetes.io/docs/tasks/configure-pod-container/extended-resource/) и рассчитывается на основе запросов и лимитов подов ([requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)), а не их фактического потребления. Такой подход обеспечивает согласованность с работой kube-scheduler, который использует аналогичный принцип при размещении подов на узлах. Это означает, что метрики использования ресурсов, отображаемые Kubelet (или командами вроде `kubectl top`), могут отличаться от расчетных показателей, так как Kubelet и связанные инструменты отображают данные о реальном потреблении ресурсов.
{% endalert %}

### LowNodeUtilization

{% alert level="info" %}
Более равномерно нагружает узлы.
{% endalert %}

Стратегия выявляет *недостаточно нагруженные узлы* и вытесняет поды с других, *избыточно нагруженных узлов*. Стратегия предполагает, что пересоздание вытесненных подов произойдет на недостаточно нагруженных узлах (при обычном поведении планировщика).

**Недостаточно нагруженный узел** — узел, использование ресурсов которого меньше **всех** пороговых значений, заданных в секции параметров [strategies.lowNodeUtilization.thresholds](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-thresholds).

**Избыточно нагруженный узел** — узел, использование ресурсов которого больше **хотя бы одного** из пороговых значений, заданных в секции параметров [strategies.lowNodeUtilization.targetThresholds](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-targetthresholds).

Узлы с использованием ресурсов в диапазоне между `thresholds` и `targetThresholds` считаются оптимально используемыми. Поды на таких узлах вытесняться не будут.

Стратегия включается параметром [spec.strategies.lowNodeUtilization.enabled](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-enabled).

{% alert level="warning" %}
Использование ресурсов узла учитывает [extended-ресурсы](https://kubernetes.io/docs/tasks/configure-pod-container/extended-resource/) и рассчитывается на основе запросов и лимитов подов ([requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)), а не их фактического потребления. Такой подход обеспечивает согласованность с работой kube-scheduler, который использует аналогичный принцип при размещении подов на узлах. Это означает, что метрики использования ресурсов, отображаемые Kubelet (или командами вроде `kubectl top`), могут отличаться от расчетных показателей, так как Kubelet и связанные инструменты отображают данные о реальном потреблении ресурсов.
{% endalert %}

### RemoveDuplicates

{% alert level="info" %}
Предотвращает запуск нескольких подов одного контроллера (ReplicaSet, ReplicationController, StatefulSet) или заданий (Job) на одном узле.
{% endalert %}

Стратегия следит за тем, чтобы на одном узле не находилось больше одного пода ReplicaSet, ReplicationController, StatefulSet или подов одного задания (Job). Если таких подов два или больше, модуль вытесняет лишние поды, чтобы они лучше распределились по кластеру.

Описанная ситуация может возникнуть, если некоторые узлы кластеры вышли из строя по каким-либо причинам, и поды с них были перемещены на другие узлы. Как только вышедшие из строя узлы снова станут доступны для приема нагрузки, эту стратегию можно будет использовать для выселения дублирующих подов с других узлов.

Стратегия включается параметром [strategies.removeDuplicates.enabled](cr.html#descheduler-v1alpha2-spec-strategies-removeduplicates-enabled).

### RemovePodsViolatingInterPodAntiAffinity

{% alert level="info" %}
Вытесняет поды, нарушающие [правила inter-pod affinity и anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity).
{% endalert %}

Стратегия гарантирует, что поды, нарушающие [правила inter-pod affinity и anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity), будут удалены с узлов.

Например, если на узле находится **Под1**, а также **Под2** и **Под3**, имеющие правила anti-affinity, которые запрещают им работать на одном узле с подом **Под1**, то **Под1** будет вытеснен с узла, чтобы **Под2** и **Под3** смогли работать. Такая ситуация может возникнуть, когда правила inter-pod affinity для **Под2** и **Под3** создаются когда поды уже запущены на узле.

Стратегия включается параметром [strategies.removePodsViolatingInterPodAntiAffinity.enabled](cr.html#descheduler-v1alpha2-spec-strategies-removepodsviolatinginterpodantiaffinity-enabled).

### RemovePodsViolatingNodeAffinity

{% alert level="info" %}
Вытесняет поды, нарушающие [правила node affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity).
{% endalert %}

Стратегия гарантирует, что все поды, которые нарушают [правила node affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity), в конечном счете будут удалены с узлов.

По сути, в зависимости от настроек параметра [strategies.removePodsViolatingNodeAffinity.nodeAffinityType](cr.html#descheduler-v1alpha2-spec-strategies-removepodsviolatingnodeaffinity-nodeaffinitytype),  
стратегия превращает правило `requiredDuringSchedulingIgnoredDuringExecution` node affinity пода в правило `requiredDuringSchedulingRequiredDuringExecution`, а правило `preferredDuringSchedulingIgnoredDuringExecution` в правило `preferredDuringSchedulingPreferredDuringExecution`.

Пример для `nodeAffinityType: requiredDuringSchedulingIgnoredDuringExecution`. Есть под, который был назначен на узел, соответствующий правилу `requiredDuringSchedulingIgnoredDuringExecution` node affinity на момент размещения. Если со временем этот узел перестанет удовлетворять правилу node affinity, и если появится другой доступный узел, соответствующий этому правилу, стратегия вытеснит под с узла, на который он был изначально назначен.

Пример для `nodeAffinityType: preferredDuringSchedulingIgnoredDuringExecution`. Есть под, который был назначен на узел, т.к. на момент размещения отсутствовали другие узлы, удовлетворяющие правилу `preferredDuringSchedulingIgnoredDuringExecution` node affinity. Если со временем в кластере появится доступный узел, соответствующий этому правилу, стратегия вытеснит под с узла, на который он был изначально назначен.

Стратегия включается параметром [strategies.removePodsViolatingNodeAffinity.enabled](cr.html#descheduler-v1alpha2-spec-strategies-removepodsviolatingnodeaffinity-enabled).
