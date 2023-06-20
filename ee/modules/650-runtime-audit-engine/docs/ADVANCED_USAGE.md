---
title: "The runtime-audit-engine module: advanced usage"
---

## Enabling debugging logs

### Falco

Get the current log level (the [yq](https://github.com/mikefarah/yq) tool is used):

```shell
kubectl -n d8-runtime-audit-engine get configmap runtime-audit-engine -o yaml | yq e '.data."falco.yaml"' - | yq e .log_level - 
```

Make the following steps to set the log level to `debug`:
- Use the following command to edit configuration:

  ```shell
  kubectl -n d8-runtime-audit-engine edit configmap runtime-audit-engine
  ```

- Find or add the `log_level` field and set it to `debug`.

  Example:

  ```yaml
  log_level: debug
  ```

### Falcosidekick

If the `DEBUG` environment variable is `true`, then all outputs will print the payload they send in stdout.

Get the current `DEBUG` environment variable value:

```shell
kubectl -n d8-runtime-audit-engine get daemonset runtime-audit-engine -o yaml | 
  yq e '.spec.template.spec.containers[] | select(.name == "falcosidekick") | .env[] | select(.name == "DEBUG") | .value' -
```

Make the following steps to enable the debug mode.

- Use the following command to edit the `runtime-audit-engine` DaemonSet configuration:

  ```shell
  kubectl -n d8-runtime-audit-engine edit daemonset runtime-audit-engine
  ```

- Add (or edit) the `env` field for the `falcosidekick` container and set the `DEBUG` environment variable to `"true"`.

  Example:

  ```yaml
  env:
    - name: DEBUG
      value: "true"
  ```

## Viewing metrics

You can use the PromQL query `falco_events{}` to get metrics:

```shell
kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
  curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
```

## Emulating a Falco event

There are two ways to emulate a Falco event:

- You can use the [event-generator](https://github.com/falcosecurity/event-generator) CLI utility to generate a Falco events.

  Use the following command to run all events with the Pod in Kubernetes cluster:

  ```shell
  kubectl run falco-event-generator --image=falcosecurity/event-generator run
  ```

- You can use the [Falcosidekick](https://github.com/falcosecurity/falcosidekick) `/test` HTTP endpoint to send a test event to all enabled outputs.

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

  - Forward port from the Pod (the `runtime-audit-engine-4cpjc` in the example above) to localhost:

    ```shell
    kubectl -n d8-runtime-audit-engine port-forward runtime-audit-engine-4cpjc 2801:2801
    ```

  - Create a debug event, by making a query:

    ```shell
    curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" localhost:2801/test
    ```
  
  - Check a debug event metric:
  
    ```shell
    kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus --  \
      curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
    ```

    Example of the output part:
  
    ```json
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
    ```
