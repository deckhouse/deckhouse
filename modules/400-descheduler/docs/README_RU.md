---
title: "Модуль descheduler"
description: "Модуль descheduler Deckhouse Platform. Каждые 15 минут анализирует состояние кластера и выполняет вытеснение подов, соответствующих условиям, описанным в активных стратегиях."
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

* `podLabelSelector` — ограничивает поды по лейблам;
* `namespaceLabelSelector` — фильтрует поды по пространствам имен.
* `nodeLabelSelector` — выбирает узлы по лейблам.

## Поставщик метрик

Если в кластере зарегистрирована API-группа `metrics.k8s.io` (например, установлен [metrics-server](https://github.com/kubernetes-sigs/metrics-server)), модуль автоматически обнаруживает её и включает поставщик метрик **KubernetesMetrics** в политике `descheduler`. Это позволяет при установленной [стратегии утилизации](#стратегии) `LowNodeUtilization` использовать данные о **фактическом** потреблении ресурсов из Metrics API, а не полагаться исключительно на запросы и лимиты подов. Соответственно, значения в параметрах [thresholds](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-thresholds) и [targetThresholds](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-targetthresholds) начинают трактоваться как процент от фактического потребления, а не от запросов подов. На стратегию `HighNodeUtilization` это не влияет: она всегда использует запросы и лимиты подов.

От пользователя не требуется никаких действий: модуль обнаруживает API-группу `metrics.k8s.io`, отслеживая ресурсы APIService, и настраивает политику автоматически. Если metrics-server установлен **после** запуска модуля, под `descheduler` будет автоматически перезапущен с обновлённой политикой.

{% alert level="info" %}
Если API-группа `metrics.k8s.io` не зарегистрирована в кластере, модуль оценивает потребление ресурсов на основе запросов и лимитов подов.
{% endalert %}

## Вытеснение в фоновом режиме

Модуль запускает descheduler с включённым feature gate `EvictionsInBackground` из upstream-проекта (`--feature-gates EvictionsInBackground=true`). Отдельной настройки модуля для него нет: поведение заложено в модуль и не требует конфигурации.

По умолчанию descheduler рассчитывает, что каждое вытеснение завершается сразу: запрос к eviction API либо выполняется, либо отклоняется. Некоторые нагрузки так вытеснить нельзя. Например, поду, обслуживающему виртуальную машину, сначала требуется живая миграция, которая может занимать минуты и требовать повторных попыток.

Если feature gate включён, вытеснение пода с аннотацией `descheduler.alpha.kubernetes.io/request-evict-only` считается *запросом* на запуск вытеснения. Контроллер, которому принадлежит под, отклоняет запрос к eviction API с кодом `429 Too Many Requests` и проставляет поду аннотацию `descheduler.alpha.kubernetes.io/eviction-in-progress` на время фактического вытеснения (например, живой миграции). Descheduler отслеживает такие поды и учитывает их в своих лимитах вытеснения, пока под не будет удалён или пока с него не снимут аннотацию. Благодаря этому он не запускает больше вытеснений, чем разрешено, пока предыдущие ещё выполняются.

Поды без аннотации `descheduler.alpha.kubernetes.io/request-evict-only` вытесняются как обычно, поэтому в кластерах, где нет нагрузок с собственной политикой вытеснения, ничего не меняется.

{% alert level="info" %}
Feature gate имеет статус alpha в upstream-проекте [descheduler](https://github.com/kubernetes-sigs/descheduler) (см. [KEP-1397](https://github.com/kubernetes-sigs/descheduler/issues/1397)). Сам по себе он ничего не меняет: без контроллера, отвечающего на запрос вытеснения описанным образом, descheduler работает ровно так же, как раньше.
{% endalert %}

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
Использование ресурсов узла учитывает [extended-ресурсы](https://kubernetes.io/docs/tasks/configure-pod-container/extended-resource/) и рассчитывается на основе запросов и лимитов подов ([requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)), а не их фактического потребления. Такой подход обеспечивает согласованность с работой kube-scheduler, который использует аналогичный принцип при размещении подов на узлах.

Стратегия всегда использует запросы и лимиты подов, независимо от того, доступна ли в кластере API-группа `metrics.k8s.io` (см. [Поставщик метрик](#поставщик-метрик)).
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

По умолчанию `thresholds` и `targetThresholds` задают абсолютные значения в процентах. Если параметр [spec.strategies.lowNodeUtilization.useDeviationThresholds](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-usedeviationthresholds) равен `true`, они отсчитываются от средней нагрузки всех узлов кластера в состоянии Ready: узел считается недостаточно нагруженным, если использование ресурсов на нем не выше среднего значения за вычетом `thresholds`, и избыточно нагруженным, если выше среднего значения с прибавлением `targetThresholds`. Например, при средней загрузке CPU 40% параметры `thresholds: {cpu: 5}` и `targetThresholds: {cpu: 5}` делают узлы с загрузкой не выше 35% недостаточно нагруженными, а выше 45% избыточно нагруженными. Так стратегия удерживает узлы в коридоре ±5% от среднего значения.

Чтобы вытеснять поды постепенно, задайте параметр [spec.strategies.lowNodeUtilization.evictionLimits.node](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-evictionlimits-node) - максимальное количество подов, которые стратегия вытесняет с одного узла за один цикл работы descheduler. Виртуальная машина, которая ещё мигрирует с узла, занимает место в лимите в каждом цикле, поэтому для виртуальных машин параметр фактически ограничивает количество одновременных миграций с одного узла.

{% alert level="warning" %}
Если API-группа `metrics.k8s.io` в кластере **не зарегистрирована**, использование ресурсов узла учитывает [extended-ресурсы](https://kubernetes.io/docs/tasks/configure-pod-container/extended-resource/) и рассчитывается на основе запросов и лимитов подов ([requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)), а не их фактического потребления. Такой подход обеспечивает согласованность с работой kube-scheduler, который использует аналогичный принцип при размещении подов на узлах.

Если API-группа `metrics.k8s.io` **зарегистрирована** (подробнее — [в разделе «Поставщик метрик»](#поставщик-метрик)), стратегия использует данные о **фактическом** потреблении ресурсов из Metrics API *вместо* запросов и лимитов. В этом случае значения, которыми оперирует descheduler, близки к выводу `kubectl top`, но перестают совпадать с оценкой kube-scheduler: узел с исчерпанными запросами может считаться недостаточно нагруженным, и поды с него вытесняться не будут.
{% endalert %}

#### Загрузка CPU, усредненная за 15 минут

Мгновенные значения из Metrics API или запросы подов реагируют на кратковременные всплески нагрузки. Чтобы балансировать узлы по устойчивой нагрузке, задайте параметру [spec.strategies.lowNodeUtilization.metricsSource](cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-metricssource) значение `Prometheus`. Тогда стратегия использует загрузку CPU узла, усредненную за последние 15 минут. Prometheus кластера рассчитывает ее по данным node-exporter.

Как работает стратегия в этом режиме:

1. Используется только ключ `cpu` из `thresholds` и `targetThresholds` как процент CPU узла. Ключ обязателен в обеих секциях, остальные ключи игнорируются.
1. Должен быть включен модуль `prometheus`. Если модуль выключен, стратегия работает как со значением `Default`, а в логе Deckhouse появляется предупреждение.
1. С `useDeviationThresholds: true` стратегия удерживает узлы в коридоре вокруг средней по кластеру загрузки CPU за 15 минут.
1. Стратегия не может оценить вклад одного пода в среднее значение, поэтому за цикл вытесняет с каждого избыточно нагруженного узла один под. Если задан `evictionLimits.node`, стратегия вытесняет с каждого избыточно нагруженного узла до указанного количества подов: в этом режиме параметр увеличивает число вытеснений и не ограничивает его. Не задавайте `evictionLimits.node` с этим источником.
1. Среднее значение отражает вытеснения с задержкой до 15 минут. [При интервале работы](configuration.html#parameters-deschedulinginterval) `Frequent` узел остается избыточно нагруженным несколько циклов после вытеснений и снова теряет поды. С этим источником используйте интервал `Moderate` (15 минут, по умолчанию) или `Rare` (30 минут).
1. Если Prometheus недоступен или в нем нет данных хотя бы для одного узла в состоянии Ready (на нем не работает node-exporter или не удается его опросить, либо узел только добавлен и еще не опрошен дважды), стратегия этого ресурса Descheduler пропускает цикл. Остальные ресурсы Descheduler продолжают работать.

Descheduler читает метрики через `aggregating-proxy` в неймспейсе `d8-monitoring` с токеном своего ServiceAccount. Модуль выдает доступ на чтение, только пока хотя бы один ресурс Descheduler использует этот режим. Пример приведен на странице [«Примеры»](examples.html).

#### Проверка узла назначения

Начиная с Deckhouse 1.78, в кластерах с включённым модулем `control-plane-manager` стратегия перед вытеснением пода запрашивает у kube-scheduler предполагаемый узел назначения. kube-scheduler выполняет для пода пробный цикл планирования без исходного узла и возвращает узлы с наибольшей оценкой.

Как стратегия использует ответ:

1. Под вытесняется, только если каждый узел с наибольшей оценкой в этом цикле недостаточно нагружен по `thresholds` этого же ресурса Descheduler. С `useDeviationThresholds: true` это значит не выше среднего минус `thresholds`. Собственных порогов у проверки нет.
1. Если kube-scheduler поместит под на узел, который не является недостаточно нагруженным, или не найдет для него узла, под остается на месте, и стратегия проверяет следующий под того же узла.
1. Под также остается на месте, если kube-scheduler не ответил или ответил ошибкой: без проверки стратегия под не вытесняет.
1. На время проверки kube-scheduler приостанавливает обычное планирование, поэтому число проверок ограничено. После 5 проверок подов одного избыточно нагруженного узла, не разрешивших вытеснение, узел ждет следующего цикла. После 100 проверок за цикл ресурс Descheduler ничего не вытесняет до следующего цикла. Если под не прошёл проверку из-за отсутствия подходящего узла или из-за того, что не все узлы с наибольшей оценкой недостаточно нагружены, остальные поды того же контроллера на исходном узле в этом цикле не проверяются.

Проверка действует для всех ресурсов Descheduler с `lowNodeUtilization` и не отключается. В управляемом Kubernetes (модуль `control-plane-manager` выключен) стратегия работает без нее. Метрика `descheduler_simulate_target_check_total` считает проверки по результату.

### RemoveDuplicates

{% alert level="info" %}
Предотвращает запуск нескольких подов одного контроллера (ReplicaSet, ReplicationController, StatefulSet) или заданий (Job) на одном узле.
{% endalert %}

Стратегия следит за тем, чтобы на одном узле не находилось больше одного пода ReplicaSet, ReplicationController, StatefulSet или подов одного задания (Job). Если таких подов два или больше, модуль вытесняет лишние поды, чтобы они лучше распределились по кластеру.

Описанная ситуация может возникнуть, если некоторые узлы кластеры вышли из строя по каким-либо причинам, и поды с них были перемещены на другие узлы. Как только вышедшие из строя узлы снова станут доступны для приема нагрузки, эту стратегию можно будет использовать для выселения дублирующих подов с других узлов.

Стратегия включается параметром [strategies.removeDuplicates.enabled](cr.html#descheduler-v1alpha2-spec-strategies-removeduplicates-enabled).

### RemovePodsHavingTooManyRestarts

Стратегия вытесняет с узлов поды со слишком большим количеством перезапусков.
Кандидатами на вытеснение становятся поды, у которых суммарное число перезапусков всех контейнеров, включая init-контейнеры, превышает порог [`podRestartThreshold`](cr.html#descheduler-v1alpha2-spec-strategies-removepodshavingtoomanyrestarts-podrestartthreshold).
Полезна для вытеснения подов в состоянии `CrashLoopBackOff` или с постоянными сбоями,
а также для освобождения ресурсов и возможности планирования новых подов на потенциально более здоровых узлах.

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

### RemovePodsViolatingTopologySpreadConstraint

Стратегия гарантирует, что поды, нарушающие [ограничения распределения по топологии](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/), будут вытеснены с узлов. Вытесняется минимальное количество подов, необходимое для приведения доменов топологии в соответствие с `maxSkew` каждого ограничения.
Стратеги полезна для перебалансировки подов между зонами доступности после восстановления зоны из аварии.
