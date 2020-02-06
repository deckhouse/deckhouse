Модуль descheduler
==================

### Что это такое

Шедулинг в Kubernetes это процесс, который биндит pending поды на ноды и выполняется компонентом kube-scheduler. Kube-scheduler на основе политик пода и состояния нод определяет, на какую ноду прибиндить под. Решение принимается в момент, когда появился под в статусе pending. Но kubernetes кластер очень динамичный и его состояние меняется с течением времени, может появиться потребность в переносе уже запущенного пода на другую ноду по разным причинам:
* Некоторые узлы кластера недогружены или перегружены.
* Первоначальные условия при шедулинге уже не соответствуют действительности (добавлены/удалены taint/labels, pod/node affinity).
* Часть узлов выпали из кластера и поды с них переехали на другие ноды.
* Новые узлы добавлены в кластер.

Descheduler находит поды по политикам и эвиктит "лишние" поды. Тогда kube-scheduler двигает эти поды по новым условиям.

### Как это работает

Данный модуль добавляет в кластер cronjob с [descheduler](https://github.com/kubernetes-incubator/descheduler), который выполняется раз в 15 минут, находит по политикам из [config-map](templates/config-map.yaml) и эвиктит найденные поды.

У descheduler есть 4 политики:
* RemoveDuplicates (**выключена по-умолчанию**)
* LowNodeUtilization (**выключена по-умолчанию**)
* RemovePodsViolatingInterPodAntiAffinity (**включена по-умолчанию**)
* RemovePodsViolatingNodeAffinity (**включена по-умолчанию**)

#### RemoveDuplicates

Данная политика следит за тем, чтобы на одной ноде не было запущенно более одного пода одного контроллера (rs, rc, deploy, job). Если таких подов 2 на одной ноде, descheduler убивает один под.

К примеру, у нас есть 3 ноды (одна из них более нагруженная), и мы хотим выкатить 6 реплик приложения. Так как одна из нод перегружена, то scheduler прибиндит к нагруженной ноде 0 или 1 под, а остальные реплики поедут на другие ноды, и в таком случае descheduler будет каждые 15 минут прибивать "лишние" поды на не нагруженных нодах и надеяться, что scheduler прибиндит их к этой нагруженной ноде.

#### LowNodeUtilization

Данная политика находит нагруженные и не нагруженные ноды в кластере по cpu/memory/pods (в процентах) и, при наличии и тех и других, эвиктит поды с нагруженных нод. Данная политика учитывает не реально потребленные ресурсы на ноде, а requests подов.
Пороги, по которым узел определяется как малонагруженный или перегруженный, в настоящий момент предопределены и их нельзя изменять:
* Параметры определения малонагруженных нод:
  * cpu — 40%
  * memory — 50%
  * pods — 40%
* Параметры определения перегруженных нод:
  * cpu — 80%
  * memory — 90%
  * pods — 80%

#### RemovePodsViolatingInterPodAntiAffinity

Данная политика следит за тем, чтобы все "нарушители" anti-affinity были удалены. В какой ситуации может быть нарушен InterPodAntiAffinity нам самим придумать не удалось, а в официальной документации по descheduler написано что-то совершенно неубедительное:
> This strategy makes sure that pods violating interpod anti-affinity are removed from nodes. For example, if there is podA on node and podB and podC(running on same node) have antiaffinity rules which prohibit them to run on the same node, then podA will be evicted from the node so that podB and podC could run. This issue could happen, when the anti-affinity rules for pods B,C are created when they are already running on node.

#### RemovePodsViolatingNodeAffinity

Данная политика отвечает за кейс, когда под был зашедулен на ноду по условию (`requiredDuringSchedulingIgnoredDuringExecution`), а потом нода перестала удовлетворять условиям и, тогда descheduler увидит это и сделает все, что бы под переехал туда, где она будет удовлетворять условиям.

### Известные особенности

* Критикал поды (с аннотацией `scheduler.alpha.kubernetes.io/critical-pod` или с `priorityClassName = system-cluster-critical/system-node-critical`) не эвиктятся.
* При эвикте подов с нагруженной ноды учитывается priorityClass.
* Поды без контроллера или с контроллером DaemonSet не эвиктятся.
* Поды с local storage не эвиктятся.
* Best effort поды эвиктсятся раньше, чем Burstable и Guaranteed.
* Descheduler использует Evict API и поэтому учитывается [Pod Disruption Budget](https://fox.flant.com/docs/kb/blob/master/qa/pod-disruption-budget.md) и если он нарушает его условия, то он не эвиктит под.

Конфигурация
-----------

### Что нужно настраивать?

Обязательных настроек нет.

### Параметры

* `removePodsViolatingNodeAffinity` — включить данную политику.
  * По-умолчанию включено (`true`).
* `removePodsViolatingInterPodAntiAffinity` — включить данную политику.
  * По-умолчанию включено (`true`).
* `removeDuplicates` — включить данную политику.
  * По-умолчанию выключено (`false`).
* `lowNodeUtilization` — включить данную политику.
  * По-умолчанию выключено (`false`).
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
  * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
  * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
  * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
  * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

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
