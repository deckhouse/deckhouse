---
title: Chrony module
permalink: en/architecture/cluster-and-infrastructure/infrastructure/chrony.html
search: chrony, ntp, time sync
description: Architecture of the chrony module in Deckhouse Kubernetes Platform.
---

The [`chrony`](/modules/chrony/) module provides time synchronization on all nodes in the Deckhouse Kubernetes Platform (DKP) cluster using [chrony](https://chrony-project.org/index.html) NTP server/client implementation.

For more details, refer to the [module documentation](/modules/chrony/configuration/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`chrony`](/modules/chrony/) module its interactions with other DKP components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Chrony module architecture](../../../../images/architecture/cluster-and-infrastructure/c4-l2-chrony.png)

## Module components

The module consists of the following components:

1. **Chrony-master**: Time synchronization service on cluster master nodes with external NTP servers.

   It consists of the following containers:

   - **chrony**: Main container.
   - **chrony-exporter**: A sidecar container that collects metrics from the chrony container and exposes them in Prometheus format.
   - **kube-rbac-proxy**: A sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to metrics from the chrony-exporter container.

1. **Chrony**: Time synchronization service on all cluster nodes except master nodes, synchronized with the chrony-master component.

   It consists of the following containers:

   - **chrony**: Main container.
   - **chrony-exporter**: A sidecar container that collects metrics from the chrony container and exposes them in Prometheus format.
   - **kube-rbac-proxy**: A sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to metrics from the chrony-exporter container.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**: Authorizes requests to retrieve metrics.

1. **External NTP servers**: Performs time synchronization.

The following external components interact with the module:

- **Prometheus-main**: Collects metrics from chrony and chrony-master.
