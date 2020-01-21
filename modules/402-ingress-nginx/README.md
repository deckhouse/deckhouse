Модуль ingress-nginx
====================
Модуль позволяет установить в кластер один или несколько [ingress-nginx controller](https://github.com/kubernetes/ingress-nginx/)'ов при помощи CRD. 

Конфигурация
------------

### Включение модуля

Модуль по-умолчанию **включен**. Для выключения добавьте в CM `d8-system/deckhouse`:

```yaml
ingressNginxEnabled: "false"
```

### Параметры модуля

* `defaultControllerVersion` — версия контроллера ingress-nginx, которая будет использоваться для всех контроллеров по-умолчанию, если не был задан параметр `controllerVersion` в IngressNginxController CRD. 
    * По-умолчанию `0.25`,
    * Доступные варианты: `0.25`, `0.26`.

Параметры ресурса IngressNginxController
----------------------------------------
Параметры указываются в поле `spec`.

**Обязательные параметры:**
* `ingressClass` — имя ingress-класса для обслуживания ingress nginx controller. При помощи данной опции можно создать несколько контроллеров для обслуживания одного ingress-класса. 
    * **Важно!** Если указать значение "nginx", то дополнительно будут обрабатываться ingress ресурсы без аннотации `kubernetes.io/ingress.class`.
* `inlet` — способ поступления трафика из внешнего мира.
    * `LoadBalancer` — устанавливается ingress controller и заказывается сервис с типом LoadBalancer. 

**Необязательные параметры:**
* `controllerVersion` — версия ingress-nginx контроллера;
    * По-умолчанию берется версия из настроек модуля.
    * Доступные варианты: `0.25`, `0.26`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет [использоваться автоматика](/README.md#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `loadBalancer` — секция настроек для inlet'а `LoadBalancer`:
    * `annotations` — аннотации, которые будут проставлены сервису для гибкой настройки балансировщика.
    * `sourceRanges` — список CIDR, которым разрешен доступ на балансировщик.
        * Облачный провайдер может не поддерживать данную опцию и игнорировать её. 
    * `behindL7Proxy` — включает обработку и передачу X-Forwarded-* заголовков.
        * **Внимание!** При использовании этой опции вы должны быть уверены, что запросы к ingress'у направляются только от доверенных источников.
    * `realIPHeader` — заголовок, из которого будет получен настоящий IP-адрес клиента.
        * По-умолчанию `X-Forwarded-For`.
        * Опция работает только при включении `behindL7Proxy`.
* `hsts` — bool, включен ли hsts.
    * По-умолчанию — выключен (`false`).
* `legacySSL` — bool, включены ли старые версии TLS. Также опция разрешает legacy cipher suites для поддержки старых библиотек и программ: [OWASP Cipher String 'C' ](https://www.owasp.org/index.php/TLS_Cipher_String_Cheat_Sheet). Подробнее [здесь](templates/ingress/configmap.yaml).
    * По-умолчанию включён только TLSv1.2 и самые новые cipher suites.
* `disableHTTP2` — bool, выключить ли HTTP/2.
    * По-умолчанию HTTP/2 включен (`false`).
* `underscoresInHeaders` — bool, разрешены ли нижние подчеркивания в хедерах. Подробнее [здесь](http://nginx.org/en/docs/http/ngx_http_core_module.html#underscores_in_headers). Почему не стоит бездумно включать написано [здесь](https://www.nginx.com/resources/wiki/start/topics/tutorials/config_pitfalls/#missing-disappearing-http-headers).
    * По-умолчанию `false`.
* `customErrors` — секция с настройкой кастомизации HTTP ошибок (если секция определена, то все параметры в ней являются обязательными, изменение любого параметра **приводит в перезапуску всех ingress-nginx контроллеров**);
    * `serviceName` — имя сервиса, который будет использоваться, как custom default backend.
    * `namespace` — имя namespace, в котором будет находится сервис, используемый, как custom default backend.
    * `codes` — список кодов ответа (массив), при которых запрос будет перенаправлятся на custom default backend.
* `config` — секция настроек ingress controller, в которую в формате `ключ: значение(строка)` можно записать [любые возможные опции](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/);
    * **Внимание!** Ошибка в указании опций может привести к отказу в работе ingress controller'а.
    * **Внимание!** Не рекомендуется использовать данную опцию, не гарантируется обратная совместимость или работоспособность ingress controller'а с использованием данной опции.
* `additionalHeaders` — дополнительные header'ы, которые будут добавлены к каждому запросу. Указываютсяв формате `ключ: значение(строка)`.


### Пример
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  realIPHeader: "CF-Connecting-IP"
  config:
    gzip-level: "4"
    worker-processes: "8"
  additionalHeaders:
    X-Different-Name: "true"  
    Host: "$proxy_host"
```

Примеры использования ресурса IngressNginxController
----------------------------------------------------


#### AWS (Network Load Balancer)
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

#### GCP
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
```

#### MetalLB с доступом только из внутренней сети
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    sourceRanges:
    - 192.168.0.0/24
```

Статистика
----------

### Основные принципы сбора статистики

1. На каждый запрос, на стадии `log_by_lua`, вызывается наш модуль, который рассчитывает необходимые данные и шлет их через [datagram socket](https://en.wikipedia.org/wiki/Network_socket#Datagram_socket) в `statsd`.
2. Вместо обычного `statsd` у нас в pod'е с ingress-controller'ом запущен sidecar-контейнер с [statsd_exporter'ом](https://github.com/prometheus/statsd_exporter), который принимает данные в формате `statsd`, разбирает, агрегирует их по установленным нами правилам и экспортирует в формате для Prometheus.
3. Prometheus каждые 30 секунд scrape'ает как сам ingress-controller (там есть небольшое количество нужных нам метрик), так и statsd_exporter, и на основании этих данных все и работает!

### Какую статистику мы собираем и как она представлена?

У всех собираемых метрик есть служебные лейблы, позволяющие идентифицировать экземпляр контроллера: `controller`, `app`, `instance` и `endpoint` (они видны в `/prometheus/targets`).

* Все метрики (кроме geo), экспортируемые statsd_exporter'ом, представлены в трех уровнях детализации:
    * `ingress_nginx_overall_*` — "вид с вертолета", у всех метрик есть лейблы `namespace`, `vhost` и `content_kind`.
    * `ingress_nginx_detail_*` — кроме лейблов уровня overall добавляются: `ingress`, `service`, `service_port` и `location`.
    * `ingress_nginx_detail_backend_*` — ограниченная часть данных, собирается в разрезе по бекендам. У этих метрик, кроме лейблов уровня detail, добавляестя лейбл `pod_ip`.

* Для уровней overall и detail собираются следующие метрики:
    * `*_requests_total` — counter количества запросов (дополнительные лейблы: `scheme`, `method`).
    * `*_responses_total` — counter количества ответов (дополнительные лейблы: `status`).
    * `*_request_seconds_{sum,count,bucket}` — histogram времени ответа.
    * `*_bytes_received_{sum,count,bucket}` — histogram размера запроса.
    * `*_bytes_sent_{sum,count,bucket}` — histogram размера ответа.
    * `*_upstream_response_seconds_{sum,count,bucket}` — histogram времени ответа upstream'а (используется сумма времен ответов всех upstream'ов, если их было несколько).
    * `*_lowres_upstream_response_seconds_{sum,count,bucket}` — тоже самое, что предыдущая метрика, только с меньшей детализацией (подходит для визуализации, но не подходит для расчета quantile).
    * `*_upstream_retries_{count,sum}` — количество запросов, при обработке которых были retry бекендов, и сумма retry'ев.

* Для уровня overall собираются следующие метрики:
    * `*_geohash_total` — counter количества запросов с определенным geohash (дополнительные лейблы: `geohash`, `place`).

* Для уровня detail_backend собираются следующие метрики:
    * `*_lowres_upstream_response_seconds` — тоже самое, что аналогичная метрика для overall и detail.
    * `*_responses_total` — counter количества ответов (дополнительный лейбл `status_class`, а не просто `status`).
    *  `*_upstream_bytes_received_sum` — counter суммы размеров ответов backend'а.
