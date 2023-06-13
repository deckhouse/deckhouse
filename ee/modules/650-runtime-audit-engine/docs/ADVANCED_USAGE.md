---
title: "Module runtime-audit-engine: advanced usage"
---

## How to enable debugging logs?

### Falco
Get current log level:

```bash
kubectl -n d8-runtime-audit-engine get configmap runtime-audit-engine -o yaml | yq e '.data."falco.yaml"' - | yq .log_level
```

Set log level to `debug`:

```bash
kubectl -n d8-runtime-audit-engine edit configmap runtime-audit-engine
```

Find or add `log_level` field and set it to `debug`:

```yaml
log_level: debug
```

### Falcosidekick

If `DEBUG` environment variable is `"true"` then all outputs will print in stdout the payload they send.

Get current `DEBUG` environment variable value:

```bash
kubectl -n d8-runtime-audit-engine get daemonset runtime-audit-engine -o yaml | yq '.spec.template.spec.containers[] | select(.name == "falcosidekick") | .env[] | select(.name == "DEBUG") | .value'
```

Enable the debug mode:

```bash
kubectl -n d8-runtime-audit-engine edit daemonset runtime-audit-engine
```

Find or add `env` field for `falcosidekick` container and set `DEBUG` to `"true"`:

```yaml
env:
  - name: DEBUG
    value: "true"
```

## How to view metrics?

You can use the PromQL query `falco_events{}`:

```bash
kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus -- curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
```

## How to emulate a Falco event?

- You can use the [event-generator](https://github.com/falcosecurity/event-generator) CLI utility to generate a Falco events:

  Run all events with the Pod in Kubernetes cluster:

  ```bash
  kubectl run falco-event-generator --image=falcosecurity/event-generator run
  ```

- You can use the [Falcosidekick](https://github.com/falcosecurity/falcosidekick) http endpoint `/test` to send a test event to all enabled outputs:

  Get a list of pods in `d8-runtime-audit-engine` namespace:
  
  ```bash
  kubectl -n d8-runtime-audit-engine get pods
  ```
  
  ```
  NAME                         READY   STATUS    RESTARTS   AGE
  runtime-audit-engine-4cpjc   4/4     Running   0          3d12h
  runtime-audit-engine-rn7nj   4/4     Running   0          3d12h
  ```
  
  Forward port from `runtime-audit-engine-4cpjc` pod to localhost:
  
  ```bash
  kubectl -n d8-runtime-audit-engine port-forward runtime-audit-engine-4cpjc 2801:2801
  ```
  
  Create a debug event:
  
  ```bash
  curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" localhost:2801/test
  ```

  Check a debug event metric:

  ```bash
  kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus -- curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
  ```
  
  ```json
  ...
  {
    "metric": {
      "__name__": "falco_events",
      "container": "kube-rbac-proxy",
      "instance": "192.168.199.60:8766",
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
  ...
  ```
