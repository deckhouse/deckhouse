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

## How to collect troubleshooting information

1. Create a diagnostic archive with the `d8` utility, redirecting its output (stdout) to a file:

   ```shell
   d8 p collect-debug-info > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

1. Send the resulting archive to the [Deckhouse team](https://github.com/deckhouse/deckhouse/issues/new/choose) for further debugging.

> The `--exclude` flag omits the specified items from the archive. Example:

  ```sh
  d8 p collect-debug-info --exclude=queue global-values > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
  ```

> The `--list-exclude` flag prints the list of items available for exclusion. Example:

  ```shell
  d8 p collect-debug-info --list-exclude
  ```

<p>The following information is produced when creating the archive. The names in the “File in archive” column correspond to top-level items inside the resulting <code>tar.gz</code> archive. Certain sensitive values (for example, <code>kubeRBACProxyCA</code> and <code>registry.dockercfg</code>) are excluded.</p>

<table>
  <thead>
    <tr>
      <th>Category</th>
      <th>Collected data</th>
      <th>File in archive</th>
    </tr>
  </thead>
  <tbody>
    <!-- Deckhouse -->
    <tr>
      <td rowspan="6"><strong>Deckhouse</strong></td>
      <td>Deckhouse queue state</td>
      <td><code>queue</code></td>
    </tr>
    <tr>
      <td>Deckhouse values (excluding <code>kubeRBACProxyCA</code> and <code>registry.dockercfg</code>)</td>
      <td><code>global-values</code></td>
    </tr>
    <tr>
      <td>Current <code>deckhouse</code> pod version</td>
      <td><code>deckhouse-version</code></td>
    </tr>
    <tr>
      <td>All <code>DeckhouseRelease</code> objects</td>
      <td><code>deckhouse-releases</code></td>
    </tr>
    <tr>
      <td>Deckhouse pod logs</td>
      <td><code>deckhouse-logs</code></td>
    </tr>
    <tr>
      <td>Controller and pod manifests from all Deckhouse namespaces</td>
      <td><code>d8-all</code></td>
    </tr>

    <!-- Cluster objects -->
    <tr>
      <td rowspan="11"><strong>Cluster objects</strong></td>
      <td><code>NodeGroup</code></td>
      <td><code>node-groups</code></td>
    </tr>
    <tr>
      <td><code>NodeGroupConfiguration</code></td>
      <td><code>node-group-configuration</code></td>
    </tr>
    <tr>
      <td><code>Node</code></td>
      <td><code>nodes</code></td>
    </tr>
    <tr>
      <td><code>Machine</code></td>
      <td><code>machines</code></td>
    </tr>
    <tr>
      <td><code>Instance</code></td>
      <td><code>instances</code></td>
    </tr>
    <tr>
      <td><code>StaticInstance</code></td>
      <td><code>staticinstances</code></td>
    </tr>
    <tr>
      <td><code>MachineDeployment</code></td>
      <td><code>cloud-machine-deployment</code>, <code>static-machine-deployment</code></td>
    </tr>
    <tr>
      <td><code>ClusterAuthorizationRule</code></td>
      <td><code>cluster-authorization-rules</code></td>
    </tr>
    <tr>
      <td><code>AuthorizationRule</code></td>
      <td><code>authorization-rules</code></td>
    </tr>
    <tr>
      <td><code>ModuleConfig</code></td>
      <td><code>module-configs</code></td>
    </tr>
    <tr>
      <td>Events (all namespaces)</td>
      <td><code>events</code></td>
    </tr>

    <!-- Modules and their states -->
    <tr>
      <td rowspan="4"><strong>Modules and their states</strong></td>
      <td>List of enabled modules</td>
      <td><code>deckhouse-enabled-modules</code></td>
    </tr>
    <tr>
      <td><code>ModuleSource</code> objects in the

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
