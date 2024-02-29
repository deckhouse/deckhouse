---
title: "Модуль ingress-nginx"
---

Устанавливает и управляет [NGINX Ingress controller](https://github.com/kubernetes/ingress-nginx) с помощью Custom Resources. Если узлов для размещения Ingress-контроллера больше одного, он устанавливается в отказоустойчивом режиме и учитывает все особенности реализации инфраструктуры облаков и bare metal, а также кластеров Kubernetes различных типов.

Поддерживает запуск и раздельное конфигурирование одновременно нескольких NGINX Ingress controller'ов — один **основной** и неограниченное количество **дополнительных**. Например, это позволяет отделять внешние и intranet Ingress-ресурсы приложений.

## Варианты терминирования трафика

Трафик к nginx-ingress может быть отправлен несколькими способами:
* напрямую без внешнего балансировщика;
* через внешний LoadBalancer, в том числе поддерживаются:
  - Qrator,
  - Cloudflare,
  - AWS LB,
  - GCE LB,
  - ACS LB,
  - Yandex LB,
  - OpenStack LB.

## Терминация HTTPS

Модуль позволяет управлять каждой NGINX Ingress controller политикой безопасности HTTPS, например:
* параметрами HSTS;
* набором доступных версий SSL/TLS и протоколов шифрования.

Также модуль интегрирован с модулем [cert-manager](../../modules/101-cert-manager/), при взаимодействии с которым, возможны автоматический заказ SSL-сертификатов и дальнейшее использование сертификатов NGINX Ingress controller.

## Мониторинг и статистика

В реализации `ingress-nginx` добавлена система сбора статистики в Prometheus со следующими метриками:
* длительностью времени ответа и upstream отдельно;
* кодами ответа;
* количеством повторов запросов (retry);
* размерами запроса и ответа;
* методами запросов;
* типами `content-type`;
* географией распределения запросов и т. д.

Данные доступны в нескольких параметрах:
* по `namespace`;
* `vhost`;
* `ingress`-ресурсу;
* `location` (в nginx).

Графики собраны в виде dashboard Grafana, при этом есть возможность drill-down по графикам. Например, при просмотре статистики namespace, можно нажать ссылку на dashboard Grafana, углубиться в статистику по `vhosts` в соответствущем `namespace` и т. д.

## Статистика

### Основные принципы сбора статистики

1. На каждый запрос стадии `log_by_lua_block` вызывается модуль, который рассчитывает необходимые данные и складывает их в буфер (у каждого nginx worker собственный буфер).
2. На стадии `init_by_lua_block` для каждого nginx worker запускается процесс, который 1 раз в секунду асинхронно отправляет данные в формате `protobuf` через TCP socket в `protobuf_exporter` (разработка DKP).
3. `protobuf_exporter` запущен sidecar-контейнером в поде с ingress-controller, где принимает сообщения в формате `protobuf`, разбирает, агрегирует их по установленным правилам и экспортирует в формате для Prometheus.
4. Prometheus каждые 30 секунд совершает scrape ingress-controller (там есть небольшое количество нужных нам метрик) и protobuf_exporter, и на основании этих данных все работает.

### Какая статистика собирается и как она представлена

У всех собираемых метрик присутствуют служебные лейблы, позволяющие идентифицировать экземпляр контроллера: `controller`, `app`, `instance` и `endpoint` (они видны в `/prometheus/targets`).

* Все метрики (кроме geo), экспортируемые protobuf_exporter, представлены в трех уровнях детализации:
  * `ingress_nginx_overall_*` — «вид с вертолета», у всех метрик есть лейблы `namespace`, `vhost` и `content_kind`;
  * `ingress_nginx_detail_*` — кроме лейблов уровня `overall`, добавляются `ingress`, `service`, `service_port` и `location`;
  * `ingress_nginx_detail_backend_*` — ограниченная часть данных, собирается в разрезе по бэкендам. У этих метрик, кроме лейблов уровня `detail`, добавляется лейбл `pod_ip`.

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
