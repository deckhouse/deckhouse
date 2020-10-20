---
title: "Модуль monitoring-applications: конфигурация"
---

## Параметры

* `enabledApplications` — список приложений, который необходимо включить безотносительно результатов автоматического исследования.
  * Формат — список строк.
  * Поддерживаемые приложения:

    | **Application** | **Grafana Dashboard** | **PrometheusRule** | **Sample Limit** |
    | ------ |:------:|:------:|:------:|
    | consul        |                    |                    | 500 |
    | elasticsearch | <span class="doc-checkmark"></span> |                    | 5000 |
    | etcd3         | <span class="doc-checkmark"></span> |                    | 1000 |
    | fluentd       |                    |                    | 500 |
    | memcached     | <span class="doc-checkmark"></span> |                    | 2500 |
    | minio         |                    |                    | 500 |
    | mongodb       | <span class="doc-checkmark"></span> |                    | 1000 |
    | nats          | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> | 500 |
    | nginx         |                    |                    | 500 |
    | php-fpm       | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> | 1000 |
    | prometheus    | <span class="doc-checkmark"></span> |                    | 5000 |
    | rabbitmq      | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> | 2500 |
    | redis         | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> | 1000 |
    | sidekiq       | <span class="doc-checkmark"></span> |                    | 1000 |
    | trickster     |                    |                    | 1000 |
    | grafana       |                    |                    | 1000 |
    | uwsgi         | <span class="doc-checkmark"></span> |                    | 1000 |

