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

<table>
    <tr>
        <th>Category</th>
        <th>Collected Data</th>
    </tr>
    <tr>
        <td>Deckhouse</td>
        <td>
            <ul>
                <li>Deckhouse queue state</li>
                <li>Deckhouse values. Except <code>kubeRBACProxyCA</code> and <code>registry.dockercfg</code> values</li>
                <li>Current deckhouse pod version data</li>
                <li>All <code>DeckhouseRelease</code> objects</li>
                <li>Deckhouse pod logs</li>
                <li>Controller and pod manifests from all Deckhouse namespaces</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Cluster objects</td>
        <td>
            <ul>
                <li>All <code>NodeGroup</code> objects</li>
                <li>All <code>NodeGroupConfiguration</code> objects</li>
                <li>All <code>Node</code> objects</li>
                <li>All <code>Machine</code> objects</li>
                <li>All <code>Instance</code> objects</li>
                <li>All <code>StaticInstance</code> objects</li>
                <li>All <code>MachineDeployment</code> objects</li>
                <li>All <code>ClusterAuthorizationRule</code> objects</li>
                <li>All <code>AuthorizationRule</code> objects</li>
                <li>All <code>ModuleConfig</code> objects</li>
                <li><code>Events</code> from all namespaces</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Modules and their states</td>
        <td>
            <ul>
                <li>List of enabled modules</li>
                <li>List of <code>ModuleSource</code> objects in the cluster</li>
                <li>List of <code>ModulePullOverride</code> objects in the cluster</li>
                <li>List of modules in <code>maintenance</code> mode</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Controller logs and manifests</td>
        <td>
            <ul>
                <li>Machine controller manager logs</li>
                <li>Cloud controller manager logs</li>
                <li>Csi controller logs</li>
                <li>Cluster autoscaler logs</li>
                <li>Vertical Pod Autoscaler admission controller logs</li>
                <li>Vertical Pod Autoscaler recommender logs</li>
                <li>Vertical Pod Autoscaler updater logs</li>
                <li>Capi controller manager YAML files</li>
                <li>Caps controller manager YAML files</li>
                <li>Machine controller manager YAML files</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Monitoring and alerts</td>
        <td>
            <ul>
                <li>Prometheus logs</li>
                <li>All burning notifications in Prometheus</li>
                <li>List of all unbooted pods, except those in Completed and Evicted states</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Network</td>
        <td>
            <ul>
                <li>All objects from Namespace <code>d8-istio</code></li>
                <li>All Custom Resources istio</li>
                <li>Envoy config istio</li>
                <li>Logs istio</li>
                <li>Logs istio ingressgateway</li>
                <li>Logs istio users</li>
                <li>Cilium connection status - <code>cilium health status</code></li>
            </ul>
        </td>
    </tr>
</table>

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
