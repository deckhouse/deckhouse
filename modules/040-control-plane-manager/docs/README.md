---
title: Managing control plane
---

The `control-plane-manager` (CPM) module is responsible for managing the cluster's control plane components. It runs on all master nodes of the cluster (nodes that have the `node-role.kubernetes.io/master: ""` label).

The control-plane-manager:
- **Manages certificates** required for the operation of the control plane (renews certificates and re-issues them in response to configuration changes, among other things). This feature allows the CPM to automatically maintain a secure control plane configuration and quickly add additional SANs for organizing secure access to the Kubernetes API.
- **Configures components**. The CPM module automatically creates the required configs and manifests of the control plane components.
- **Upgrades/downgrades components**. Makes sure that the versions of the components in the cluster are the same.
- **Manages the configuration of the etcd cluster** and its members. The CPM module scales master nodes and migrates the cluster from single-master to multi-master (and vice versa).
- **Configures kubeconfig**. The CPM module maintains an up-to-date configuration for smooth kubectl operation. It generates, renews, updates kubeconfig with the cluster-admin rights, and creates a symlink for the root user so that kubeconfig can be used by default.

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
- `kube-apiserver` processes localhost requests.

In the *multi-master* mode, `control plane` components are automatically deployed in a fault-tolerant manner:
- `kube-apiserver`  is configured to work with all etcd instances;
- The additional proxy server that processes localhost requests is set up on each master node. By default, the proxy server sends requests to the local `kube-apiserver` instance. If it is unavailable, the proxy tries to connect to other `kube-apiserver` instances.

### Scaling master nodes
The `control-plane` nodes are scaled automatically using the `node-role.kubernetes.io/master=””` label:
- Attaching the `node-role.kubernetes.io/master=””` label to a node results in deploying `control plane` components on this node, connecting the new `etcd` node to the etcd cluster, and regenerating all the necessary certificates and config files.
- Removing the `node-role.kubernetes.io/master=””` label results in deleting all `control plane` components on a node, gracefully removing it from the etcd cluster, and regenerating all the necessary config files and certificates.

> **Note** that **manual `etcd` actions** are required when decreasing the number of nodes from two to one. In all other cases, all the necessary actions are performed automatically.

## Version control

The **patch version** of any control plane component (i.e. within the minor version, for example, from 1.19.3 to 1.19.8) is upgraded automatically along with the Deckhouse version.

The upgrade of a **minor version** of any control plane component is performed in a safe way. You just need to specify the desired minor version in the `control plane` settings. Deckhouse implements a smart strategy for changing the versions of `control plane` components if the desired version does not match the current one:
- When upgrading:
  - The upgrades are performed **sequentially**, one minor version at a time: 1.19 -> 1.20, 1.20 -> 1.21, 1.21 -> 1.22.
  - You cannot proceed to the next version until all the `control plane` components have been successfully upgraded to the current one.
  - The version to upgrade to can only be one minor version ahead of the kubelet versions on the nodes.
- When downgrading:
  - The downgrade is performed **sequentially**, one minor version at a time: 1.22 -> 1.21, 1.21 -> 1.20, 1.20 -> 1.19.
  - Master nodes cannot have a lower version than workers: the downgrade isn't possible if the kubelet versions on the nodes aren't downgraded yet.
  - When downgrading, the component version can only be one version behind the highest ever used minor version of the control plane components:
    - Suppose, `maxUsedControlPlaneVersion = 1.20`. In this case, the lowest possible version of the control plane components is `1.19`.

[List of supported Kubernetes versions...](../../supported_versions.html#kubernetes)

## Auditing

Kubernetes [Auditing](https://kubernetes.io/docs/tasks/debug-application-cluster/debug-cluster/) can help you if you need to keep track of operations in your Namespaces or troubleshoot the cluster. You can configure it by setting the appropriate [Audit Policy](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/#audit-policy). As the result you will have the log file `/var/log/kube-audit/audit.log` containing audit events according to the configured Policy.

By default, in a cluster with Deckhouse, a basic policy is created for logging events:
- related to the creation, deletion, and changing of resources;
- committed from the names of ServiceAccounts from the "system" Namespace `kube-system`, `d8-*`;
- committed with resources in the "system" Namespace `kube-system`, `d8-*`.

A basic policy can be disabled by setting the [basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) flag to `false`.

You can find how to set up policies in [a special FAQ section](faq.html#how-do-i-configure-additional-audit-policies).
