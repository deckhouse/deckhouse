---
title: Managing control plane
description: Deckhouse manages a Kubernetes cluster control plane components — certificates, manifests, versions. Manages the configuration of the etcd cluster and maintains an up-to-date kubectl configuration.
---

The `control-plane-manager` (CPM) module is responsible for managing the cluster's control plane components. It runs on all master nodes of the cluster (nodes that have the `node-role.kubernetes.io/control-plane: ""` label).

The control-plane-manager:

- **Manages certificates** required for the operation of the control plane (renews certificates and re-issues them in response to configuration changes, among other things). This feature allows the CPM to automatically maintain a secure control plane configuration and quickly add additional SANs for organizing secure access to the Kubernetes API.
- **Configures components**. The CPM module automatically creates the required configs and manifests of the control plane components.
- **Upgrades/downgrades components**. Makes sure that the versions of the components in the cluster are the same.
- **Manages the configuration of the etcd cluster** and its members. The CPM module scales master nodes and migrates the cluster from single-master to multi-master (and vice versa).
- **Configures kubeconfig**. The CPM module maintains an up-to-date configuration for smooth kubectl operation. It generates, renews, updates kubeconfig with the cluster-admin rights, and creates a symlink for the root user so that kubeconfig can be used by default.
- **Extends scheduler functionality** by integrating external plugins via webhooks. Manages by [KubeSchedulerWebhookConfiguration](cr.html#kubeschedulerwebhookconfiguration) resource. Allows more complex logic to be used in workload scheduling tasks within the cluster. For example:
  - placing data storage application pods closer to the data itself,
  - state-based node prioritization (network load, storage subsystem status, etc.),
  - dividing nodes into zones, etc.

## Managing certificates

The CPM module manages certificates of the `control-plane` components, such as:

- Server certificates for `kube-apiserver` & `etcd`. These are stored in the secret `d8-pki` of the namespace `kube-system`:
  - the root CA kubernetes certificate (`ca.crt` & `ca.key`),
  - the root CA etcd certificate (`etcd/ca.crt` & `etcd/ca.key`),
  - the RSA certificate and the key for signing Service Accounts (`sa.pub` & `sa.key`),
  - the root CA certificate for the extension API servers (`front-proxy-ca.key` & `front-proxy-ca.crt`).
- Client certificates for connecting `control-plane` components to each other. The CPM module issues, renews, and re-issues if something has changed (e.g., the SAN list). These certificates are stored on the nodes only:
  - The server-side API server certificate (`apiserver.crt` & `apiserver.key`).
  - The client-side certificate for connecting `kube-apiserver` to `kubelet` (`apiserver-kubelet-client.crt` & `apiserver-kubelet-client.key`).
  - The client-side certificate for connecting `kube-apiserver` to `etcd` (`apiserver-etcd-client.crt` & `apiserver-etcd-client.key`).
  - The client-side certificate for connecting `kube-apiserver` to the extension API servers (`front-proxy-client.crt` & `front-proxy-client.key`).
  - The server-side `etcd` certificate (`etcd/server.crt` & `etcd/server.key`).
  - The client-side certificate for connecting `etcd` to other cluster members (`etcd/peer.crt` & `etcd/peer.key`).
  - The client-side certificate for connecting `kubelet` to `etcd` for performing health-checks  (`etcd/healthcheck-client.crt` & `etcd/healthcheck-client.key`).

Also, the CPM module lets you add the additional SANs to certificates (this way, you can quickly and effortlessly add more "entry points" to the Kubernetes API).

The CPM module automatically updates the kubeconfig configuration when certificates are changed.

## Scaling

The CPM module supports `control plane` running in a *single-master* or *multi-master* mode.

In the *single-master* mode:

- `kube-apiserver` only uses the `etcd` instance that is hosted on the same node;
- A proxy server is configured on the node that responds to localhost, `kube-apiserver` responds to the IP address of the master node.

In the *multi-master* mode, `control plane` components are automatically deployed in a fault-tolerant manner:

- `kube-apiserver`  is configured to work with all etcd instances;
- The additional proxy server that processes localhost requests is set up on each master node. By default, the proxy server sends requests to the local `kube-apiserver` instance. If it is unavailable, the proxy tries to connect to other `kube-apiserver` instances.

### Scaling master nodes

The `control-plane` nodes are scaled automatically using the `node-role.kubernetes.io/control-plane=""` label:

- Attaching the `node-role.kubernetes.io/control-plane=""` label to a node results in deploying `control plane` components on this node, connecting the new `etcd` node to the etcd cluster, and regenerating all the necessary certificates and config files.
- Removing the `node-role.kubernetes.io/control-plane=""` label results in deleting all `control plane` components on a node, gracefully removing it from the etcd cluster, and regenerating all the necessary config files and certificates.

{% alert level="warning" %}
Manual `etcd` [actions](./faq.html#what-if-the-etcd-cluster-fails) are required when decreasing the number of nodes from two to one. In all other cases, all the necessary actions are performed automatically. Please note that when scaling from any number of master nodes to one, sooner or later at the last step, the situation of scaling nodes from two to one will arise.
{% endalert %}

### Dynamic terminated pod garbage collection threshold

Automatically configures the optimal `--terminated-pod-gc-threshold` based on cluster size:

- **Small clusters** (less than 100 nodes): 1000 terminated Pods.
- **Medium clusters** (from 100 to 300 nodes): 3000 terminated Pods.
- **Large clusters** (from 300 nodes): 6000 terminated Pods.

{% alert level="info" %}
Note. This feature only takes effect in environments where the `terminated-pod-gc-threshold` parameter is configurable. On managed Kubernetes services (such as EKS, GKE, AKS), this setting is controlled by managed provider.
{% endalert %}

## Version control

**Patch versions** of control plane components (i.e. within the minor version, for example, from `1.30.13` to `1.30.14`) are upgraded automatically together with the Deckhouse version updates. You can't manage patch version upgrades.

Upgrading **minor versions** of control plane components (e.g. from `1.30.*` to `1.31.*`) can be managed using the [`kubernetesVersion`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter. It specifies the automatic update mode (if set to `Automatic`) or the desired minor version of the control plane. The default control plane version (to use with `kubernetesVersion: Automatic`) as well as a list of supported Kubernetes versions can be found in [the documentation](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html).

The control plane upgrade is performed in a safe way for both single-master and multi-master clusters. The API server may be temporarily unavailable during the upgrade. At the same time, it does not affect the operation of applications in the cluster and can be performed without scheduling a maintenance window.

If the target version (set with the [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter) does not match the current control plane version in the cluster, a smart strategy for changing component versions is applied:

- General remarks
  - Updating in different NodeGroups is performed in parallel. Within each NodeGroup, nodes are updated sequentially, one at a time.
- When upgrading:
  - Upgrades are carried out sequentially, one minor version at a time: 1.30 -> 1.31, 1.31 -> 1.32, 1.32 -> 1.33.
  - At each step, the control plane version is upgraded first, followed by kubelet upgrades on the cluster nodes.
- When downgrading:
  - Successful downgrading is only guaranteed for a single version down from the maximum minor version of the control plane ever used in the cluster.
  - kubelets on the cluster nodes are downgraded first, followed by the control plane components.

## Auditing

Kubernetes [Auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) can help you if you need to keep track of operations in your Namespaces or troubleshoot the cluster. You can configure it by setting the appropriate [Audit Policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy). As the result you will have the log file `/var/log/kube-audit/audit.log` containing audit events according to the configured Policy.

By default, in a cluster with Deckhouse, a basic policy is created for logging events:

- related to the creation, deletion, and changing of resources;
- committed from the names of ServiceAccounts from the "system" Namespace `kube-system`, `d8-*`;
- committed with resources in the "system" Namespace `kube-system`, `d8-*`.

A basic policy can be disabled by setting the [basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) flag to `false`.

When OIDC authentication is configured, additional user information is included in audit logs under the `user.extra` field:
- `user-authn.deckhouse.io/name` — user's display name
- `user-authn.deckhouse.io/preferred_username` — user's preferred username  
- `user-authn.deckhouse.io/dex-provider` — Dex provider identifier (requires `federated:id` scope)

You can find how to set up policies in [a special FAQ section](faq.html#how-do-i-configure-additional-audit-policies).

## Feature Gates

You can configure feature gates using the [enabledFeatureGates](configuration.html#parameters-enabledFeatureGates) parameter of the `control-plane-manager` ModuleConfig.

Changing the list of feature gates causes a restart of the corresponding component (for example, `kube-apiserver`, `kube-scheduler`, `kube-controller-manager`, `kubelet`).

The following example enables the `ComponentFlagz` and `ComponentStatusz` feature gates:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 2
  enabled: true
  settings:
    enabledFeatureGates:
      - ComponentFlagz
      - ComponentStatusz
```

If a feature gate is not supported or is deprecated, the monitoring system generates the [D8ProblematicFeatureGateInUse](/products/kubernetes-platform/documentation/v1/reference/alerts.html#control-plane-manager-d8problematicfeaturegateinuse) alert indicating that the feature gate will not be applied.

{% alert level="warning" %}
The Kubernetes version update (controlled by the [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter) will not occur if the list of enabled feature gates for the new version of Kubernetes includes deprecated feature gates.
{% endalert %}

More information about feature gates is available in the [Kubernetes documentation](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/){:target="_blank"}.

{% include feature_gates.liquid %}
