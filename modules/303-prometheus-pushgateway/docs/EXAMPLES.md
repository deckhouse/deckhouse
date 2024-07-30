---
title: "The Prometheus Pushgateway module: examples"
---

## Example of the module configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus-pushgateway
spec:
  version: 1
  enabled: true
  settings:
    instances:
    - first
    - second
    - another
```

PushGateway address (from a container pod): `http://first.kube-prometheus-pushgateway:9091`.

## Pushing a metric

An example of pushing a metric using curl:

```shell
echo "test_metric{env="dev"} 3.14" | curl --data-binary @- http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp
```

The metrics will be available in Prometheus in 30 seconds (after the data are scraped). An example:

```text
test_metric{container="prometheus-pushgateway", env="dev", exported_job="myapp", 
    instance="10.244.1.155:9091", job="prometheus-pushgateway", pushgateway="prometheus-pushgateway", tier="cluster"} 3.14
```

{% alert %} The job name (`myapp` in the example) will be available in Prometheus in the label `exported_job`, and not `job` (because the label `job` already exists in Prometheus and is renamed when receiving metrics from PushGateway).
{% endalert %}

{% alert %} You may need to get a list of all available job names to choose a unique name (in order not to spoil existing graphs and alerts). Use the following query to get a list of all existing jobs: {% raw %}`count({__name__=~".+"}) by (job)`.{% endraw %}
{% endalert %}

## Deleting metrics

An example of deleting all metrics of a group `{instance="10.244.1.155:9091",job="myapp"}` using curl:

```shell
curl -X DELETE http://first.kube-prometheus-pushgateway:9091/metrics/job/myapp/instance/10.244.1.155:9091
```

Since PushGateway stores the scraped metrics in memory, **all metrics will be lost when the Pod is restarted**.
