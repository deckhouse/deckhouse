---
title: Cluster control plane management
permalink: en/architecture/kubernetes-and-scheduling/control-plane-management.html
search: control-plane-manager, control plane management
description: Architecture and functions of the control-plane-manager module in Deckhouse Kubernetes Platform.
---

## Control-plane-manager module

Cluster control plane components are managed by the [`control-plane-manager`](/modules/control-plane-manager/) module, which runs on all master nodes (nodes labeled with `node-role.kubernetes.io/control-plane: ""`).

The module operates with the following custom resources:

- [ControlPlaneNode](/modules/control-plane-manager/cr.html#controlplanenode): Describes the parameters and state of control plane nodes (master nodes) in the cluster. It's used for managing the lifecycle and configuration of each control plane component.
- [ControlPlaneOperation](/modules/control-plane-manager/cr.html#controlplaneoperation): Defines operations on control plane components (upgrade, downgrade, addition, or removal of components), and allows tracking and managing the execution of these operations at the cluster level.
- [KubeSchedulerWebhookConfiguration](/modules/control-plane-manager/cr.html#kubeschedulerwebhookconfiguration): Describes the parameters and logic for connecting external webhooks to the `kube-scheduler` component to extend its functionality.

{% alert level="warning" %}
The ControlPlaneNode and ControlPlaneOperation custom resources are available to users in read-only mode. Full lifecycle management of these resources is performed exclusively by the `control-plane-manager` module.
{% endalert %}

Control plane management functions:

* **Certificate management**: Issuing, renewing, and rotating certificates required for control plane operation. Ensures automatic and secure control plane configuration and allows additional Subject Alternative Names (SAN) to be added for secure access to the Kubernetes API.
* **Component configuration**: Automatic generation of the required configuration files and manifests for control plane components.
* **Component upgrade and downgrade**: Maintains consistent component versions across the cluster.
* **Management of etcd cluster configuration**: Scaling master nodes and migrating between single-master and multi-master configurations.
* **Management of kubeconfig**: Maintains an up-to-date configuration for using `kubectl` on cluster nodes. Generates, renews, and updates the kubeconfig with *cluster-admin* privileges and creates a symbolic link for the `root` user so that the kubeconfig is used by default.
* **Scheduler extension**: Enables external plugins via webhooks using the [KubeSchedulerWebhookConfiguration](/modules/control-plane-manager/cr.html#kubeschedulerwebhookconfiguration) resource. This allows advanced scheduling logic when planning cluster loads, for example:

  * Placing data-intensive application pods closer to their data.
  * Prioritizing nodes based on their state (network load, storage subsystem health, etc.).
  * Dividing nodes into zones, etc.

* **Periodic etcd defragmentation**. In clusters with three or more etcd members, this feature is enabled by default. For more details, see the [`control-plane-manager` module documentation](/modules/control-plane-manager/configuration.html#parameters-etcd-defrag).

For detailed configuration options and usage examples, refer to the [`control-plane-manager` module documentation](/modules/control-plane-manager/).

### Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`control-plane-manager`](/modules/control-plane-manager/) module and its interactions with other platform components are shown in the following diagram:

![control-plane-manager module architecture](../../images/architecture/kubernetes-and-scheduling/c4-l2-control-plane-manager.png)

## Module components

The module consists of the following components:

1. **d8-control-plane-manager** (DaemonSet): Manages cluster control plane components and runs on all master nodes.

   The **d8-control-plane-manager** controller performs the following actions:

   * Monitors the `d8-control-plane-manager-config` and `d8-pki` Secrets and, based on their information, creates or updates the ControlPlaneNode custom resource for each master node.

   * If the required node configuration differs from the current one, creates a ControlPlaneOperation resource to perform operations to update the node's configuration.

   * Determines the order in which to execute the requested ControlPlaneOperation operations to maintain the required cluster fault tolerance during updates.

   * Monitors the execution of operations specified in the ControlPlaneOperation resource.

   * After the requested operations are completed, updates the current state of the master node in the ControlPlaneNode resource.

   It consists of the following containers:

   * **control-plane-manager**: Main container. Developed by Flant.

   * A set of sidecar containers used to pre-pull images of control plane components. These containers remain paused and serve only as image holders:

     * **image-holder-kube-apiserver**
     * **image-holder-kube-apiserver-healthcheck**
     * **image-holder-kube-controller-manager**
     * **image-holder-kube-scheduler**
     * **image-holder-etcd**

1. **kubernetes-api-proxy** (static pods): Additional proxy server configured on each master node to handle requests to `localhost`. By default, it proxies requests to the local **kube-apiserver** instance. If the latter is unavailable, it sequentially queries the remaining **kube-apiserver** instances. It includes the following containers:

   * **kubernetes-api-proxy**: [NGINX](https://github.com/nginx/nginx)-based proxy server.
   * **kubernetes-api-proxy-reloader**: Sidecar container that restarts the proxy server when its configuration changes. Developed by Flant.

1. **control-plane-proxy** (DaemonSet): Component that is installed in the cluster when the [`prometheus`] module(/modules/prometheus/) is enabled. In this case, the control-plane-proxy runs on all master nodes and forwards authorized requests for metrics of the following components of the cluster control plane:

   * **kube-controller-manager**;
   * **kube-scheduler**;
   * **etcd**.

   It consists of a single container:

   * **kube-rbac-proxy***: Container with an authorization proxy based on Kubernetes RBAC, providing secure access for metrics of the components of the cluster control plane. It is an [open source project](https://github.com/brancz/kube-rbac-proxy).

1. **control-plane-proxy-etcd-arbiter** (DaemonSet): Optional component that is installed in the cluster when the [`prometheus`] module(/modules/prometheus/) is enabled, if the cluster operates [in HA mode with two master nodes and an arbiter node](../admin/configuration/high-reliability-and-availability/enable.html#configuring-ha-mode-with-two-master-nodes-and-an-arbiter-node). In this case, Control-plane-proxy-etcd-arbiter runs on the arbiter node and forwards authorized requests for metrics to the etcd instance running on it.

   It consists of a single container:

   * **kube-rbac-proxy***: Container with an authorization proxy based on Kubernetes RBAC, providing secure access for metrics of the etcd instance (described above).

1. **d8-etcd-backup** (CronJob): Periodically performs backups of the cluster's **etcd** database. It consists of the following container:

   * **backup**: Container running a shell script that creates an etcd snapshot using `etcdctl` and stores it in `/var/lib/etcd` on the master node (default directory, configurable via the [module parameters](/modules/control-plane-manager/configuration.html#parameters-etcd-backup)).

### Module interactions

The module interacts with the following components:

1. **kube-apiserver**:

   * Manages cluster control plane components.
   * Reconciles ControlPlaneNode and ControlPlaneOperation custom resources.
   * Watches `d8-control-plane-manager-config` and `d8-pki` Secrets.
   * Proxies and load-balances requests to **kube-apiserver** sent to `localhost`.
   * Authorizes the requests for metrics.

1. **etcd**:

   * Manages etcd cluster configuration and membership.
   * Performs periodic database backups.

The following external components interact with the module:

* **kubelet**: Requests to **kube-apiserver** sent to `localhost` are proxied by the module's **kubernetes-api-proxy** component.
* **Prometheus-main**: Collects metrics of the control plane components.

## Cluster control plane monitoring

The module provides control plane monitoring, ensuring secure metrics collection and providing a basic set of monitoring rules for the following cluster components:

* **kube-apiserver**
* **kube-controller-manager**
* **kube-scheduler**
* **etcd**

Metrics from **kube-apiserver** are collected directly by **prometheus-main**. Metrics of the other control-plane components are collected by **prometheus-main** with authorization in kube-apiserver via the control-plane-proxy component. The `control-plane-manager` module adds the corresponding metric collection rules to the **prometheus-main** configuration.
