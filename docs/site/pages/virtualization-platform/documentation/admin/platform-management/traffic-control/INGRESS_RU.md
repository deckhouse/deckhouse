---
title: "Управление входящим трафиком"
permalink: ru/virtualization-platform/documentation/admin/platform-management/traffic-control/ingress.html
lang: ru
---

## Возможности

Для обеспечения настроек доступа к виртуальным машинам извне, можно использовать возможности модуля ingress-nginx,
который устанавливает и управляет `Ingress` контроллерами, использующими NGINX в качестве обратного прокси-сервера и балансировщика нагрузки.

Если узлов для размещения Ingress-контроллера больше одного, он устанавливается в отказоустойчивом режиме 
и учитывает все особенности реализации инфраструктуры облаков и bare metal, а также кластеров Kubernetes различных типов.

Поддерживает запуск и раздельное конфигурирование одновременно нескольких NGINX Ingress контроллеров — 
один основной и сколько угодно дополнительных. Например, это позволяет отделять внешние и intranet Ingress-ресурсы приложений.

### Варианты терминирования трафика

Трафик к nginx-ingress может быть отправлен:
- напрямую без внешнего балансировщика;
- через внешний LoadBalancer. В том числе поддерживаются:
  - Qrator,
  - Cloudflare,
  - AWS LB,
  - GCE LB,
  - ACS LB,
  - Yandex LB,
  - OpenStack LB.

### Терминация HTTPS

Модуль позволяет управлять для каждого из NGINX Ingress контроллера политиками безопасности HTTPS, в частности:

- параметрами HSTS;
- набором доступных версий SSL/TLS и протоколов шифрования.

Также модуль интегрирован с модулем cert-manager, при взаимодействии с которым возможны автоматический заказ SSL-сертификатов и их дальнейшее использование NGINX Ingress контроллерами.

### Мониторинг и статистика

В нашей реализации `ingress-nginx` добавлена система сбора статистики в Prometheus с множеством метрик:
- по длительности времени всего ответа и апстрима отдельно;
- кодам ответа;
- количеству повторов запросов (retry);
- размерам запроса и ответа;
- методам запросов;
- типам `content-type`;
- географии распределения запросов и т. д.

Данные доступны в нескольких разрезах:
- по `namespace`;
- `vhost`;
- `ingress`-ресурсу;
- `location` (в nginx).

Все графики собраны в виде удобных досок в Grafana, при этом есть возможность drill-down'а по графикам:
при просмотре, например, статистики в разрезе namespace есть возможность, нажав на ссылку на dashboard в Grafana,
углубиться в статистику по `vhosts` в этом `namespace` и т. д.

#### Основные принципы сбора статистики

1. На каждый запрос на стадии `log_by_lua_block` вызывается наш модуль, который рассчитывает необходимые данные и складывает их в буфер (у каждого nginx worker'а свой буфер).
2. На стадии `init_by_lua_block` для каждого nginx worker'а запускается процесс, который раз в секунду асинхронно отправляет данные в формате `protobuf` через TCP socket в `protobuf_exporter` (наша собственная разработка).
3. `protobuf_exporter` запущен sidecar-контейнером в поде с Ingress контроллером, принимает сообщения в формате `protobuf`, разбирает, агрегирует их по установленным нами правилам и экспортирует в формате для Prometheus.
4. Prometheus каждые 30 секунд scrape'ает как сам Ingress контроллер (там есть небольшое количество нужных нам метрик), так и protobuf_exporter, на основании этих данных все и работает!

#### Какая статистика собирается и как она представлена

У всех собираемых метрик есть служебные лейблы, позволяющие идентифицировать экземпляр контроллера: `controller`, `app`, `instance` и `endpoint` (они видны в `/prometheus/targets`).

* Все метрики (кроме geo), экспортируемые protobuf_exporter'ом, представлены в трех уровнях детализации:
  * `ingress_nginx_overall_*` — «вид с вертолета», у всех метрик есть лейблы `namespace`, `vhost` и `content_kind`;
  * `ingress_nginx_detail_*` — кроме лейблов уровня overall, добавляются `ingress`, `service`, `service_port` и `location`;
  * `ingress_nginx_detail_backend_*` — ограниченная часть данных, собирается в разрезе по бэкендам. У этих метрик, кроме лейблов уровня detail, добавляется лейбл `pod_ip`.

* Для уровней overall и detail собираются следующие метрики:
  * `*_requests_total` — counter количества запросов (дополнительные лейблы — `scheme`, `method`);
  * `*_responses_total` — counter количества ответов (дополнительный лейбл — `status`);
  * `*_request_seconds_{sum,count,bucket}` — histogram времени ответа;
  * `*_bytes_received_{sum,count,bucket}` — histogram размера запроса;
  * `*_bytes_sent_{sum,count,bucket}` — histogram размера ответа;
  * `*_upstream_response_seconds_{sum,count,bucket}` — histogram времени ответа upstream'а (используется сумма времен ответов всех upstream'ов, если их было несколько);
  * `*_lowres_upstream_response_seconds_{sum,count,bucket}` — то же самое, что и предыдущая метрика, только с меньшей детализацией (подходит для визуализации, но не подходит для расчета quantile);
  * `*_upstream_retries_{count,sum}` — количество запросов, при обработке которых были retry бэкендов, и сумма retry'ев.

* Для уровня overall собираются следующие метрики:
  * `*_geohash_total` — counter количества запросов с определенным geohash (дополнительные лейблы — `geohash`, `place`).

* Для уровня detail_backend собираются следующие метрики:
  * `*_lowres_upstream_response_seconds` — то же самое, что аналогичная метрика для overall и detail;
  * `*_responses_total` — counter количества ответов (дополнительный лейбл — `status_class`, а не просто `status`);
  * `*_upstream_bytes_received_sum` — counter суммы размеров ответов бэкенда.

## Создание контроллера

Чтобы создать NGINX Ingress контроллер на bare metal, примените следующий ресурс `IngressNginxController`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostWithFailover
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
EOF
```

### Внешний балансировщик

Альтернативно, при ипользовании внешнего балансировщика (например, Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp и др.) на bare metal
можно создать NGINX Ingress контроллер следующим образом:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
EOF
```

### Балансировщик MetalLB в режиме BGP

{% alert level="warning" %} Функция доступна только в Enterprise Edition {% endalert %}

Для создания контроллера на bare metal с балансировщиком MetalLB в режиме BGP:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
EOF
```

В случае использования MetalLB его speaker-поды должны быть запущены на тех же узлах, что и поды Ingress–контроллера.

Контроллер должен получать реальные IP-адреса клиентов — поэтому его Service создается
с параметром `externalTrafficPolicy: Local` (запрещая межузловой SNAT), и для удовлетворения данного параметра
MetalLB speaker анонсирует этот Service только с тех узлов, где запущены целевые поды.

Таким образом, для данного примера конфигурация модуля metallb должна быть такой:

```yaml
metallb:
  speaker:
    nodeSelector:
      node-role.deckhouse.io/frontend: ""
    tolerations:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: frontend
```
