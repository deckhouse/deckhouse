---
title: "Модуль nginx-ingress"
---

Модуль устанавливает **один или несколько** [nginx-ingress controller'ов](https://github.com/kubernetes/ingress-nginx/) и учитывает все особенности интеграции с кластерами Kubernetes различных типов.

Дополнительная информация
-------------------------
* Видео-объяснение ([часть 1](https://www.youtube.com/watch?v=BS9QrmH6keI), [часть 2](https://www.youtube.com/watch?v=_ZG8umyd0B4)) о модуле и его настройках
* [Видео-объяснение](https://www.youtube.com/watch?v=IQac_TgiSao) про графики и как ими пользоваться

Конфигурация
------------

### Что нужно настраивать?

**Важно!** В абсолютном большинстве случаев **ничего не нужно настраивать**! Лучший конфиг — пустой конфиг.

### Параметры

Модуль поддерживает несколько контроллеров — один **основной** и сколько угодно **дополнительных**, для них можно указывать следующие параметры:
* `inlet` — способ поступления трафика из внешнего мира.
    * Определяется автоматически в зависимости от типа кластера (GCE и ACS — LoadBalancer, AWS — AWSClassicLoadBalancer, Manual — Direct; подробнее [здесь](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/templates/_helpers.tpl#L22-30))!
    * Поддерживаются следующие inlet'ы:
        * `LoadBalancer` (автоматически для `GCE` и `ACS`) — заказывает автоматом LoadBalancer.
        * `AWSClassicLoadBalancer` (автоматически для`AWS`) — заказывает автоматом LoadBalancer и включает proxy protocol, используется по умолчанию для AWS.
        * `Direct` (автоматически `Manual`) — pod'ы работают в host network, nginx слушает на 80 и 443 порту, хитрая схема с direct-fallback.
        * `NodePort` — создает сервис с типом NodePort, подходит в тех ситуациях, когда необходимо настроить "сторонний" балансировщик (например, использовать AWS Application Load Balancer, Qrator или  CloudFLare). Допустимый диапазон 30000-32767 (настраивается параметром `kube-apiserver --service-node-port-range`).
    * Очень наглядно посмотреть отличия четырех типов inlet'ов можно [здесь](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/templates/controller.yaml).
* `nodePortHTTP` — для inlet'ов с типом `NodePort` позволяет задать конкретный `nodePort` для публикации 80-го порта (по умолчанию ничего не указывается и kube-controller-manager подбирает случайный свободный).
* `nodePortHTTPS` — для inlet'ов с типом `NodePort` позволяет задать конкретный `nodePort` для публикации порта 443 (по умолчанию аналогично `nodePortHTTP`).
* `config.hsts` — bool, включен ли hsts.
    * По умолчанию — выключен (`false`).
* `config.legacySSL` — bool, включены ли старые версии TLS. Также опция разрешает legacy cipher suites для поддержки старых библиотек и программ: [OWASP Cipher String 'C' ](https://cheatsheetseries.owasp.org/cheatsheets/TLS_Cipher_String_Cheat_Sheet.html). Подробнее [здесь](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/templates/_template.config.tpl).
    * По умолчанию включён только TLSv1.2 и самые новые cipher suites.
* `config.disableHTTP2` — bool, выключить ли HTTP/2.
    * По умолчанию HTTP/2 включен (`false`).
* `config.ComputeFullForwardedFor` - bool, дополнять ли заголовок X-Forwarded-For прокси адресами или заменять его, нужно только в тех случаях, если на входе стоит балансер не под нашим управлением.
    * По умолчанию `compute-full-forwarded-for выключен (`false`).
* `config.underscoresInHeaders` — bool, разрешены ли нижние подчеркивания в хедерах. Подробнее [здесь](http://nginx.org/en/docs/http/ngx_http_core_module.html#underscores_in_headers). Почему не стоит бездумно включать написано [здесь](https://www.nginx.com/resources/wiki/start/topics/tutorials/config_pitfalls/#missing-disappearing-http-headers)
    * По умолчанию `false`.
* `config.setRealIPFrom` — список CIDR, с которых разрешено использовать заголовок `X-Forwarded-For` в качестве адреса клиента.
    * Список строк - именно **YAML list**, а не строка со значениями через запятую!
    * **Важно!** Так как nginx ingress (как и сам nginx) не поддерживает получение адреса клиента из `X-Forwarded-For`, при одновременном использовании proxy protocol параметр `config.setRealIPFrom` запрещено использовать для inlet'ов `Direct` и `AWSClassicLoadBalancer`.
* (только для дополнительных контроллеров) `name` (обязательно) — название контроллера.
    * Используется в качестве суффикса к имени namespace {% raw %}`kube-nginx-ingress-{{ $name }}` и в качестве суффикса к названию класса nginx `nginx-{{ $name }}` (того самого класса, который потом указывается в аннотации `kubernetes.io/ingress.class` к ingress ресурсам).{% endraw %}
* `customErrorsServiceName` - имя сервиса, который будет использоваться, как custom default backend.
    * **Важно!** Данный параметр является обязательным, если указан любой из других `customErrors` параметров.
    * **Важно!** Добавление, удаление или изменение параметра приводит к рестарту nginx'ов.
* `customErrorsNamespace` - имя namespace, в котором будет находится сервис, используемый, как custom default backend.
    * **Важно!** Данный параметр является обязательным, если указан любой из других `customErrors` параметров.
    * **Важно!** Добавление, удаление или изменение параметра приводит к рестарту nginx'ов.
* `customErrorsCodes` - список кодов ответа (массив), при которых запрос будет перенаправляться на custom default backend.
    * Список строк - именно **YAML list**, а не строка со значениями через запятую!
    * **Важно!** Данный параметр является обязательным, если указан любой из других `customErrors` параметров.
    * **Важно!** Добавление, удаление или изменение параметра приводит к рестарту nginx'ов.
* `enableIstioSidecar` — добавить к подам контроллера sidecar для контролирования трафика средствами Istio. Будут добавлены две аннотации: `sidecar.istio.io/inject: "true"` и `traffic.sidecar.istio.io/includeOutboundIPRanges: "<Service CIDR>"`, а непосредственно добавление контейнера в под осуществит [sidecar-injector от Istio](/modules/360-istio/usage.html#ingress).
    * **Важно!** Так как в режиме `Direct` pod'ы работают в host network, то механизм идентификации трафика для заворачивания в Istio, который основан на Service CIDR, несёт потенциальную опасность для других сервисов с ClusterIP. Для подобных inlet'ов поддержка Istio отключена.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/overview.html#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфига
{% raw %}

```yaml
nginxIngress: |
  config:
    hsts: true
  nodeSelector: false
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
  additionalControllers:
  - name: direct
    inlet: Direct
    config:
      hsts: true
    customErrorsServiceName: "error-backend"
    customErrorsNamespace: "default"
    customErrorsCodes:
    - 404
    - 502
    nodeSelector:
      node-role/example: ""
    tolerations:
    - key: dedicated
      operator: Equal
      value: example
  - name: someproject
    inlet: NodePort
    nodeSelector: false
    tolerations: false
  - name: foo
```
{% endraw %}

### Особенности использования дополнительных контроллеров

* Для каждого дополнительного контроллера обязательно указывается `name`, при этом разворачивается полная копия всего в отдельном namespace с названием `kube-nginx-ingress-<name>`.
* Дополнительные экземпляры контроллера работают с отдельным классом, который необходимо указывать в ingress ресурсах через аннотацию `kubernetes.io/ingress.class: "nginx-<name>"`.

### Нюансы использования canary аннотаций

Использование canary аннотаций в ingress-nginx сопровождают *нюансы*. Вот известный список:

1. Возможно использовать только по-одному Ingress с canary аннотациями на каждый не-Canary Ingress.
1. [Корректно](https://github.com/kubernetes/ingress-nginx/pull/4198) работают **только** Ingress'ы с одним path.
1. Не работают sticky sessions по canary/НЕ canary (каждый следующий запрос может попасть как в canary, так и в основную версию).

### Кастомные страницы ошибок

Для использования необходимо приложение, которое будет отдавать страницы указанных кодов ошибок. Отдаваемый HTML-файл должен включать в себя все ассеты (стили, графику, шрифты). Сервис должен принимать подключения на 80 порту. Данный сервис должен отдавать для 500 ошибок код ответа 500, для 502 ошибок, код 502 и так далее, если нет явных причин отдавать другие коды ответа.

Примеры использования
---------------------

### Bare Metal + Qrator

Кейс:
* Не production площадки (test, stage, etc) и инфраструктурные компоненты (prometheus, dashboard, etc) ходят напрямую.
* Все ресурсы production ходят через Qrator.

Способ реализации:
* Оставляем основной контроллер работать без измений.
* Указываем дополнительный контроллер с inlet `NodePort`.
* Опционально, в дополнительном контроллере указываем конкретные nodePort-порты для HTTP и HTTPS (`nodePortHTTP` и `nodePortHTTPS`).
* В ingress ресурсах прода указываем аннотацию `kubernetes.io/ingress.class: "nginx-qrator"`.
* Настраиваем Qrator, чтобы он отправлял трафик на "эфемерные" порты сервиса с типом NodePort. Если не указали конкретные порты (`nodePortHTTP`, `nodePortHTTPS`), то узнать, какие порты выбрал controller-manager, можно с помощью команды: `kubectl -n kube-nginx-ingress-qraror get svc nginx -o yaml`.

```
nginxIngress: |
  additionalControllers:
  - name: qrator
    inlet: NodePort
    nodePortHTTP: 30080
    nodePortHTTPS: 30443
    config:
      setRealIPFrom:
      - 87.245.197.192
      - 87.245.197.193
      - 87.245.197.194
      - 87.245.197.195
      - 87.245.197.196
      - 83.234.15.112
      - 83.234.15.113
      - 83.234.15.114
      - 83.234.15.115
      - 83.234.15.116
      - 66.110.32.128
      - 66.110.32.129
      - 66.110.32.130
      - 66.110.32.131
      - 130.117.190.16
      - 130.117.190.17
      - 130.117.190.18
      - 130.117.190.19
      - 185.94.108.0/24
```

### AWS + CloudFlare

Кейс:
* Большая часть production ресурсов, все не production ресурсы (test, stage, etc) и инфраструктурные компоненты (prometheus, dashboard, etc) ходят через обычный AWSClassicLoadBalancer.
* Однако часть production ресурсов надо отправить через CloudFront, а `setRealIPFrom` не поддерживается при использовании `AWSClassicLoadBalancer` (из-за несовместимости с proxy protocol).

Способ реализации:
* Оставляем основной контроллер работать без измений.
* Указываем дополнительный контроллер с inlet `NodePort`.
* Настраиваем CloudFlare, чтобы он отправлял трафик на адрес сервиса: `kubectl -n kube-nginx-ingress-cf get svc nginx -o yaml`

```
nginxIngress: |
  additionalControllers:
  - name: cf
    inlet: LoadBalancer
    config:
      setRealIPFrom:
      - 103.21.244.0/22
      - 103.22.200.0/22
      - 103.31.4.0/22
      - 104.16.0.0/12
      - 108.162.192.0/18
      - 131.0.72.0/22
      - 141.101.64.0/18
      - 162.158.0.0/15
      - 172.64.0.0/13
      - 173.245.48.0/20
      - 188.114.96.0/20
      - 190.93.240.0/20
      - 197.234.240.0/22
      - 198.41.128.0/17
```

### AWS + AWS Application Load Balancer

Кейс:
* У клиента уже есть сертификаты, заказанные в Amazon и их оттуда никуда не вытащищь.
* Не хочется делать несколько контроллеров и несколько LoadBalancer'ов в Amazon, чтобы сэкономить деньги.

Способ реализации:
* Будем использовать в качестве основной и единственной точки входа - `AWS Application Load Balancer`.
* Для этого перенастраиваем основной контроллер с inlet `NodePort`.
* Настраиваем в AWS - `Application Load Balancer`, чтобы он кидал трафик по "эфемерным" портам сервиса с типом `NodePort`: `kubectl -n kube-nginx-ingress get svc nginx -o yaml`.

```
nginxIngress: |
  inlet: NodePort
  config:
    setRealIPFrom:
    - 0.0.0.0/0
```

### AWS + AWS HTTP Classic Load Balancer

Кейс:
* Все ходит через обычный `AWS Classic Load Balancer`, но нужно заказать сертификат в Amazon, а его нельзя повесить на существующий `AWS Classic Load Balancer`.


Способ реализации:
* Оставляем основной контроллер работать без измений.
* Указываем дополнительный контроллер с inlet `NodePort`.
* Создаем (руками или через infra проект в gitlab) сколько необходимо сервисов (со специальными аннотациями для подключения сертификатов).

```
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-backend-protocol: http
    service.beta.kubernetes.io/aws-load-balancer-ssl-cert: arn:aws:acm:eu-central-1:206112445282:certificate/23341234d-7813-45e8-b249-123421351251234
    service.beta.kubernetes.io/aws-load-balancer-ssl-ports: "443"
  name: nginx-site-1
  namespace: kube-nginx-ingress-aws-http
spec:
  externalTrafficPolicy: Local
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
  - name: https
    port: 443
    protocol: TCP
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```


```
nginxIngress: |
  additionalControllers:
  - name: aws-http
    inlet: NodePort
    config:
      setRealIPFrom:
      - 0.0.0.0/0
```

### Bare Metal + несколько проектов, которые не должны быть аффилированны

Кейс:
* Есть основной проект и два дополнительных, но никто не должен знать что они принадлежат одним владельцам (хостятся на одной площадке).

Способ реализации:
* Выделяем основной контроллер на отдельные машины (ставим на них label и taint `node-role.deckhouse.io/frontend`).
* Создаем два дополнительных контроллера и выделенные для них машины (с label и taint `node-role/frontend-foo` и `node-role/frontend-bar`).

```
nginxIngress: |
  additionalControllers:
  - name: foo
    nodeSelector:
      node-role/frontend-foo: ""
    tolerations:
    - key: node-role/frontend-foo
      operator: Exists
  - name: bar
    nodeSelector:
      node-role/frontend-bar: ""
    tolerations:
    - key: node-role/frontend-bar
      operator: Exists
```

Статистика
----------

### Основные принципы работы статистики

1. На каждый запрос, на стадии `log_by_lua`, вызывается [наш модуль](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/images/controller/rootfs/etc/nginx/template/nginx.tmpl#L750), который [рассчитывает необходимые данные и шлет их по UDP](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/images/controller/rootfs/etc/nginx/lua/statsd.lua) в `statsd`.
2. Вместо обычного `statsd` у нас в pod'е с ingress-controller'ом запущен sidecar контейнер с [statsd_exporter'ом](https://github.com/prometheus/statsd_exporter), который принимает данные в формате `statsd`, разбирает и агрегирует их [[по установленным нами правилам](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/images/statsd-exporter/rootfs/etc/statsd_mapping.conf) и экспортирует в формате для Prometheus.
3. Prometheus каждые 30 секунд scrape'ает как сам ingress-controller (там есть небольшое количество нужных нам метрик), так и statsd_exporter, и на основании этих данных все и работает!

### Какая информация собирается и как она представлена?

* У всех собираемых метрик есть служебные лейблы, позволяющие идентифицировать экземпляр контроллера: `controller`, `app`, `instance` и `endpoint` (они видны в `/prometheus/targets`).
* Все метрики (кроме geo), экспортируемые statsd_exporter'ом, представлены в трех уровнях детализации:
    * `ingress_nginx_overall_*` — "вид с вертолета", у всех метрик есть лейблы `namespace`, `vhost` и `content_kind`.
    * `ingress_nginx_detail_*` — кроме лейблов уровня overall добавляются: `ingress`, `service`, `service_port` и `location`.
    * `ingress_nginx_detail_backend_*` — ограниченная часть данных, собирается в разрезе по бекендам. У этих метрик, кроме лейблов уровня detail, добавляестя лейбл `pod_ip`.
* Для уровней overall и detail собираются следующие метрики:
    * `..._requests_total` — counter количества запросов (дополнительные лейблы: `scheme`, `method`).
    * `..._responses_total` — counter количества ответов (дополнительные лейблы: `status`).
    * `..._request_seconds_{sum,count,bucket}` — histogram времени ответа.
    * `..._bytes_received_{sum,count,bucket}` — histogram размера запроса.
    * `..._bytes_sent_{sum,count,bucket}` — histogram размера ответа.
    * `..._upstream_response_seconds_{sum,count,bucket}` — histogram времени ответа upstream'а (используется сумма времен ответов всех upstream'ов, если их было несколько).
    * `..._lowres_upstream_response_seconds_{sum,count,bucket}` — тоже самое, что предыдущая метрика, только с меньшей детализацией (подходит для визуализации, но не подходит для расчета quantile).
    * `..._upstream_retries_{count,sum}` — количество запросов, при обработке которых были retry бекендов, и сумма retry'ев.
* Для уровня overall собираются следующие метрики:
    * `..._geohash_total` — counter количества запросов с определенным geohash (дополнительные лейблы: `geohash`, `place`).
* Для уровня detail_backend собираются следующие метрики:
    * `..._lowres_upstream_response_seconds` — то же самое, что аналогичная метрика для overall и detail.
    * `..._responses_total` — counter количества ответов (дополнительный лейбл `status_class`, а не просто `status`).
    *  `..._upstream_bytes_received_sum` — counter суммы размеров ответов backend'а.

Дополнительная информация
-------------------------

### Ключевые отличия в работе балансировщиков в разных Cloud

* При создании Service с `spec.type=LoadBalancer` Kubernetes создает сервис с типом `NodePort` и, дополнительно, лезет в клауд и настраивает балансировщик клауда, чтобы он бросал трафик на все узлы Kubernetes на определенные `spec.ports[*].nodePort` (генерятся рандомные в диапазоне `30000-32767`).
* В GCE и Azure балансировщик отправляет трафик на узлы сохраняя source адрес клиента. Если при создании сервиса в Kubernetes указать `spec.externalTrafficPolicy=Local`, то Kubernetes приходящий на узел трафик не будет раскидывать по всем узлам, на которых есть endpoint'ы, а будет кидать только на локальные endpoint'ы, находящиеся на этом узле, а если их нет — соединение не будет устанавливаться. Подробнее об этом [тут](https://kubernetes.io/docs/tutorials/services/source-ip/) и [особенно тут](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#preserving-the-client-source-ip).
* В AWS все интересней:
    * До версии Kubernetes 1.9 единственным типом LB, который можно было создать в AWS из Kubernetes, был Classic. При этом, по умолчанию создается `AWS Classic LoadBalancer`, который проксирует TCP трафик (так же на `spec.ports[*].nodePort`). Трафик при этом приходит не с адреса клиента, а с адресов LoadBalancer'а. И единственный способ узнать адрес клиента — включить proxy protocol (это можно сделать через [аннотацию сервиса в Kubernetes](https://github.com/kubernetes/legacy-cloud-providers/blob/master/aws/aws.go).
    * Начиная с версии Kubernetes 1.9 [можно заводить Network LoadBalancer'ы](https://kubernetes.io/docs/concepts/services-networking/service/#network-load-balancer-support-on-aws-alpha). Такой LoadBalancer работает аналогично Azure и GCE — отправляет трафик с сохранением source адреса клиента.

