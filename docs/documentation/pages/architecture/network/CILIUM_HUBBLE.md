---
title: Cilium-hubble module
permalink: en/architecture/network/cilium-hubble.html
search: cilium-hubble, cilium, hubble
description: Architecture of the cilium-hubble module in Deckhouse Kubernetes Platform.
---

The [`cilium-hubble`](/modules/cilium-hubble/) module provides visualization of the cluster network stack if the Cilium CNI is enabled.

For more details about module configuration, refer to the [corresponding documentation section](/modules/cilium-hubble/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`cilium-hubble`](/modules/cilium-hubble/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

![Cilium-hubble module architecture](../../images/architecture/network/c4-l2-cilium-hubble.png)

{% alert level="info" %}
The numbers in the diagram indicate the sequence of steps that a user request goes through before reaching the hubble-ui component:

- In steps 1, 2, and 3, the request passes through the Ingress NGINX Controller, where mandatory user authentication is performed using the [`user-authn`](/modules/user-authn) module;
- In steps 4 and 5, the user is authorized based on Kubernetes RBAC to provide secure access.
{% endalert %}

## Module components

The module consists of the following components:

1. **Hubble-relay**: Component that aggregates events from all cluster nodes into a single view. Hubble-relay establishes a permanent connection to each Cilium Agent on the nodes and, through the gRPC stream, receives events, deduplicates them, and provides a single gRPC endpoint for the Hubble CLI and Hubble UI (web interface). While doing this, hubble-relay does not store history, it broadcasts an event stream in real time.

   It consists of a single container:

   * **hubble-relay**: Main container. It is a part of [Cilium](https://github.com/cilium/cilium) project.

1. **Hubble-ui**: Component that implements the web interface. Hubble-ui connects to hubble-relay and provides convenient access to data that comes from Cilium agents. Hubble-ui builds an interactive Service Map and a flow table with filters.

   It consists of the following containers:

   * **frontend**: Container that is an [NGINX](https://github.com/nginx/nginx) proxy server, which distributes static files of the hubble-ui web interface and forwards requests to the `/api` endpoint to the **backend** container of the component.
   * **backend**: Container implementing the hubble-ui API that sends requests to the hubble-relay component to receive in real time data coming from Cilium agents.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to Hubble UI. It is an [open source project](https://github.com/brancz/kube-rbac-proxy).

   Frontend and backend containers are built based on [Hubble-UI](https://github.com/cilium/hubble-ui ), which is an Open Source project.

## Module interactions

The module interacts with the following components:

1. **Cilium-agent**: Receives in real time an event stream from Cilium agents.
1. **Kube-apiserver**: Authorizes requests to Hubble UI.

The following external components interact with the module:

* **Controller nginx**: Forwards external user requests to the module web interface.
