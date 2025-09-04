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

2. Send the archive to the [Deckhouse team](https://github.com/deckhouse/deckhouse/issues/new/choose) for further debugging.

Data that will be collected:

<table>
  <thead>
    <tr>
      <th>Category</th>
      <th>Collected data</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><strong>Deckhouse</strong></td>
      <td>
        <ul>
          <li>Deckhouse queue state</li>
          <li>Deckhouse values (except for <code>kubeRBACProxyCA</code> and <code>registry.dockercfg</code>)</li>
          <li>Current version of the <code>deckhouse</code> Pod</li>
          <li>All DeckhouseRelease objects</li>
          <li>Logs of Deckhouse Pods</li>
          <li>Manifests of controllers and Pods from all Deckhouse namespaces</li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Cluster objects</strong></td>
      <td>
        All objects of the following resources:
        <ul>
          <li>NodeGroup</li>
          <li>NodeGroupConfiguration</li>
          <li>Node</li>
          <li>Machine</li>
          <li>Instance</li>
          <li>StaticInstance</li>
          <li>MachineDeployment</li>
          <li>ClusterAuthorizationRule</li>
          <li>AuthorizationRule</li>
          <li>ModuleConfig</li>
        </ul>
        As well as Events from all namespaces
      </td>
    </tr>
    <tr>
      <td><strong>Modules and their states</strong></td>
      <td>
        <ul>
          <li>List of enabled modules</li>
          <li>List of ModuleSource objects in the cluster</li>
          <li>List of ModulePullOverride objects in the cluster</li>
          <li>List of modules in <code>maintenance</code> mode</li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Controller logs and manifests</strong></td>
      <td>
        Logs of the following components:
        <ul>
          <li><code>machine-controller-manager</code></li>
          <li><code>cloud-controller-manager</code></li>
          <li><code>csi-controller</code></li>
          <li><code>cluster-autoscaler</code></li>
          <li>Vertical Pod Autoscaler admission controller</li>
          <li>Vertical Pod Autoscaler recommender</li>
          <li>Vertical Pod Autoscaler updater</li>
        </ul>
        YAML manifests of the following controllers:
        <ul>
          <li><code>capi-controller-manager</code></li>
          <li><code>caps-controller-manager</code></li>
          <li><code>machine-controller-manager</code></li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Monitoring and alerts</strong></td>
      <td>
        <ul>
          <li>Prometheus logs</li>
          <li>All active alerts in Prometheus</li>
          <li>List of all Pods not in the <code>Running</code> state, except those in <code>Completed</code> or <code>Evicted</code> states</li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Network</strong></td>
      <td>
        <ul>
          <li>All objects from the <code>d8-istio</code> namespace</li>
          <li>All <code>istio</code> custom resources</li>
          <li>Envoy configuration for <code>istio</code></li>
          <li>Logs of <code>istio</code></li>
          <li>Logs of the <code>istio</code> ingressgateway</li>
          <li>Logs of the <code>istio</code> users</li>
          <li>Cilium connection status (<code>cilium health status</code>)</li>
        </ul>
      </td>
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
