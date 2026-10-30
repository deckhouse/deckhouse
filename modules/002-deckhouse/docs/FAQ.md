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

1. Create a diagnostic archive with the `d8` utility, redirecting its output (stdout) to a file:

   ```shell
   d8 system collect-debug-info > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

1. Send the resulting archive to the [Deckhouse team](https://github.com/deckhouse/deckhouse/issues/new/choose) for further debugging.

> The `--exclude` flag omits the specified items from the archive. Example:

  ```sh
  d8 system collect-debug-info --exclude=queue global-values > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
  ```

> The `--list-exclude` flag prints the list of items available for exclusion. Example:

  ```shell
  d8 system collect-debug-info --list-exclude
  ```

<p>The following information is produced when creating the archive. Names in the "File in archive" column correspond to top-level items inside the resulting <code>tar.gz</code> archive. Certain sensitive values (e.g., <code>kubeRBACProxyCA</code> and <code>registry.dockercfg</code>) are excluded.</p>

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
<td>Manifests of controllers and pods from all Deckhouse namespaces</td>
<td><code>d8-all</code></td>
</tr>

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

<tr>
<td rowspan="4"><strong>Modules and states</strong></td>
<td>List of enabled modules</td>
<td><code>deckhouse-enabled-modules</code></td>
</tr>
<tr>
<td><code>ModuleSource</code> objects in the cluster</td>
<td><code>deckhouse-module-sources</code></td>
</tr>
<tr>
<td><code>ModulePullOverride</code> objects in the cluster</td>
<td><code>deckhouse-module-pull-overrides</code></td>
</tr>
<tr>
<td>Modules in <code>maintenance</code> mode</td>
<td><code>deckhouse-maintenance-modules</code></td>
</tr>

<tr>
<td rowspan="10"><strong>Controller logs and manifests</strong></td>
<td><code>machine-controller-manager</code> logs</td>
<td><code>mcm-logs</code></td>
</tr>
<tr>
<td><code>cloud-controller-manager</code> logs</td>
<td><code>ccm-logs</code></td>
</tr>
<tr>
<td><code>csi-controller</code> logs</td>
<td><code>csi-controller-logs</code></td>
</tr>
<tr>
<td><code>cluster-autoscaler</code> logs</td>
<td><code>cluster-autoscaler-logs</code></td>
</tr>
<tr>
<td>Vertical Pod Autoscaler admission controller logs</td>
<td><code>vpa-admission-controller-logs</code></td>
</tr>
<tr>
<td>Vertical Pod Autoscaler recommender logs</td>
<td><code>vpa-recommender-logs</code></td>
</tr>
<tr>
<td>Vertical Pod Autoscaler updater logs</td>
<td><code>vpa-updater-logs</code></td>
</tr>
<tr>
<td><code>capi-controller-manager</code> YAML</td>
<td><code>capi-controller-manager</code></td>
</tr>
<tr>
<td><code>caps-controller-manager</code> YAML</td>
<td><code>caps-controller-manager</code></td>
</tr>
<tr>
<td><code>machine-controller-manager</code> YAML</td>
<td><code>machine-controller-manager</code></td>
</tr>

<tr>
<td rowspan="4"><strong>Monitoring and alerts</strong></td>
<td>Prometheus logs</td>
<td><code>prometheus-logs</code></td>
</tr>
<tr>
<td>Active (firing) Prometheus alerts</td>
<td><code>alerts</code></td>
</tr>
<tr>
<td>Pods not in <code>Running</code> (excluding <code>Completed</code> and <code>Evicted</code>)</td>
<td><code>bad-pods</code></td>
</tr>
<tr>
<td>List of Audit Policies</td>
<td><code>audit-policy</code></td>
</tr>

<tr>
<td rowspan="7"><strong>Network</strong></td>
<td>All objects in the <code>d8-istio</code> namespace</td>
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
<td><code>istio</code> logs</td>
<td><code>d8-istio-system-logs</code></td>
</tr>
<tr>
<td><code>istio</code> ingress gateway logs</td>
<td><code>d8-istio-ingress-logs</code></td>
</tr>
<tr>
<td><code>istio</code> users logs</td>
<td><code>d8-istio-users-logs</code></td>
</tr>
<tr>
<td>Cilium connection status (<code>cilium health status</code>)</td>
<td><code>cilium-health-status</code></td>
</tr>

<tr><td colspan="3" style="padding:0;"></td></tr>
</tbody>
</table>

## How to debug pod problems with ephemeral containers?

Run the following command:

```shell
d8 k -n <namespace_name> debug -it <pod_name> --image=ubuntu <container_name>
```

More info in [official documentation](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container).

An ephemeral container cannot be added to a Pod labeled `heritage: deckhouse`, see the next section.

## Who can exec into Deckhouse Pods and scale Deckhouse objects?

Only Deckhouse system users. The `deckhouse` module denies the following operations to everyone else, cluster administrators and superadmins included, in every namespace:

- `kubectl exec`, `kubectl attach`, `kubectl cp`, `kubectl port-forward` and `kubectl debug` into a Pod labeled `heritage: deckhouse`. A port-forward to a Service ends up in one of its Pods and is denied the same way;
- `kubectl scale` of a Deployment, StatefulSet, ReplicaSet or ReplicationController labeled `heritage: deckhouse`, unless the object is under maintenance, that is, labeled `maintenance.deckhouse.io/no-resource-reconciliation`.

The label is checked on the object itself: a Pod without `heritage: deckhouse` is not protected, even if its Deployment carries the label.

The checks do not apply to:

- `system:sudouser`;
- `dhctl` and `observability`;
- service accounts from `d8-*` and `kube-*` namespaces (`system:serviceaccount:d8-*`, `system:serviceaccount:kube-*`), the Deckhouse controllers and the horizontal pod autoscaler of `kube-controller-manager` among them;
- the Kubernetes control plane components (`system:apiserver`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`) and nodes (the `system:nodes` group);
- the `system:masters` group, that is, `super-admin.conf` on a master node. `admin.conf` (the `kubeadm:cluster-admins` group) is subject to the checks.

Apart from `system:masters`, these are the users who may also change objects labeled `heritage: deckhouse`. The checks do not depend on the `admission-policy-engine` module.

The checks run in the `webhook-handler` Pods of the `d8-system` namespace and fail closed. While `webhook-handler` is unavailable, `kubectl exec`, `attach`, `cp`, `port-forward` into any Pod and `kubectl scale` of any Deployment, StatefulSet, ReplicaSet or ReplicationController are denied in every namespace for everyone but the users listed above, whether or not the target is labeled. To get in for diagnostics, run the command as `system:sudouser` in the `system:sudousers` group, which needs the right to impersonate both, or use `super-admin.conf` on a master node. The group is needed for exec because the `user-authz` module checks exec on the same `webhook-handler` and exempts the group rather than the user:

```shell
d8 k --as=system:sudouser --as-group=system:sudousers -n d8-system exec -it <pod_name> -- sh
```

## How to debug node problems with ephemeral containers?

Run the following command:

```shell
d8 k debug node/mynode -it --image=ubuntu
```

More info in [official documentation](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#node-shell-session).
