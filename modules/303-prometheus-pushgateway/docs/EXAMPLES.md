---
title: "THe Prometheus Pushgateway module: examples"
---

## Example of the module configuration

```yaml
prometheusPushgatewayEnabled: "true"
prometheusPushgateway: |
  instances:
  - first
  - second
  - another
```

{% raw %}

PushGateway address: `http://first.kube-prometheus-pushgateway:9091`.

## Pushing a metric using curl

```shell
# echo "test_metric 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/app
```

The metrics will be available in Prometheus in 30 seconds (after the data are scraped):

```text
test_metric{instance="10.244.1.155:9091",job="app",pushgateway="first"} 3.14
```

**Caution!** The job value must be unique in Prometheus to preserve the consistency of the existing graphs and alerts. Use the following query to get a list of all existing jobs: `count({__name__=~".+"}) by (job)`.

## Deleting all metrics of a group `{instance="10.244.1.155:9091",job="app"}` using curl

```shell
# curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/app/instance/10.244.1.155:9091
```

Since PushGateway stores the scraped metrics in memory, **all metrics will be lost when the Pod is restarted**.
{% endraw %}
