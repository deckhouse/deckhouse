Модуль prometheus
=======

Модуль устанавливает [prometheus](https://prometheus.io/) (используя модуль [prometheus-operator](../200-prometheus-operator/)) и полностью его настраивает!

В зависимости от настроек, модуль может установить один или несколько экземпляров Prometheus:
* **main** — основной Prometheus, который выполняет scrape каждые 30 секунд. Именно он обрабатывает все правила, шлет алерты и является основным источником данных.
* **longterm** — дополнительный Prometheus, который выполняет scrape данных из main каждые 5 минут. Используется для продолжительного хранения истории и для отображения больших промежутков времени.

Дополнительная информация
-------------------------

* [Интеграция с Madison](docs/MADISON.md)
* [Разработка правил для Prometheus (алертов и recording)](docs/PROMETHEUS_RULES_DEVELOPMENT.md)
* [Разработка графиков (Dashboard'ов для Grafana)](docs/GRAFANA_DASHBOARD_DEVELOPMENT.md)
* [Разработка target'ов для Prometheus (целей, которые мониторить)](docs/PROMETHEUS_TARGETS_DEVELOPMENT.md)
* [Кастомные правила и графики для конкретного проекта](docs/PROJECT_CUSTOMIZATION.md)

Конфигурация
------------

### Что нужно настраивать?

При установке **нужно настроить два параметра**:
```yaml
prometheus: |
  retentionDays: 15
  estimatedNumberOfMetrics: 250000
```

### Отключение автоматической регистрации в Madison

**Важно!** Есть два случая, когда необходимо **обязательно** отключить автоматическую регистрацию в Madison:
1. В **тестовом кластере**, который вы развернули для своих экспериментов (или каких-то экспериментов в команде) нужно **обязательно отключать алерты**. Это правило НЕ относится к dev-кластерам клиентов, в которых алерты нам обязательно нужны.
2. В любых **кластерах снятых с поддержки** (например, когда мы расстаемся с клиентом).

Чтобы полностью отключить и автоматическую регистрацию в Madison и алерты нужно привести конфиг к следующему виду (удалив все другие параметры prometheus):

```yaml
prometheus: |
  madisonSelfSetupKey: false
```

### Параметры

* `retentionDays` — сколько дней хранить данные.
    * По-умолчанию `7`.
    * **Важно!!!** При изменении этого параметра перезаказывается диск (при этом удаляются все данные).
* `retentionGigabytes` — сколько гибибайт хранить.
    * По-умолчанию `200`.
* `estimatedNumberOfMetrics` — примерное количество метрик, которые планируется хранить в prometheus (на основании этого параметра рассчитывается размер диска и размер памяти для prometheus)
    * По-умолчанию `200000`.
    * **Важно!!!** При изменении этого параметра перезаказывается диск (при этом удаляются все данные).
    * Примерные значения (в зависимости от количества узлов и подов):
        * 1 узел, 37 подов (someproject) — 22 000
        * 6 узлов, 72 пода (someproject) — 49 000
        * 10 узлов, 310 подов (someproject) — 333 000
        * 22 узла, 570  подов (someproject.prod) — 400 000
    * Для рассчета правильного значения нужно открыть prometheus, выполнить запрос `count(max_over_time({__name__=~".+"}[1h]))`, после чего добавить запас "на глаз" (обычно 20-50%). Если prometheus открывать не удобно, то вот готовый скрипт: `curl -s 'http://'$(kubectl -n kube-prometheus get pod/prometheus-main-0 -o json | jq '.status.podIP' -r)':9090/api/v1/query?query=count(max_over_time(%7B__name__%3D~%22.%2B%22%7D%5B1h%5D))' | jq '.data.result[0].value[1]' -r`. Так же информация о количестве метрик есть в `prometheus_tsdb_head_series` и `sum(scrape_samples_post_metric_relabeling)` (по ним можно рисовать графики).
    * Значения для метрик собираются каждые 30 секунд, каждая метрика занимает примерно два байта, соответственно объем данных за сутки рассчитывается по следующей формуле: `estimatedNumberOfMetrics / 30 секунд * 3600 * 24 * 2 байта / 1024 / 1024`. Диск заказывается с запасом 30% так, чтобы хватило на `retentionDays` (с округлением до целого количества гигабайт в большую сторону).
    * Значение request по памяти для pod'а вычисляется из расчета 8Кб на каждую метрику, limit — 16Кб на метрику.
* `storageClassName` — имя storageClass'а, который использовать.
    * Если не указано — используется или `global.storageClassName`, или `global.discovery.defaultStorageClassName`, а если и они не указаны — данные сохраняются в emptyDir.
    * Если указать `false` — будет форсироваться использование emptyDir'а.
* `longtermStorageClassName` — имя storageClass'а, который использовать для Longterm Prometheus.
    * Если не указано — используется или `prometheus.storageClassName` от основного Prometheus, или `global.storageClassName`, или `global.discovery.defaultStorageClassName`, а если и они не указаны — данные сохраняются в emptyDir.
    * Если указать `false` — будет форсироваться использование emptyDir'а.
* `userPassword` — пароль пользователя `user` (генерируется автоматически, но можно изменять).
* `adminPassword` — пароль пользователя `admin` (генерируется автоматически, но можно изменять).
* `madisonAuthKey` — ключ для отправки алертов в Madison ([подробнее об интеграции с Madison](docs/MADISON.md)).
* `longtermRetentionDays` — сколько дней хранить данные в longterm Prometheus. Если параметр не установлен, то longterm Prometheus не создается.
* `madisonSelfSetupKey` — ключ для автоматической регистрации в Madison, значение по-умолчанию лежит в [values.yaml](values.yaml).
    * Если указать `false` — автоматическая регистрация будет отключена.
* `additionalAlertmanagers` — массив, позволяющий указать дополнительные Endpoints, указывающие на внешние Alertmanager'ы. Endpoints объект необходимо создать вручную. **ВНИМАНИЕ!** В данный момент поддерживается только plain HTTP протокол.
    * Пример массива:
      ```yaml
      additionalAlertmanagers:
      - namespace: default
        name: external-alertmanager
        port: 8080
        pathPrefix: "/"
      ```
    * Пример Endpoint'а:
        ```yaml
        kind: Endpoints
        apiVersion: v1
        metadata:
          name: external-alertmanager
          namespace: default
        subsets:
          - addresses:
              - ip: 1.2.3.4
            ports:
              - port: 8080
        ```
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"node-role/system","operator":"Exists"}]` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `certificateForIngress` — выбираем, какой типа сертификата использовать для pormetheus/grafana.
    * `certmanagerClusterIssuerName` — указываем, какой ClusterIssuer использовать для prometheus/grafana (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По-умолчанию `letsencrypt`.
    * `customCertificateSecretName` — указываем имя secret'а в namespace `antiopa`, который будет использоваться для prometheus/grafana (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets).
        * По-умолчанию `false`.
        * При указании этого параметра не забудьте выставить `certmanagerClusterIssuerName` в значение `false`.
    * Если вы хотите отключить https, то оба параметра необходимо выставить в `false`.

### Пример конфига

```yaml
prometheus: |
  userPassword: xxxxxx
  adminPassword: yyyyyy
  retentionDays: 7
  estimatedNumberOfMetrics: 200000
  storageClassName: rbd
  nodeSelector:
    node-role/monitoring: ""
  tolerations:
  - key: node-role/monitoring
    operator: Exists
```
