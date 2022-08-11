---
title: "The deckhouse module: FAQ"
---

## How to run kube-bench in my cluster?

First, you have to exec in Deckhouse Pod:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- bash
```

Then you have to select which node you want to run kube-bench.

* Run on random node:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl create -f -
  ```
  
* Run on specific node, e.g. control-plane node:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | yq r - -j | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | kubectl create -f -
  ```

Then you can check report:

```shell
kubectl logs job.batch/kube-bench
```

## How to collect debug info?

We always appreciate helping users with debugging complex issues. Please follow these steps so that we can help you:

1. Collect all the necessary information by running the following command:

   ```sh
   kubectl -n d8-system exec deploy/deckhouse \
     -- deckhouse-controller collect-debug-info \
     > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

2. Send the archive to the [Deckhouse team](https://github.com/deckhouse/deckhouse/issues/new/choose) for further debugging.

Data that will be collected:
* Deckhouse queue state
* global Deckhouse values (without any sensitive data)
* enabled modules list
* controllers and pods manifests from namespaces owned by Deckhouse
* `nodes` state
* `nodegroups` state
* `machines` state
* all `deckhousereleases` objects
* `events` from all namespaces
* Deckhouse logs
* machine controller manager logs
* cloud controller manager logs
* all firing alerts from Prometheus
* terraform-state-exporter metrics
