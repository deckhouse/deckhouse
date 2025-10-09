---
title: "The deckhouse module: FAQ"
---

## How to run kube-bench in my cluster?

First, you have to exec in Deckhouse Pod:

```shell
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- bash
```

Then you have to select which node you want to run kube-bench.

* Run on random node:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | d8 k create -f -
  ```

* Run on specific node, e.g. control-plane node:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | d8 k apply -f - --dry-run=client -o json | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | d8 k create -f -
  ```

Then you can check report:

```shell
d8 k logs job.batch/kube-bench
```

{% alert level="warning" %}
Deckhouse set the log retention period to 7 days. However, according to the security requirements specified in kube-bench, logs should be retained for at least 30 days. Use separate storage for logs if you need to keep logs for more than 7 days.
{% endalert %}

## How to collect debug info?

We always appreciate helping users with debugging complex issues. Please follow these steps so that we can help you:

1. Collect all the necessary information by running the following command:

   ```sh
   d8 p collect-debug-info > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

{% alert level="info" %}
The `--exclude` flag allows you to exclude files whose data will not be included in the archive..

   ```sh
   d8 p collect-debug-info --exclude=queue global-values > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

The `--list-exclude` flag displays a list of files that can be excluded from the selection.
{% endalert %}

2. Send the archive to the [Deckhouse team](https://github.com/deckhouse/deckhouse/issues/new/choose) for further debugging.

Data that will be collected:

<table>
  <thead>
    <tr>
      <th>Category</th>
      <th>Collected data</th>
      <th>File in archive</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td rowspan="6"><strong>Deckhouse</strong></td>
      <td>Deckhouse queue state</td>
      <td><code>queue</code></td>
    </tr>
    <tr>
      <td>Deckhouse values (except for <code>kubeRBACProxyCA</code> and <code>registry.dockercfg</code>)</td>
      <td><code>global-values</code></td>
    </tr>
    <tr>
      <td>Current version of the <code>deckhouse</code> Pod</td>
      <td><code>deckhouse-version</code></td>
    </tr>
    <tr>
      <td>All DeckhouseRelease objects</td>
      <td><code>deckhouse-releases</code></td>
    </tr>
    <tr>
      <td>Logs of Deckhouse Pods</td>
      <td><code>deckhouse-logs</code></td>
    </tr>
    <tr>
      <td>Manifests of controllers and Pods from all Deckhouse namespaces</td>
      <td><code>d8-all</code></td>
    </tr>
    <tr>
      <td rowspan="11"><strong>Cluster objects</strong></td>
      <td>NodeGroup</td>
      <td><code>node-groups</code></td>
    </tr>
    <tr>
      <td>NodeGroupConfiguration</td>
      <td><code>node-group-configuration</code></td>
    </tr>
    <tr>
      <td>Node</td>
      <td><code>nodes</code></td>
    </tr>
    <tr>
      <td>Machine</td>
      <td><code>machines</code></td>
    </tr>
    <tr>
      <td>Instance</td>
      <td><code>instances</code></td>
    </tr>
    <tr>
      <td>StaticInstance</td>
      <td><code>staticinstances</code></td>
    </tr>
    <tr>
      <td>MachineDeployment</td>
      <td><code>cloud-machine-deployment</code>, <code>static-machine-deployment</code></td>
    </tr>
    <tr>
      <td>ClusterAuthorizationRule</td>
      <td><code>cluster-authorization-rules</code></td>
    </tr>
    <tr>
      <td>AuthorizationRule</td>
      <td><code>authorization-rules</code></td>
    </tr>
    <tr>
      <td>ModuleConfig</td>
      <td><code>module-configs</code></td>
    </tr>
    <tr>
      <td>Events from all namespaces</td>
      <td><code>events</code></td>
    </tr>
    <tr>
      <td rowspan="4"><strong>Modules and their states</strong></td>
      <td>List of enabled modules</td>
      <td><code>deckhouse-enabled-modules</code></td>
    </tr>
    <tr>
      <td>List of ModuleSource objects in the cluster</td>
      <td><code>deckhouse-module-sources</code></td>
    </tr>
    <tr>
      <td>List of ModulePullOverride objects in the cluster</td>
      <td><code>deckhouse-module-pull-overrides</code></td>
    </tr>
    <tr>
      <td>List of modules in <code>maintenance</code> mode</td>
      <td><code>deckhouse-maintenance-modules</code></td>
    </tr>
    <tr>
      <td rowspan="10"><strong>Controller logs and manifests</strong></td>
      <td>Logs of <code>machine-controller-manager</code></td>
      <td><code>mcm-logs</code></td>
    </tr>
    <tr>
      <td>Logs of <code>cloud-controller-manager</code></td>
      <td><code>ccm-logs</code></td>
    </tr>
    <tr>
      <td>Logs of <code>csi-controller</code></td>
      <td><code>csi-controller-logs</code></td>
    </tr>
    <tr>
      <td>Logs of <code>cluster-autoscaler</code></td>
      <td><code>cluster-autoscaler-logs</code></td>
    </tr>
    <tr>
      <td>Logs of Vertical Pod Autoscaler admission controller</td>
      <td><code>vpa-admission-controller-logs</code></td>
    </tr>
    <tr>
      <td>Logs of Vertical Pod Autoscaler recommender</td>
      <td><code>vpa-recommender-logs</code></td>
    </tr>
    <tr>
      <td>Logs of Vertical Pod Autoscaler updater</td>
      <td><code>vpa-updater-logs</code></td>
    </tr>
    <tr>
      <td>YAML manifest of <code>capi-controller-manager</code></td>
      <td><code>capi-controller-manager</code></td>
    </tr>
    <tr>
      <td>YAML manifest of <code>caps-controller-manager</code></td>
      <td><code>caps-controller-manager</code></td>
    </tr>
    <tr>
      <td>YAML manifest of <code>machine-controller-manager</code></td>
      <td><code>machine-controller-manager</code></td>
    </tr>
    <tr>
      <td rowspan="4"><strong>Monitoring and alerts</strong></td>
      <td>Prometheus logs</td>
      <td><code>prometheus-logs</code></td>
    </tr>
    <tr>
      <td>All active alerts in Prometheus</td>
      <td><code>alerts</code></td>
    </tr>
    <tr>
      <td>List of all Pods not in the <code>Running</code> state, except those in <code>Completed</code> or <code>Evicted</code> states</td>
      <td><code>bad-pods</code></td>
    </tr>
    <tr>
      <td>Audit Policy list</td>
      <td><code>audit-policy</code></td>
    </tr>
    <tr>
      <td rowspan="7"><strong>Network</strong></td>
      <td>All objects from the <code>d8-istio</code> namespace</td>
      <td><code>d8-istio-resources</code></td>
    </tr>
    <tr>
      <td>All <code>istio</code> custom resources</td>
      <td><code>d8-istio-custom-resources</code></td>
    </tr>
    <tr>
      <td>Envoy configuration for <code>istio</code></td>
      <td><code>d8-istio-envoy-config</code></td>
    </tr>
    <tr>
      <td>Logs of <code>istio</code></td>
      <td><code>d8-istio-system-logs</code></td>
    </tr>
    <tr>
      <td>Logs of the <code>istio</code> ingressgateway</td>
      <td><code>d8-istio-ingress-logs</code></td>
    </tr>
    <tr>
      <td>Logs of the <code>istio</code> users</td>
      <td><code>d8-istio-users-logs</code></td>
    </tr>
    <tr>
      <td>Cilium connection status (<code>cilium health status</code>)</td>
      <td><code>cilium-health-status</code></td>
    </tr>
  </tbody>
</table>

## How to debug pod problems with ephemeral containers?

Run the following command:

```shell
d8 k -n <namespace_name> debug -it <pod_name> --image=ubuntu <container_name>
```

More info in [official documentation](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container).

## How to debug node problems with ephemeral containers?

Run the following command:

```shell
d8 k debug node/mynode -it --image=ubuntu
```

More info in [official documentation](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#node-shell-session).
