---
title: Возможности Deckhouse
permalink: features.html
---

## Что работает сразу
- [Тюнинг](/candi/bashible/common-steps/node-group/041_configure_sysctl_tuner.sh.tpl) системных параметров, в том числе:
   - отключение Transparent Huge Pages (THP);
   - тюнинг сетевого стека;
   - увеличение лимитов PID, inotify, файлов, соединений, и т.п. ...
- Kubernetes [Dashboard]({{ site.baseurl }}/modules/500-dashboard/);
- [Nginx Ingress Controller]({{ site.baseurl }}/modules/400-nginx-ingress/);
- [Автокопирование секретов]({{ site.baseurl }}/modules/600-secret-copier/) при создании `namespace`;
- [Очистка](/candi/bashible/common-steps/node-group/042_configure_systemd_slices_cleaner.sh.tpl) systemd-слайсов в Ubuntu 16 на нодах;
- Настраивается [cert-manager]({{ site.baseurl }}/modules/101-cert-manager/) и создаются его CRD;
- **[v1.11+]** Создаются [PriorityClass]({{ site.baseurl }}/modules/010-priority-class/). **Но!** Чтобы заработал учет приоритетов при шедулинге, необходимо еще [расставить]({{ site.baseurl }}/modules/010-priority-class/) `priorityClassName` контроллерам подов;
- [Prometheus и Grafana]({{ site.baseurl }}/modules/300-prometheus) — ключевой компонент мониторинга кластера. Если он включен, то также сразу работают:
    - [Ping мониторинг]({{ site.baseurl }}/modules/340-monitoring-ping/) сетевого взаимодействия между всеми узлами кластера;
    - [Расширенный мониторинг]({{ site.baseurl }}/modules/340-extended-monitoring/) на ноде по месту и inode;
    - [HPA]({{ site.baseurl }}/modules/301-prometheus-metrics-adapter/) — для работы горизонтального автомасштабирования (экземплярами подов);
    - [VPA]({{ site.baseurl }}/modules/302-vertical-pod-autoscaler/) — для работы вертикального автомасштабирования (ресурсами подов). Для работы нужен включенный `prometheus-mertics-adapter`.

## Рекомендуется включить или настроить
- Включить [кэширующий DNS]({{ site.baseurl }}/modules/350-node-local-dns/) — это ускорит работу с DNS, особенно на нагруженных системах.
- Включить расширенный мониторинг в продуктивных `namespace` ([extended-monitoring]({{ site.baseurl }}/modules/340-extended-monitoring/)) — если поставить на `namespace` аннотацию `extended-monitoring.flant.com/enabled`, то включается расширенный мониторинг с алертами.
- **[1.11+]** [Расставить]({{ site.baseurl }}/modules/010-priority-class/) `priorityClassName` контроллерам подов, чтобы заработал учет приоритетов при шедулинге.
- [Включить VPA]({{ site.baseurl }}/modules/302-vertical-pod-autoscaler/) для каждого пода, как минимум в режиме `Off`.

## Автоскейлинг (масштабирование)

В Antiopa есть следующие модули, для настройки автоскейлинга:
- [priority-class]({{ site.baseurl }}/modules/010-priority-class/)
- [vertical-pod-autoscaler]({{ site.baseurl }}/modules/302-vertical-pod-autoscaler/)
- [prometheus-metrics-adapter]({{ site.baseurl }}/modules/301-prometheus-metrics-adapter/)

В кластерах с версии 1.9 для работы HPA используется `prometheus-metrics-adapter` (нужен включенный `prometheus`).

