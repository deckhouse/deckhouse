---
title: "Описание архитектуры"
---

- [Агент](#агент)
- [Сервер](#сервер)
  - [CRD](#crd)
    - [Downtime](#downtime)
    - [UpmeterRemoteWrite](#upmeterremotewrite)
- [Проверки](#проверки)
  - [Control Plane](#control-plane)
    - [Жизненный цикл объекта](#жизненный-цикл-объекта)
    - [Жизненный цикл состояния объекта](#жизненный-цикл-состояния-объекта)
    - [Жизненный цикл дочернего объекта](#жизненный-цикл-дочернего-объекта)
  - [Synthetic](#synthetic)
    - [Функционал тестирования](#функционал-тестирования)

Приложение состоит из трех частей:

- upmeter agent (ds/upmeter-agent)
- upmeter server (sts/upmeter)
- smoke-mini (sts/smoke-mini-{a,b,c,d,e})

## Агент

Агент измеряет доступность компонентов Deckhouse. Измерение состоит в проверке состояния объекта через API кластера или проверке ответа на HTTP-запрос к приложению. Например, что состояние Pod'а
Ready или что Prometheus отвечает корректно на HTTP-запрос.

Логическая единица доступности называется пробой. Проба тестирует некоторую функциональность. Проба
состоит из одной или нескольких параллельных проверок. Например, проба `cluster-scaling` состоит из
трех проверок, проверяющих статус Pod'ов `cloud-controller-manager`, `machine-controller-manager` и
`bashible-apiserver`.

Если одна из проверок выявила недоступность, компонента, статус пробы принимается `down`. Помимо статуса `down`, статус может быть:  `up`, если нет недоступных результатов проверок; `uncertain`,
если все проверки не смогли установить доступность или недоступность компонентов. Так может
получиться, если не выполнены условия, заложенные в проверку. Например, проверки, опирающиеся на
статус Pod'а, будут в статусе `uncertain`, если перед тем как проверить Pod не подтвердится
доступность API-сервера.

Результат запуска пробы — статус доступности той функциональности, которую проба проверяет. Пробы
запускаются периодически с заранее заданным интервалом. Самая частая проба — `dns`, — запускается 5
раз в секунду. Самые редкие пробы запускаются раз в минуту, например, проверка жизненного цикла
`namespace`.

Результаты проверок снимаются с минимальной необходимой частотой — 5 раз в секунду и собираются в
массив статусов проб. Накопление статусов длится 30 секунд. Каждые 30 секунд собирается статистика
для каждой пробы. Статистика состоит из четырех чисел:

- длительность доступности (uptime);
- длительность недоступности (downtime);
- длительность неопределенности (uncertain);
- оставшееся время из 30 секунд, за которое изменения не проводилось (nodata).

Измерение не проводится только в том случае, если агент не запущен.

Пробы объединяются в группы доступности. На эти группы выдается SLA. Статистика доступности группы
 вычисляется агентом таким же образом во время сбора статистики проб. Статус группы вычисляется из статусов
проб так же, как статус пробы вычисляется из статусов проверок.

Статистика доступности проб и групп за 30 секунд отправляется в `upmeter server` по HTTP.

Agent — это DaemonSet, который запускается только на узлах control plane. Pod'ы агента используют
SQLite для «WAL». Поэтому если `upmeter` недоступен, данные будут отправлены, когда тот поднимется.
Данные в WAL ограничены последними 24 часами.

## Сервер

Принимает статистику доступности за 30 секунд от агентов и складывает в статистику за 5 минут. Если
в кластере более одного агента, сервер выбирает лучший вариант статистики. 5-минутная статистика
хранится за все доступное время. БД для хранения — SQLite, файл `/db/dowtime.db.sqlite`.

Сервер отдает данные в виде JSON. Этими данным пользуются дашборд upmeter и статус-страница.

### CRD

#### Downtime

Функциональность не реализована до конца.

Если в кластере или в инфраструктуре проводились работы, повлекшие за собой простой, то это можно
зафиксировать с помощью объекта CRD `downtime.deckhouse.io`. В этом объекте указывают ожидаемый тип
простоя, интервал времени и затронутые группы доступности или пробы. Сервер учет это время как
`uncertain` для указанных групп и проб.

- Умеет читать CR `Downtime`, в этих CRD:
  - startDate, endDate: время начала и время конца простоя в формате ISO,
  - type: тип простоя:
    - Accident – авария “по нашей вине”;
    - Maintenance – плановые работы;
    - InfrastructureMaintenance – плановые работы у провайдера инфраструктуры;
    - InfrastructureAccident – проблемы с инфраструктурой у провайдера;
  - description: информация для пользователей;
  - affected: перечень подсистем/компонентов, которых касается касается Downtime.

Пример Downtime:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Downtime
metadata:
  name: change-pod-cidr
  labels:
    heritage: deckhouse
    module: upmeter
spec:
- startDate: "2020-10-23T12:00:00Z"
  endDate: "2020-10-23T13:00:00Z"
  type: Maintenance
  description: "Change Pod's CIDR, ticket #33121"
  affected:
  - synthetic
  - control-plane
```

#### UpmeterRemoteWrite

Сервер может экспортировать данные в виде метрик по протоколу Prometheus Remote Write.

```yaml
apiVersion: deckhouse.io/v1
kind: UpmeterRemoteWrite
metadata:
  labels:
    heritage: upmeter
    module: upmeter
  name: victoriametrics
spec:
  additionalLabels:
    cluster: cluster-name
    some: fun
  config:
    url: https://victoriametrics.somewhere/api/v1/write
    basicAuth:
      password: "Cdp#Cd.OxfZsx4*89SZ"
      username: upmeter
  intervalSeconds: 300
```

## Проверки

- control-plane
  - access
  - basic-functionality
  - controller-manager
  - namespace
  - scheduler
  - cert-manager
- deckhouse
  - cluster-configuration
- extensions
  - cluster-scaling
  - dashboard
  - dex
  - grafana
  - openvpn
  - prometheus-longterm
- load-balancing
  - load-balancer-configuration
  - metallb
- monitoring-and-autoscaling
  - prometheus
  - prometheus-metrics-adapter
  - vertical-pod-autoscaler
  - horizontal-pod-autoscaler
  - key-metrics-presence
  - metric-sources
  - trickster
- nginx
  - *(controller name)*
- nodegroups
  - *(CloudEphemeral node group name)*
- synthetic
  - access
  - dns
  - neighbor
  - neighbor-via-service

### Control Plane

#### Жизненный цикл объекта

Проба направлена на доступность API-сервера. Созданием и удалением объекта проверяется простой
жизненный цикл объекта. Ошибки операций с API-сервером засчитываются как нерабочее состояние.

Пробы:

- `Basic Functionality`, создается и удаляется ConfigMap
- `Namespace`, создается и удаляется Namespace

![Single object lifecycle](01-single-object-lifecycle.png)

#### Жизненный цикл состояния объекта

Проба направлена на определение состояние объекта в его жизненном цикле. Проба считается
непрошедшей, если объект не приобретает ожидаемого состояния. Например, если Pod не шедулится на
узел. Ошибки операций с API-сервером считаются условием для неопределенного результата пробы.

Пробы:

- `Scheduler`, Pod'у должен быть назначен узел

![Controller object lifecycle](02-controller-object-lifecycle.png)

#### Жизненный цикл дочернего объекта

Проба направлена на контроллер, который при создании одного объекта порождает другой. Проба
проверяет, что жизненный цикл дочернего объекта ожидаемо связан с жизненным циклом родительского.

Пробы:

- `Controller Manager`: StatefulSet → Pod,
- `Cert Manager`: Certificate → Secret.

![Parent-child lifecycle](03-parent-child-lifecycle.png)

### Synthetic

Пробы из группы «synthetic» опрашивают smoke-mini — модельное приложение, веб-сервер. Это позволяет
оценить, как будут вести себя настоящие приложения в кластере. `smoke-mini` запускает пять
StatefulSet с HTTP-сервером и предоставляющим API для выполнения тестов. Pod одного из StatefulSet
пересоздаются раз в минуту на случайные узлы. Планировщик для smoke-mini — хук
`smokemini/reschedule.go`.

#### Функционал тестирования

* `/` – доступность самого Pod'а, всегда возращает код ответа 200;
* `/dns` – проверяет работу кластерного DNS, выполняя разрешение доменого имени `kubernetes.default`;
* `/neighbor` – проверяет, есть ли доступ к «соседнему» StatefulSet по HTTP-адресу Pod'а;
* `/neighbor-via-service` – проверяет, есть ли доступ к «соседнему» StatefulSet через общий сервис.
