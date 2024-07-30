---
title: "The runtime-audit-engine module: advanced usage"
description: Examples of deeper use of the runtime-audit-engine Deckhouse module.
---

{% raw %}

## Enabling debugging logs

### Falco

By default, the log level for `Falco` is set to `debug`.

### Falcosidekick

By default, the debug logging for `Falcosidekick` is disabled.

To enable debugging logging set the `spec.settings.debugLogging` parameter to `true`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: runtime-audit-engine
spec:
  enabled: true
  settings:
    debugLogging: true
```

## Viewing metrics

You can use the PromQL query `falco_events{}` to get metrics:

```shell
kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
  curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
```

We will add Grafana dashboard in the future for viewing metrics.

## Emulating a Falco event

You can use the [event-generator](https://github.com/falcosecurity/event-generator) CLI utility to generate a Falco events.

`event-generator` can generate a variety of suspect actions(syscalls, k8s audit events, ...).

Use the following command to run all events with the Pod in Kubernetes cluster:

```shell
kubectl run falco-event-generator --image=falcosecurity/event-generator run
```

If you need to implement an action, use this [guide](https://github.com/falcosecurity/event-generator/blob/main/events/README.md).

## Emulating a Falcosidekick event

You can use the [Falcosidekick](https://github.com/falcosecurity/falcosidekick) `/test` HTTP endpoint to send a test event to all enabled outputs.

- Get a list of Pods in `d8-runtime-audit-engine` namespace:

  ```shell
  kubectl -n d8-runtime-audit-engine get pods
  ```

  Example of the output:

  ```text
  NAME                         READY   STATUS    RESTARTS   AGE
  runtime-audit-engine-4cpjc   4/4     Running   0          3d12h
  runtime-audit-engine-rn7nj   4/4     Running   0          3d12h
  ```

- Get `runtime-audit-engine-4cpjc` Pod IP address:

  ```shell
  export POD_IP=$(kubectl -n d8-runtime-audit-engine get pod runtime-audit-engine-4cpjc --template '{{.status.podIP}}')
  ```

- Create a debug event, by making a query:

  ```shell
  kubectl run curl --image=curlimages/curl curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" $POD_IP:2801/test
  ```

- Check a debug event metric:

  ```shell
  kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus --  \
    curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
  ```

- Example of the output part:

  ```json
  {
    "metric": {
      "__name__": "falco_events",
      "container": "kube-rbac-proxy",
      "instance": "192.168.199.60:4212",
      "job": "runtime-audit-engine",
      "node": "dev-master-0",
      "priority": "Debug",
      "rule": "Test rule",
      "tier": "cluster"
    },
    "value": [
      1687150913.828,
      "2"
    ]
  }
  ```

{% endraw %}
