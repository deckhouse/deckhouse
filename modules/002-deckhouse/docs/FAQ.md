---
title: "The deckhouse module: FAQ"
---

## How to run kube-bench in my cluster?

First, you have to exec in Deckhouse Pod:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- bash
```

Then you have to select which node you want to run kube-bench.

* Run on random node:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl create -f -
  ```

* Run on specific node, e.g. control-plane node:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl apply -f - --dry-run=client -o json | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | kubectl create -f -
  ```

Then you can check report:

```shell
kubectl logs job.batch/kube-bench
```

{% alert level="warning" %}
Deckhouse set the log retention period to 7 days. However, according to the security requirements specified in kube-bench, logs should be retained for at least 30 days. Use separate storage for logs if you need to keep logs for more than 7 days.
{% endalert %}

## How to collect debug info?

We always appreciate helping users with debugging complex issues. Please follow these steps so that we can help you:

1. Collect all the necessary information by running the following command:

   ```sh
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse \
     -- deckhouse-controller collect-debug-info \
     > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

2. Send the archive to the [Deckhouse team](https://github.com/deckhouse/deckhouse/issues/new/choose) for further debugging.

Data that will be collected:
* Deckhouse queue state
* global Deckhouse values. Except for the values of `kubeRBACProxyCA` and `registry.dockercfg`
* enabled modules list
* list of modules in `maintenance` mode
* `events` from all namespaces
* controllers and pods manifests from namespaces owned by Deckhouse
* `nodegroups` state
* `nodes` state
* `machines` state
* `instances` state
* `staticinstances` state
* deckhouse pods version
* all `deckhousereleases` objects
* Deckhouse logs
* machine controller manager logs
* cloud controller manager logs
* cluster autoscaler logs
* Vertical Pod Autoscaler admission controller logs
* Vertical Pod Autoscaler recommender logs
* Vertical Pod Autoscaler updater logs
* Prometheus logs
* all firing alerts from Prometheus

## How to debug pod problems with ephemeral containers?

Run the following command:

```shell
kubectl -n <namespace_name> debug -it <pod_name> --image=ubuntu <container_name>
```

More info in [official documentation](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container).

## How to debug node problems with ephemeral containers?

Run the following command:

```shell
kubectl debug node/mynode -it --image=ubuntu
```

More info in [official documentation](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#node-shell-session).
