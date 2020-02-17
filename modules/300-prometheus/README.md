---
title: "Модуль prometheus"
tags:
  - prometheus
type:
  - instruction
search: prometheus
---

Модуль prometheus
=======

Модуль устанавливает [prometheus](https://prometheus.io/) (используя модуль [operator-prometheus](../200-operator-prometheus/)) и полностью его настраивает!

Модуль устанавливает два экземпляра Prometheus:
* **main** — основной Prometheus, который выполняет scrape каждые 30 секунд (с помощью параметра `scrapeInterval` можно изменить это значение). Именно он обрабатывает все правила, шлет алерты и является основным источником данных.
* **longterm** — дополнительный Prometheus, который выполняет scrape данных из main каждые 5 минут. Используется для продолжительного хранения истории и для отображения больших промежутков времени.

Когда данный модуль установлен на Kubernetes 1.11 и имеет storage class, поддерживающий [автоматическое расширение диска](https://kubernetes.io/blog/2018/07/12/resizing-persistent-volumes-using-kubernetes/), в случае, если в prometheus не будет помещаться данные, то диск будет автоматически расширяться. В ином случае придет алерт о том, что дисковое пространство в Prometheus заканчивается.

Ресурсы cpu и memory автоматически выставляются при пересоздании пода на основе истории потребления, благодаря модулю [Vertical Pod Autoscaler](../302-vertical-pod-autoscaler). Так же, благодаря кешированию запросов к Prometheus с помощью trickster, потребление памяти Prometheus'ом сильно сокращается.

Дополнительная информация
-------------------------

* [Разработка правил для Prometheus (алертов и recording)](docs/PROMETHEUS_RULES_DEVELOPMENT.md)
* [Разработка графиков (Dashboard'ов для Grafana)](docs/GRAFANA_DASHBOARD_DEVELOPMENT.md)
* [Разработка target'ов для Prometheus (целей, которые мониторить)](docs/PROMETHEUS_TARGETS_DEVELOPMENT.md)
* [Кастомные правила и графики для конкретного проекта](docs/PROJECT_CUSTOMIZATION.md)

Конфигурация
------------

### Что нужно настраивать?

При установке можно ничего не настраивать.

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
* `longtermRetentionDays` — сколько дней хранить данные в longterm Prometheus.
  * По-умолчанию `1095`.
  * Работает совместно с параметром `longtermRetentionGigabytes`.
* `longtermRetentionGigabytes` — сколько гигабайт хранить.
    * По-умолчанию `30` гигабайт.
    * Работает совместно с параметром `longtermRetentionDays`.
* `auth` — опции, связанные с аутентификацией или авторизацией в приложении:
    * `externalAuthentication` - параметры для подключения внешней аутентификации (используется механизм Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/), работающей на основе модуля Nginx [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html).
         * `authURL` - URL сервиса аутентификации. Если пользователь прошел аутентификацию, сервис должен возвращать код ответа HTTP 200.
         * `authSignInURL` - URL, куда будет перенаправлен пользователь для прохождения аутентификации (если сервис аутентификации вернул код ответа HTTP отличный от 200).
    * `password` — пароль для http-авторизации для пользователя `admin` (генерируется автоматически, но можно менять)
         * Используется если не включен параметр `externalAuthentication`.
    * `allowedUserGroups` — массив групп, пользователям которых позволен доступ в grafana и prometheus. 
         * Используется если включен параметр `externalAuthentication` и модуль `user-authn`.
    * `whitelistSourceRanges` — массив CIDR, которым разрешено проходить авторизацию в grafana и prometheus.
    * `satisfyAny` — разрешает пройти только одну из аутентификаций. В комбинации с опцией whitelistSourceRanges позволяет считать авторизованными всех пользователей из указанных сетей без ввода логина и пароля.
* `grafana` - настройки для инсталляции Grafana.
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
    * При использовании этого параметра полностью переопределяются глобальные настройки `global.modules.https`.
    * `mode` — режим работы HTTPS:
        * `Disabled` — в данном режиме grafana/prometheus будут работать только по http;
        * `CertManager` — grafana/prometheus будут работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
        * `CustomCertificate` — grafana/prometheus будут работать по https используя сертификат из namespace `d8-system`;
        * `OnlyInURI` — grafana/prometheus будет работать по http (подразумевая, что перед ними стоит внешний https балансер, который терминирует https) и все ссылки в `user-authn` будут генерироваться с https схемой.
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для grafana/prometheus (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По-умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `d8-system`, который будет использоваться для grafana/prometheus (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
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
* `scrapeInterval` — с помощью данного параметра можно указать, как часто prometheus будет собирать метрики с таргетов. Evaluation Interval всегда равен scrapeInterval.
  * По-умолчанию `30s`.
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
