
## agent

- ds на мастерах
- hostnetwork
- прожка на go делает частые (раз в секунду?) проверки по нашему SLA и возвращает counter’ы:
количество проверок
количество успешных проверок
секунд было недоступно
- раз в 5 минут агент скидывает данные в upmeter
- для отправки используется “wal”, так что если upmeter недоступен – данные дошлются.

## upmeter

- хранит всё в sqlite базе, в файле /db/dowtime.db.sqlite

- Умеет принимать данные от агентов:
Сразу дедуплицирует данные из 30 секундных таймслотов в 5 минутные (выбирая “лучший” результат)
Складывает данные в sqlite базу

- Хранит 30 секундные таймслоты только за сутки, старое удаляет из таблицы
- Хранит 5 минутные таймслоты постоянно, ничего не удаляет 

-  Умеет читать CRD `Downtime`, в этих CRD:
  - startDate, endDate: время начала и время конца простоя в ISO формате,
  - type: тип простоя:
    - Accident – авария “по нашей вине”
    - Maintenance – плановые работы,
    - InfrastructureMaintenance – плановые работы у провайдера инфраструктуры
    - InfrastructureAccident – проблемы с инфраструктурой у провайдера
  - description: информация для пользователей
  - affected: перечень подсистем/компонентов, которых касается касается Downtime

Пример Downtime:

```
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

### API

Список подсистем, с информацией о списке компонентов для каждой подсистемы.
Уровень доступности по подсистеме/компоненту, в запросе передается:
Период (с-по) и step, например,
чтобы получить месячный uptime нужно передать последние 30 дней в качестве периода и step 30d,
а чтобы получить данные по дням за последнюю неделю нужно передать 7 дней и step 1d.
Дополнительным параметром можно передать, какие виды простоя включить в расчет (Maintenance, InfrastructureMaintenance, InfrastructureAccident) – в этом случае уровень доступности рассчитывается без учета простоев этих типов.
Состояние доступности по подсистеме/компоненту (для отрисовки “графика доступности”). Передаются step и период (с-по). Для каждого step возвращается состояние:


доступен
не доступен
если есть, uid Downtime с Accident
не доступен, без нарушения SLA
был Maintenance (+ uid Downtime)
был InfrastructureMaintenance (+ uid Downtime)
был InfrastructureAccident (+ uid Downtime)
нет данных


### Алгоритм

- Подписывается на CR Downtime и на список pod’ов измерятора.
- С информером работает web-интерфейс Deckhouse, а так же “отправлятор” в “центральную штуку” (cronjob).

## Проверки

- control-plane — запросы к API серверу кластера
  - access
  - basic-functionality
  - control-plane-manager
  - namespace
  - scheduler
- synthetic — запросы к smokeMini
  - access
  - dns
  - neighbor
  - neighbor-via-service
- nginx
- node-group
- monitoring-and-autoscaling
- extensions-availability

## smoke-mini

Пробы из группы "synthetic" опрашивают smoke-mini — приложение, имитирующее настоящее. Это позволяет оценить, как будут вести себя настоящие приложения в кластере. `smoke-mini` запускает 3 `StatefulSet` использующих `PV` и каждый имеющий 1 реплику, со специальным приложением, поднимающим http-сервер и предоставляющим API для выполнения тестов. Ресурсы одного из `StatefulSet` перешедуливаются раз в 10 минут на случайные узлы.

### Функционал тестирования
* `/` – return 200
* `/error` – return 500
* `/api` – проверяет доступ к API Kubernetes (запрашивается информация по поду из которого выполняется запрос `/api/v1/namespaces/d8-smoke-mini/pods/<POD_NAME>`)
* `/dns` – проверяет работу кластерного dns (выполняет резолв домена `kubernetes.default`)
* `/disk` – проверяет, что может создать и удалить файл
* `/neighbor` – проверяет, есть ли доступ к "соседу" по HTTP
* `/prometheus` – проверяет, что может отправить запрос в прометей `/api/v1/metadata?metric=prometheus_build_info`