HPA ([Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)) — встроенный функционала kubernetes, позволяет увеличивать количество экземпляров подов в зависимости от указанных значений метрик. Модуль `prometheus-metrics-adapter` добавляет к стандартным метрикам автоскейлинга по `cpu` и `memory` еще больше метрик, по которым можно настраивать автоскейлинг (в итоге автоскейлить можно по любой метрике, если [завести issue](https://github.com/deckhouse/deckhouse/issues/new?issue) в Antiopa).

В обычной ситуации HPA имеет смысл применять на облачной инфраструктуре, т.к. скейлить внутри bare-metal кластера как правило — некуда. Но, в связке с модулем [priority-class]({{ site.baseurl }}/modules/010-priority-class/) автоскелить имеет смысл и на bare-metal кластерах. Модуль `priority-class` позволяет кластерному шедулеру работать с учетом установленных у подов приоритетов. Таким образом, при автоскейлинге и нехватке ресурсов в кластере, поды более высокого приоритета будут вытеснять поды более низкого приоритета. Это может например привести к тому, что при увеличении нагрузки на продуктивный namespace, поды тестового namespace будут удалены (evicted) и повиснут в `pending` пока нагрузка не спадет и HPA не уменьшит количество подов в продуктивном `namespace`.


## Полезные доски Grafana

### Анализ HTTP/HTTPS трафика

Доски в разделе `Ingress Nginx` — информация в разрезе namespace и vhost с глубокой детализацией.

### Анализ по namespace

Доски:
- `Main / Namespace`
- `Main / Namespace / Controller`
- `Main / Namespace / Controller / Pod`
- `Main / Namespaces`

Потребление ресурсов в разрезе namespace, контроллеров подов. Доски позволяют вам найти поды, которые потребляют нетипичное количество ресурсов, к которым приходит OOMKiller и т.д., а также позволяют более точно настроить лимиты, отталкиваясь от анализа данных по:
- `Over req` (CPU/MEM) - количество ядер или памяти запрошеных с избытком (не используемых).
- `Under req` (CPU/MEM) - количество ядер или памяти запрошеных с недостатком — реальное потребление выше чем запрошено.

Statusmap (второй блок сверху на всех досках, кроме `Main / Namespaces`) позволяет быстро увидеть что где-то что-то рестартилось, отваливалось, и т.д.

В группе графиков Controllers memory есть параметр `Working set bytes` - показывает минимально необходимую процессам контейнера память, включая кэш открытых файлов, сетевые сокеты, RSS, память ядра. Соответственно памяти контейнеру должно быть выделено больше, чем указано в `Working set bytes`.

> Если под работает в `host network`, то сетевой трафик будет дублироваться на графиках по количеству таких подов. Анализ трафика на ноде нужно делать на графике `Kubernetes / Nodes` по соответствующей ноде.

## Отказоустойчивость

Некоторые модули поддерживают режим отказоустойчивости, в этом режиме модуль должен продолжать работу при выходе из строя одной ноды. Например, некоторые компоненты дополняются репликами, а также принимаются иные меры, такие как настройка кворума и пр.

Как управлять? В порядке приоритета:

* Вручную, с помощью настроек в CM:
    * Локальный флаг `moduleName.highAvailability`. По-умолчанию не определён.
    * Глобальный флаг `global.highAvailability`. По-умолчанию не определён.
* Автоматически — есть autodiscovery-параметр `global.discovery.clusterControlPlaneIsHighlyAvailable`, который отражает наличие реплик у apiserver. Если они есть, значит инженеры посчитали кластер важным и позаботились о репликах для control plane, а значит — и дополнения должны работать в отказоустойчивом режиме.

> Внимание! Следующие компоненты, от которых зависит работа apiserver, отправляются на master сервера с количеством реплик, соответствующим количеству мастеров в кластере:
> 1. cainjector и webhook из 101-cert-manager
> 2. prometheus-metrics-adapter из 301-prometheus-metrics-adapter
> 3. vpa-admission-controller из 302-vertical-pod-autoscaler
> 4. cloud-controller-manager из модулей 030-cloud-provider-*
> 5. machine-controller-manager из 040-cloud-instance-manager
> 6. ingress-conversion-webhook из 400-nginx-ingress
>
>В случае, если используется managed кластер kubernetes от cloud provider и включен highAvailability либо глобально, либо для одного из модулей, то
>предполагается, что в кластере есть минимум 2 ноды, которые подходят под стратегию `master` ([node selector'ов]({{ site.baseurl }}/guides/development.html#node-selector))

| Модуль   |      Статус   |
|----------|---------------|
| 010-priority-class              | Не требуется |
| 010-operator-prometheus-crd     | Не требуется |
| 010-vertical-pod-autoscaler-crd | Не требуется |
| 020-deckhouse                   | Нет возможности |
| 030-cloud-provider-gcp          | Да |
| 030-cloud-provider-openstack    | Да |
| 030-cloud-instance-manager      | Да |
| 101-cert-manager                | Да |
| 140-user-authz                  | DaemonSet |
| 150-user-authn                  | Да |
| 200-operator-prometheus         | [Не требуется](https://github.com/coreos/prometheus-operator/issues/2491) |
| 230-vsphere-csi-driver          | Не требуется |
| 300-prometheus                  | Да |
| 301-prometheus-metrics-adapter  | Да |
| 302-vertical-pod-autoscaler     | Не требуется |
| 303-prometheus-pushgateway      | Нет возможности |
| 340-node-problem-detector       | DaemonSet |
| 340-extended-monitoring         | [Пока нет](https://github.com/deckhouse/deckhouse/issues/510) |
| 350-node-local-dns              | DaemonSet |
| 360-istio                       | *Нет* |
| 400-descheduler                 | Не требуется |
| 400-nginx-ingress               | DaemonSet |
| 450-keepalived                  | Да |
| 450-network-gateway             | Да |
| 500-basic-auth                  | Да |
| 500-dashboard                   | Да |
| 500-dynatrace                   | DaemonSet |
| 500-okmeter                     | DaemonSet |
| 500-openvpn                     | [Пока нет](https://github.com/deckhouse/deckhouse/issues/518) |
| 600-ping-exporter               | DaemonSet |
| 600-secret-copier               | Не требуется |
| 999-helm                        | Не требуется |
