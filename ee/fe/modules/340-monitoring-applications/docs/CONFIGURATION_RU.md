---
title: "Модуль monitoring-applications: настройки"
---

<!-- SCHEMA -->

## Параметры

* `enabledApplications` — список приложений, который необходимо включить безотносительно результатов автоматического обнаружения. Формат — список строк.
  * Поддерживаемые приложения:

---
    | **Application** | **Grafana Dashboard**               | **PrometheusRule**                  |
    |-----------------|-------------------------------------|-------------------------------------|
    | consul          |                                     |                                     |
    | elasticsearch   | <span class="doc-checkmark"></span> |                                     |
    | etcd3           | <span class="doc-checkmark"></span> |                                     |
    | fluentd         |                                     |                                     |
    | loki            | <span class="doc-checkmark"></span> |                                     |
    | memcached       | <span class="doc-checkmark"></span> |                                     |
    | minio           |                                     |                                     |
    | mongodb         | <span class="doc-checkmark"></span> |                                     |
    | nats            | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> |
    | nginx           |                                     |                                     |
    | pgbouncer       | <span class="doc-checkmark"></span> |                                     |
    | php-fpm         | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> |
    | prometheus      | <span class="doc-checkmark"></span> |                                     |
    | rabbitmq        | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> |
    | redis           | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> |
    | sidekiq         | <span class="doc-checkmark"></span> |                                     |
    | trickster       |                                     |                                     |
    | grafana         |                                     |                                     |
    | uwsgi           | <span class="doc-checkmark"></span> |                                     |
