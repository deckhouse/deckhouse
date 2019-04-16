Модуль prometheus
=======

Модуль устанавливает [prometheus](https://prometheus.io/) (используя модуль [prometheus-operator](../200-prometheus-operator/)) и полностью его настраивает!

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
* `storageClassName` — имя storageClass'а, который использовать.
    * Если не указано — используется или `global.storageClassName`, или `global.discovery.defaultStorageClassName`, а если и они не указаны — данные сохраняются в emptyDir.
* `longtermStorageClassName` — имя storageClass'а, который использовать для Longterm Prometheus.
    * Если не указано — используется или `prometheus.storageClassName` от основного Prometheus, или `global.storageClassName`, или `global.discovery.defaultStorageClassName`, а если и они не указаны — данные сохраняются в emptyDir.
* `userPassword` — пароль пользователя `user` (генерируется автоматически, но можно изменять).
* `adminPassword` — пароль пользователя `admin` (генерируется автоматически, но можно изменять).
* `longtermRetentionDays` — сколько дней хранить данные в longterm Prometheus.
  * По-умолчанию `1095`.
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
* `vpa`
    * `maxCPU` — максимальная граница CPU requests/limits, выставляемая VPA контроллером. Если не прописано в конфиге, подбирается автоматически, исходя из максимально возможного количества подов на нодах.
    * `maxMemory` — максимальная граница Memory requests/limits, выставляемая VPA контроллером. Если не прописано в конфиге, подбирается автоматически, исходя из максимально возможного количества подов на нодах.
    * `updateMode` — режим обновления Pod'ов.
        * По-умолчанию `Initial`, но возможно поставить `Auto` или `Off`.

### Пример конфига

```yaml
prometheus: |
  userPassword: xxxxxx
  adminPassword: yyyyyy
  retentionDays: 7
  storageClassName: rbd
  nodeSelector:
    node-role/monitoring: ""
  tolerations:
  - key: node-role/monitoring
    operator: Exists
```
