---
title: Cluster control plane management
permalink: en/architecture/kubernetes-and-scheduling/control-plane-management/
search: control-plane-manager, control plane management
description: Architecture and functions of the control-plane-manager module in Deckhouse Kubernetes Platform.
---

## Control-plane-manager module

Cluster control plane components are managed by the [`control-plane-manager`](/modules/control-plane-manager/) module, which runs on all master nodes (nodes labeled with `node-role.kubernetes.io/control-plane: ""`).

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

For detailed configuration options and usage examples, refer to the [`control-plane-manager` module documentation](/modules/control-plane-manager/).

### Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows pod containers interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). If a specific Service is used, its name is indicated above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`control-plane-manager`](/modules/control-plane-manager/) module and its interactions with other platform components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4 --->
![control-plane-manager module architecture](../../../images/architecture/kubernetes-and-scheduling/c4-l2-control-plane-manager.png)

## Module components

The module consists of the following components:

1. **d8-control-plane-manager** (DaemonSet): Manages cluster control plane components and runs on all master nodes. It includes the following containers:

   * **control-plane-manager**: Main container. Developed by Flant.

   * A set of sidecar containers used to pre-pull images of control plane components. These containers remain paused and serve only as image holders:

     * **image-holder-kube-apiserver**
     * **image-holder-kube-apiserver-healthcheck**
     * **image-holder-kube-controller-manager**
     * **image-holder-kube-scheduler**
     * **image-holder-etcd**

2. **kubernetes-api-proxy** (static pods): Additional proxy server configured on each master node to handle requests to `localhost`. By default, it proxies requests to the local **kube-apiserver** instance. If the latter is unavailable, it sequentially queries the remaining **kube-apiserver** instances. It includes the following containers:

   * **kubernetes-api-proxy**: [NGINX](https://github.com/nginx/nginx)-based proxy server.
   * **kubernetes-api-proxy-reloader**: Sidecar container that restarts the proxy server when its configuration changes. Developed by Flant.

3. **d8-etcd-backup** (CronJob): Periodically performs backups of the cluster's **etcd** database. It consists of the following container:

   * **backup**: Container running a shell script that creates an etcd snapshot using `etcdctl` and stores it in `/var/lib/etcd` on the master node (default directory, configurable via the [module parameters](/modules/control-plane-manager/configuration.html#parameters-etcd-backup)).

### Module interactions

The module interacts with the following components:

1. **kube-apiserver**:

   * Manages cluster control plane components.
   * Proxies and load-balances requests to **kube-apiserver** sent to `localhost`.

2. **etcd**:

   * Manages etcd cluster configuration and membership.
   * Performs periodic database backups.

The following external components interact with the module:

* **kubelet**: Requests to **kube-apiserver** sent to `localhost` are proxied by the module's **kubernetes-api-proxy** component.

## Cluster control plane monitoring

Control plane monitoring is provided by the [`monitoring-kubernetes-control-plane`](/modules/monitoring-kubernetes-control-plane/) module, which ensures secure metrics collection and provides a basic set of monitoring rules for the following cluster components:

* **kube-apiserver**
* **kube-controller-manager**
* **kube-scheduler**
* **etcd**

For configuration details, refer to the [`monitoring-kubernetes-control-plane` module documentation](/modules/monitoring-kubernetes-control-plane/).

### Components of the monitoring-kubernetes-control-plane module

The module consists of a single component:

1. **control-plane-proxy** (DaemonSet): Runs on all master nodes and includes a single container:

   * **kube-rbac-proxy**: Authorization proxy based on Kubernetes RBAC that provides secure access to metrics.

### Interactions of control-plane-proxy component

Control-plane-proxy interacts with the following components:

1. **kube-apiserver**: Authorizes requests for metrics.

2. Control plane components: **control-plane-proxy** forwards authorized metric requests to:

   * **kube-controller-manager**
   * **kube-scheduler**
   * **etcd**

**Prometheus-main** interacts with **control-plane-proxy** to collect control plane component metrics.

The interaction between the `monitoring-kubernetes-control-plane` module and the cluster control plane is shown in the architecture diagram of the `control-plane-manager` module above.

### Metrics collection from kube-apiserver

Metrics from **kube-apiserver** are collected directly by **prometheus-main**. The [`monitoring-kubernetes-control-plane`](/modules/monitoring-kubernetes-control-plane/) module adds the corresponding metric collection rules to the **prometheus-main** configuration.
