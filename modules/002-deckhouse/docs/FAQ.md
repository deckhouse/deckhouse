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

##### Deckhouse:
* Deckhouse queue state;
* Deckhouse values. Except `kubeRBACProxyCA` and `registry.dockercfg` values;
* Current deckhouse pod version data;
* All `DeckhouseRelease` objects;
* Deckhouse pod logs;
* Controller and pod manifests from all Deckhouse namespaces;

##### Cluster objects:
* All `NodeGroup` objects;
* All `NodeGroupConfiguration` objects;
* All `Node` objects;
* All `Machine` objects;
* All `Instance` objects;
* All `StaticInstance` objects;
* All `MachineDeployment` objects;
* All `ClusterAuthorizationRule` objects;
* All `AuthorizationRule` objects;
* All `ModuleConfig` objects;
* `Events` from all namespaces;

##### Modules and their states:
* List of enabled modules;
* List of `ModuleSource` objects in the cluster;
* List of `ModulePullOverride` objects in the cluster;
* List of modules in `maintenance` mode;

##### Controller logs and manifests:
* Machine controller manager logs;
* Cloud controller manager logs;
* Csi controller logs;
* Cluster autoscaler logs;
* Vertical Pod Autoscaler admission controller logs;
* Vertical Pod Autoscaler recommender logs;
* Vertical Pod Autoscaler updater logs;
* Capi controller manager YAML files;
* Caps controller manager YAML files;
* Machine controller manager YAML files;

##### Monitoring and alerts:
* Prometheus logs;
* All burning notifications in Prometheus;
* List of all unbooted pods, except those in Completed and Evicted states;

##### Network:
* All objects from Namespace `d8-istio`;
* All Custom Resources istio;
* Envoy config istio;
* Logs istio;
* Logs istio ingressgateway;
* Logs istio users;
* Cilium connection status - `cilium health status`;

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
