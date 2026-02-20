---
title: Kubelet
permalink: en/architecture/kubernetes-and-scheduling/kubelet/
search: kubelet, kubelet agent, kubelet architecture, kubelet interactions
description: Architecture and role of kubelet in Deckhouse Kubernetes Platform.
---

Kubelet is not a control plane component, but it plays a key role in the operation of a Kubernetes cluster.

Kubelet is an agent that runs on every node in a Kubernetes cluster. It ensures that containers in pods are started and run according to their specifications. Kubelet continuously interacts with kube-apiserver to verify and maintain the state of nodes and containers. It is also responsible for starting control plane components.

## Kubelet interactions

Kubelet interactions are shown in the [architecture diagram of the `control-plane-manager` module](../control-plane-management/).

Kubelet interacts with the following components:

1. **kubernetes-api-proxy**: Proxies requests to **kube-apiserver** that are sent to the `localhost` address. It is part of the [`control-plane-manager`](/modules/control-plane-manager/) module.
2. **kube-apiserver-healthcheck**: Checks the health of **kube-apiserver**.

The following components interact with kubelet:

1. **kube-apiserver**:

   * Retrieving pod logs (the `kubectl logs` command)
   * Executing commands in running pods (the `kubectl exec` command)
   * Port forwarding (the `kubectl port-forward` command)

2. **prometheus-main**: Collects kubelet metrics.
