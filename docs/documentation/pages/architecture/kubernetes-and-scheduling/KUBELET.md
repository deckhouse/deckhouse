---
title: Kubelet
permalink: en/architecture/kubernetes-and-scheduling/kubelet.html
search: kubelet, kubelet agent, kubelet architecture, kubelet interactions
description: Architecture and role of kubelet in Deckhouse Kubernetes Platform.
---

Kubelet is not a control plane component, but it plays a key role in the operation of a Kubernetes cluster.

Kubelet is an agent that runs on every node in a Kubernetes cluster. It ensures that containers in pods are started and run according to their specifications. Kubelet continuously interacts with kube-apiserver to verify and maintain the state of nodes and containers. It is also responsible for starting control plane components.

## Static pod manifests

Kubelet starts control plane components from static pod manifests located in the `/etc/kubernetes/manifests` directory. In Deckhouse Kubernetes Platform, kubelet processes only files with the `.yaml` or `.yml` extension in this directory.

Files with other extensions, such as `kube-apiserver.backup`, `kube-apiserver.yaml.bak`, editor swap files, or other temporary files, are ignored. This prevents accidental processing of backup or non-manifest files as static pod manifests.

## Kubelet interactions

Kubelet interactions are shown in the [architecture diagram of the `control-plane-manager` module](control-plane-management.html).

Kubelet interacts with the following components:

1. **kubernetes-api-proxy**: Proxies requests to **kube-apiserver** that are sent to the `localhost` address. It is part of the [`control-plane-manager`](/modules/control-plane-manager/) module.
2. **kube-apiserver-healthcheck**: Checks the health of **kube-apiserver**.

The following components interact with kubelet:

1. **kube-apiserver**:

   * Retrieving pod logs (the `kubectl logs` command)
   * Executing commands in running pods (the `kubectl exec` command)
   * Port forwarding (the `kubectl port-forward` command)

2. **prometheus-main**: Collects kubelet metrics.
