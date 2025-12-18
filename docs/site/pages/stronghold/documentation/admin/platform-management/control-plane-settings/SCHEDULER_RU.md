---
title: "Планировщик"
permalink: ru/stronghold/documentation/admin/platform-management/control-plane-settings/scheduler.html
lang: ru
---

## Описание алгоритма планировщика

За распределение подов по узлам отвечает планировщик Kubernetes (компонент `kube-scheduler`).

Алгоритм принятия решения планировщиком разбит на 2 фазы: `Filtering` и `Scoring`.

В рамках каждой фазы планировщик запускает набор плагинов, которые реализуют принятие решения, например:

- **ImageLocality** — плагин отдает предпочтение узлам, на которых уже есть образы контейнеров, которые используются в запускаемом поде. Фаза: `Scoring`.
- **TaintToleration** — реализует механизм [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Фазы: `Filtering, Scoring`.
- **NodePorts** — проверяет, есть ли у узла свободные порты, необходимые для запуска пода. Фаза: `Filtering`.

Полный список плагинов можно посмотреть в [документации Kubernetes](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).

В первой фазе фильтрации (`Filtering`) работают filter-плагины, которые проверяют узлы на совпадение с условиями фильтров (taints, nodePorts, nodeName, unschedulable и т.д.).

Отфильтрованный список сортируется с чередованием зон, чтобы не размещать все поды в одной зоне. Предположим, что после фильтрации остались узлы, распределённые по зонам следующим образом:

```text
Zone 1: Node 1, Node 2, Node 3, Node 4
Zone 2: Node 5, Node 6
```

В этом случае они будут выбираться в следующем порядке:

```text
Node 1, Node 5, Node 2, Node 6, Node 3, Node 4
```

Обратите внимание, что с целью оптимизации выбираются не все попадающие под условия узлы, а только их часть. По умолчанию функция выбора количества узлов линейная. Для кластера из ≤50 узлов будут выбраны 100% узлов, для кластера из 100 узлов — 50%, а для кластера из 5000 узлов — 10%. Минимальное значение — 5% при количестве узлов более 5000. Подробнее про ограничение числа узлов можно прочесть в документации Kubernetes на ресурс [KubeSchedulerConfiguration](https://kubernetes.io/docs/reference/config-api/kube-scheduler-config.v1/#kubescheduler-config-k8s-io-v1-KubeSchedulerConfiguration). Deckhouse использует значение по умолчанию, поэтому в очень больших кластерах нужно учитывать это поведение планировщика.

После того как выбраны узлы, подходящие под условия фильтров, запускается фаза `Scoring`. Плагины этой фазы анализируют список отфильтрованных узлов и назначают оценку (score) каждому узлу. Оценки от разных плагинов суммируются. На этой фазе оцениваются доступные ресурсы на узлах, pod capacity, affinity, volume provisioning и так далее.

Результатом работы этой фазы является список узлов с максимальной оценкой. Если в списке больше одного узла, то узел выбирается случайным образом.

### Документация

- [Общее описание scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/);
- [Система плагинов](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins);
- [Подробности фильтрации узлов](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduler-perf-tuning/);
- [Исходный код scheduler](https://github.com/kubernetes/kubernetes/tree/master/cmd/kube-scheduler).

## Изменение и расширение логики работы планировщика

Для изменения логики работы планировщика можно использовать [механизм плагинов расширения](https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/624-scheduling-framework/README.md).

Каждый плагин представляет собой вебхук, отвечающий следующим требованиям:

* Использование TLS.
* Доступность через сервис внутри кластера.
* Поддержка стандартных *Verbs* (filterVerb = filter, prioritizeVerb = prioritize).
* Также, предполагается что все подключаемые плагины могут кэшировать информацию об узле (`nodeCacheCapable: true`).

Подключить такой вебхук extender можно при помощи ресурса [KubeSchedulerWebhookConfiguration](/modules/control-plane-manager/cr.html#kubeschedulerwebhookconfiguration).

{% alert level="critical" %}
При использовании опции `failurePolicy: Fail`, ошибка в работе вебхука приводит к остановке работы планировщика и новые поды не смогут запуститься.
{% endalert %}

## Ускорение восстановления при потере узла

<!-- TODO вот тут надо состыковать как-то с виртуальными машинами. -->

По умолчанию, если узел в течении 40 секунд не сообщает свое состояние, он помечается как недоступный. Ещё через 5 минут поды такого узла будут назначены планировщиком на другие узлы. Итоговое время недоступности приложений составляет около 6 минут.

Для специфических задач, когда приложение не может быть запущено в нескольких экземплярах, в настройках модуля `control-plane-manager` есть способ сократить период  недоступности:

1. Уменьшить время перехода узла в состояние `Unreachable` при потере с ним связи настройкой параметра `nodeMonitorGracePeriodSeconds`.
1. Уменьшить таймаут переназначения узла поду в параметре `failedNodePodEvictionTimeoutSeconds`.

### Пример

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    nodeMonitorGracePeriodSeconds: 10
    failedNodePodEvictionTimeoutSeconds: 50
```

В этом случае при потере связи с узлом приложения будут запущены на других узлах примерно через 1 минуту.

{% alert level="warning" %}
Оба описанных параметра оказывают непосредственное влияние на потребление ресурсов процессора и памяти на master-узлах. Уменьшенные таймауты заставляю системные компоненты чаще производить отправку статусов и сверку состояний ресурсов.

В процессе подбора подходящих значений обращайте внимание на графики потребления ресурсов master-узлов. Будьте готовы к тому, что для обеспечения приемлимых значений параметров может потребоваться увеличение мощностей, выделенных для master-узлов.
{% endalert %}
