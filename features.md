Antiopa features list
=====================

## Работает сразу
- [Тюнинг](/modules/700-sysctl-tuner/README.md) системных параметров, в том числе:
   - отключение Transparent Huge Pages (THP);
   - тюнинг сетевого стека;
   - увеличение лимитов PID, inotify, файлов, соединений...
- [Мониторинг](/modules/600-node-ping/README.md) сетевого взаимодействия между всеми узлами кластера и отрисовка графиков (работает, если включен `prometheus`).
- [Мониторинг](/modules/350-extended-monitoring/README.md) на ноде по месту и inode.

## Рекомендуется включить или настроить
- Включить кэширующий DNS ([node-local-dns](/modules/350-node-local-dns/README.md)). Ускоряет работу с DNS, особенно на нагруженных системах.
- Включить расширенный мониторинг в продуктивных `namespace` ([extended-monitoring](/modules/350-extended-monitoring/README.md)). Если поставить на `namespace` аннотацию `extended-monitoring.flant.com/enabled`, то включается расширенный мониторинг с алертами.
- В Kubernetes **>= 1.11** [расставить](/modules/010-priority-class/README.md) `priorityClassName`, чтобы заработал учет приоритетов при шедулинге).
- [VPA](/modules/302-vertical-pod-autoscaler/README.md) для каждого пода, как минимум в режиме `Off`.

## Автоскейлинг (масштабирование)

В Antiopa есть следующие модули, для настройки автоскейлинга:
- [priority-class](/modules/010-priority-class/README.md)
- [vertical-pod-autoscaler](/modules/302-vertical-pod-autoscaler/README.md)
- [prometheus-metrics-adapter](/modules/301-prometheus-metrics-adapter/README.md)
- [heapster](/modules/200-heapster/README.md)

`Heapster` нужен для работы HPA ([Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)) - встроенного функционала kubernetes. HPA позволяет увеличивать количество экземпляров подов в зависимости от указанных значений метрик, а модуль `prometheus-metrics-adapter` добавляет к стандартным метрикам по `cpu` и `memory` еще больше метрик, по которым можно настраивать автоскейлинг (в итоге автоскейлить можно по любой метрике, если завести issue в Antiopa).

В обычной ситуации HPA имеет смысл применять на облачной инфраструктуре, т.к. скейлить внутри bare-metal кластера как правило - некуда. Но, в связке с модулем [priority-class](/modules/010-priority-class/README.md) автоскелить имеет смысл и на bare-metal кластерах. Модуль `priority-class` позволяет кластерному шедулеру работать с учетом установленных у подов приоритетов. Таким образом, при автоскейлинге и нехватке ресурсов в кластере, поды более высокого приоритета будут вытеснять поды более низкого приоритета. Это может например привести к тому, что при увеличении нагрузки на продуктивный namespace, поды тестового namespace будут удалены (evicted) и повиснут в `pending` пока нагрузка не спадет и HPA не уменьшит количество подов в продуктивном `namespace`.


## Полезные доски Grafana

### Анализ по namespace

Доски:
- `Main / Namespace`
- `Main / Namespace / Controller`
- `Main / Namespace / Controller / Pod`
- `Main / Namespaces`

Потребление ресурсов в разрезе namespace, контроллеров подов. Доски позволяют вам найти поды, которые потребляют нетипичное количество ресурсов, к которым приходит OOMKiller и т.д., а также позволяют более точно настроить лимиты, отталкиваясь от анализа данных по:
- `Over req` (CPU/MEM) - количество ядер или памяти запрошеных с избытком (не используемых).
- `Under req` (CPU/MEM) - количество ядер или памяти запрошеных с недостатком - реальное потребление выше чем запрошено.

Statusmap (второй блок сверху на всех досках, кроме `Main / Namespaces`) позволяет быстро увидеть что где-то что-то рестартилось, отваливалось, и т.д.

В группе графиков Controllers memory есть параметр `Working set bytes` - показывает минимально необходимую процессам контейнера память, включая кэш открытых файлов, сетевые сокеты, RSS, память ядра. Соответственно памяти контейнеру дожно быть выделенно больше, чем указано в `Working set bytes`.

> Если под работает в `host network`, то сетевой трафик будет дублироваться на графиках по количеству таких подов. Анализ трафика на ноде нужно делать на графике `Kubernetes / Nodes` по соответствующей ноде.
