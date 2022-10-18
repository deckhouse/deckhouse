---
title: "The monitoring-applications module: configuration"
---

## Parameters

* `enabledApplications` — a list of applications to explicitly include in monitoring regardless of the results of the auto-discovery;
  * Format — a list of strings;
  * Supported applications:

    | **Application** | **Grafana Dashboard**               | **PrometheusRule**                  |
    |-----------------|-------------------------------------|-------------------------------------|
    | consul          |                                     |                                     |
    | elasticsearch   | <span class="doc-checkmark"></span> |                                     |
    | etcd3           | <span class="doc-checkmark"></span> |                                     |
    | fluentd         |                                     |                                     |
    | memcached       | <span class="doc-checkmark"></span> |                                     |
    | minio           |                                     |                                     |
    | mongodb         | <span class="doc-checkmark"></span> |                                     |
    | nats            | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> |
    | nginx           |                                     |                                     |
    | php-fpm         | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> |
    | prometheus      | <span class="doc-checkmark"></span> |                                     |
    | rabbitmq        | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> |
    | redis           | <span class="doc-checkmark"></span> | <span class="doc-checkmark"></span> |
    | sidekiq         | <span class="doc-checkmark"></span> |                                     |
    | trickster       |                                     |                                     |
    | grafana         |                                     |                                     |
    | uwsgi           | <span class="doc-checkmark"></span> |                                     |
