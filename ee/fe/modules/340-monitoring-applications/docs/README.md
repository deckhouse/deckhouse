---
title: "The monitoring-applications module"
---

## Description

The module collects information about applications running in the cluster and configures for them:
* a set of Dashboards for Grafana (dashboards found on the Internet; no guarantees of smooth operation),
* PrometheusRules for Prometheus,
* ServiceMonitor for Prometheus.

You can also activate applications explicitly via the [configuration](configuration.html).

## Dashboard availability in Grafana

Dashboards are only available for applications that were either discovered by auto-searching for the `prometheus.deckhouse.io/target` service label or explicitly specified via the `enabledApplications` parameter.

## How do I collect application metrics?

> The method of collecting metrics provided below is intended for cases when the application is in the list of [supported applications](configuration.html#parameters). If the target application is **not in the list** of [supported applications](configuration.html#parameters), you need to use a [different method](../prometheus/faq.html#how-do-I-collect-metrics-from-applications) to collect metrics via the [monitoring-custom](../monitoring-custom/) module.

1. Attach the `prometheus.deckhouse.io/target` label to the Service you want to monitor. In the label, you must specify the name of the application from the [list](configuration.html#parameters).
2. Set the `http-metrics` and `https-metrics` name to the port that will be used for collecting metrics in order to connect to it over HTTP or HTTPS, respectively.
If it is not feasible for some reason, use the following annotations: `prometheus.deckhouse.io/port: port_number` to set the port number, and `prometheus.deckhouse.io/tls: "true"` if the metrics are collected over HTTPS.
3. Specify additional annotations to fine-tune the monitoring:
    * `prometheus.deckhouse.io/path` — the path to collect metrics (default: `/metrics`);
    * `prometheus.deckhouse.io/query-param-$name` — the $name=$value argument for the GET query (default: ``)
    * `prometheus.deckhouse.io/allow-unready-pod` — allows collecting metrics for pods in any state (by default, Prometheus scrapes metrics from the Ready pods only).
    * `prometheus.deckhouse.io/sample-limit` — sample limit for a Pod (refer to the table above to find out the default sample limit for an application).

Click [here](../prometheus/faq.html) to learn more about application monitoring..
