Модуль prometheus
=======

Модуль устанавливает [prometheus](https://prometheus.io/) (используя модуль [operator-prometheus](../200-operator-prometheus/)) и полностью его настраивает!

Модуль устанавливает два экземпляра Prometheus:
* **main** — основной Prometheus, который выполняет scrape каждые 30 секунд. Именно он обрабатывает все правила, шлет алерты и является основным источником данных.
* **longterm** — дополнительный Prometheus, который выполняет scrape данных из main каждые 5 минут. Используется для продолжительного хранения истории и для отображения больших промежутков времени.

Когда данный модуль установлен на Kubernetes 1.11 и имеет storage class, поддерживающий [автоматическое расширение диска](https://kubernetes.io/blog/2018/07/12/resizing-persistent-volumes-using-kubernetes/), в случае, если в prometheus не будет помещаться данные, то диск будет автоматически расширяться. В ином случае придет алерт о том, что дисковое пространство в Prometheus заканчивается.

Ресурсы cpu и memory автоматически выставляются при пересоздании пода на основе истории потребления, благодаря модулю [Vertical Pod Autoscaler](../302-vertical-pod-autoscaler). Так же, благодаряю кешированию запросов к Prometheus с помощью trickster, потребление памяти Prometheus'ом сильно сокращается.

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

При установке можно ничего не настраивать.

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
    * По-умолчанию `15`.
    * Работает совместно с параметром `retentionGigabytes`.
* `retentionGigabytes` — сколько гигабайт хранить.
    * По-умолчанию `30` гигабайт.
    * Работает совместно с параметром `retentionDays`.
* `storageClass` — имя storageClass'а, который использовать.
    * Если не указано — используется или `global.storageClass`, или `global.discovery.defaultStorageClass`, а если и они не указаны — данные сохраняются в emptyDir.
* `longtermStorageClass` — имя storageClass'а, который использовать для Longterm Prometheus.
    * Если не указано — используется или `prometheus.storageClass` от основного Prometheus, или `global.storageClass`, или `global.discovery.defaultStorageClass`, а если и они не указаны — данные сохраняются в emptyDir.
* `password` — пароль для http-авторизации для пользователя `admin` (генерируется автоматически, но можно менять)
    * Используется если не включен модуль `user-authn`.
* `longtermRetentionDays` — сколько дней хранить данные в longterm Prometheus.
  * По-умолчанию `1095`.
  * Работает совместно с параметром `longtermRetentionGigabytes`.
* `longtermRetentionGigabytes` — сколько гигабайт хранить.
    * По-умолчанию `30` гигабайт.
    * Работает совместно с параметром `longtermRetentionDays`.
* `madisonAuthKey` — ключ для отправки алертов в Madison ([подробнее об интеграции с Madison](docs/MADISON.md)).
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
* `grafana` - настройки для инсталяции Grafana.
    * `storageClass` — имя storageClass'а, который использовать для Grafana.
       * Если не указано — используется или `prometheus.storageClass` от основного Prometheus, или `global.storageClass`, или `global.discovery.defaultStorageClass`, а если и они не указаны — данные сохраняются в emptyDir.
    * `customPlugins` - список дополнительных [plug-in'ов](https://grafana.com/grafana/plugins) для Grafana. Необходимо указать в качестве значения список имен плагинов из официального репозитория.
       * Пример добавления plug-in'ов для возможности указания в качестве datasource clickhouse и панели flow-chart:
           ```yaml
           grafana:
             customPlugins:
             - agenty-flowcharting-panel
             - vertamedia-clickhouse-datasource
           ```
* `ingressClass` — класс ingress контроллера, который используется для grafana/prometheus.
    * Опциональный параметр, по-умолчанию используется глобальное значение `modules.ingressClass`.
* `https` — выбираем, какой типа сертификата использовать для grafana/prometheus.
    * `mode` — режим работы HTTPS:
        * `Disabled` — в данном режиме grafana/prometheus будут работать только по http;
        * `CertManager` — grafana/prometheus будут работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
        * `CustomCertificate` — grafana/prometheus будут работать по https используя сертификат из namespace `antiopa`;
        * `OnlyInURI` — grafana/prometheus будет работать по http (подразумевая, что перед ними стоит внешний https балансер, который терминирует https) и все ссылки в `user-authn` будут генерироваться с https схемой.
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для grafana/prometheus (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По-умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `antiopa`, который будет использоваться для grafana/prometheus (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
        * По-умолчанию `false`.
* `vpa`
    * `maxCPU` — максимальная граница CPU requests, выставляемая VPA контроллером для pod'ов основного Prometheus.
        * Значение по-умолчанию подбирается автоматически, исходя из максимального количества подов, которое можно создать в кластере при текущем количестве узлов и их настройках. Подробнее [см. хук](hooks/detect_vpa_max).
    * `maxMemory` — максимальная граница Memory requests, выставляемая VPA контроллером для pod'ов основного Prometheus.
        * Значение по-умолчанию подбирается автоматически, исходя из максимального количества подов, которое можно создать в кластере при текущем количестве узлов и их настройках. Подробнее [см. хук](hooks/detect_vpa_max).
    * `longtermMaxCPU` — максимальная граница CPU requests, выставляемая VPA контроллером для pod'ов longterm Prometheus.
        * Значение по-умолчанию подбирается автоматически, исходя из максимального количества подов, которое можно создать в кластере при текущем количестве узлов и их настройках. Подробнее [см. хук](hooks/detect_vpa_max).
    * `longtermMaxMemory` — максимальная граница Memory requests, выставляемая VPA контроллером для pod'ов longterm Prometheus.
        * Значение по-умолчанию подбирается автоматически, исходя из максимального количества подов, которое можно создать в кластере при текущем количестве узлов и их настройках. Подробнее [см. хук](hooks/detect_vpa_max).
    * `updateMode` — режим обновления Pod'ов.
        * По-умолчанию `Initial`, но возможно поставить `Auto` или `Off`.
* `highAvailability` — ручное управление [режимом отказоустойчивости](/FEATURES.md#отказоустойчивость).
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфига

```yaml
prometheus: |
  password: xxxxxx
  retentionDays: 7
  storageClass: rbd
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```
