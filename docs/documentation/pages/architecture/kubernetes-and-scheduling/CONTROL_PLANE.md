---
title: Cluster control plane
permalink: en/architecture/kubernetes-and-scheduling/control-plane/
search: control plane architecture
description: Architecture of the cluster control plane in Deckhouse Kubernetes Platform.
---

Deckhouse Kubernetes Platform (DKP) uses a standard ("vanilla") Kubernetes cluster. The cluster control plane includes the following core components:

1. **kube-apiserver**: Kubernetes API server. It processes REST requests, provides access to the overall cluster state through which all other components interact, validates Kubernetes API resources, and stores them in **etcd** storage. It includes the following containers:

   * **kube-apiserver**: Main container.
   * **kube-apiserver-healthcheck**: Sidecar container that enables health checks of **kube-apiserver** without enabling anonymous authentication and without exposing an unauthenticated port. It uses a client certificate to authenticate to the API server. It is an [open source project](https://github.com/kubernetes/kops/blob/master/cmd/kube-apiserver-healthcheck).

2. **etcd**: Distributed key-value storage that holds all configuration data and Kubernetes cluster resources.

3. **kube-scheduler**: Kubernetes scheduler. It analyzes node resources and assigns pods based on constraints and rules such as affinity and taints.

4. **kube-controller-manager**: Kubernetes controller manager. It runs controller loops that monitor and reconcile the state of standard Kubernetes resources to match the desired state. Examples of controllers shipped with Kubernetes: replication controller, endpoints controller, namespace controller, and ServiceAccount controller.

Interactions between the Kubernetes control plane components are shown in the [architecture diagram of the `control-plane-manager` module](../control-plane-management/).
