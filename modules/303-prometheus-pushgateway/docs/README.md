---
title: "The Prometheus Pushgateway module"
---

This module installs [Prometheus Pushgateway](https://github.com/prometheus/pushgateway) into the cluster. It gets metrics from the app and pushes them to Prometheus.

[Learn more](https://prometheus.io/docs/practices/pushing/) about when to use `Prometheus Pushgateway`.
[Learn how](https://prometheus.io/docs/instrumenting/pushing/) to use `Prometheus Pushgateway`.

{% raw %}

#### Example of using PushGateway

PushGateway address: `http://first.kube-prometheus-pushgateway:9091`.

##### Pushing a metric using curl

```shell
# echo "test_metric 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/app
```

The metrics will be available in Prometheus in 30 seconds (after the data are scraped):

```
test_metric{instance="10.244.1.155:9091",job="app",pushgateway="first"} 3.14
```

**Caution!** The job value must be unique in Prometheus to preserve the consistency of the existing graphs and alerts. Use the following query to get a list of all existing jobs:  `count({__name__=~".+"}) by (job)`.

##### Deleting all metrics of a group `{instance="10.244.1.155:9091",job="app"}` using curl

```shell
# curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/app/instance/10.244.1.155:9091
```

Since PushGateway stores the scraped metrics in memory, **all metrics will be lost when the Pod is restarted**.
{% endraw %}
