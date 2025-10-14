---
title: "Advanced usage"
description: Examples of deeper use of the runtime-audit-engine Deckhouse module.
---


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

You can use the PromQL query `falcosecurity_falcosidekick_falco_events_total{}` to get metrics:

```shell
d8 k -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
  curl -s "http://127.0.0.1:9090/api/v1/query?query=falcosecurity_falcosidekick_falco_events_total" | jq
```

We will add Grafana dashboard in the future for viewing metrics.

## Emulating a Falco event

You can use the event-generator CLI utility to generate a Falco events.

`event-generator` can generate a variety of suspect actions(syscalls, k8s audit events, ...).

Use the following command to run all events with the Pod in Kubernetes cluster:

```shell
d8 k run falco-event-generator --image=falcosecurity/event-generator run
```

## Emulating a Falcosidekick event

You can use the [Falcosidekick](https://github.com/falcosecurity/falcosidekick) `/test` HTTP endpoint to send a test event.

- Create a debug event, by executing a command:

  ```shell
  nsenter -t $(pidof falcosidekick) curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" http://localhost:2801/test
  ```

- Check a debug event metric:

  ```shell
  d8 k -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
    curl -s "http://127.0.0.1:9090/api/v1/query?query=falcosecurity_falcosidekick_falco_events_total" \
    | jq '.data.result.[] | select (.metric.priority_raw == "debug")'
  ```

- Example of the output part:

  ```json
  {
    "metric": {
      "__name__": "falcosecurity_falcosidekick_falco_events_total",
      "container": "kube-rbac-proxy",
      "hostname": "falcosidekick",
      "instance": "192.168.208.7:4212",
      "job": "runtime-audit-engine",
      "node": "dev-master-0",
      "priority": "1",
      "priority_raw": "debug",
      "rule": "Test rule",
      "source": "internal",
      "tier": "cluster"
    },
    "value": [
      1744234729.799,
      "1"
    ]
  }
  ```

