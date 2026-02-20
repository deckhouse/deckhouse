---
title: Kubernetes & Scheduling subsystem
permalink: en/architecture/kubernetes-and-scheduling/
search: Kubernetes subsystem, scheduling, control-plane-manager, descheduler, VPA, kubelet
description: Architecture of the Kubernetes & Scheduling subsystem in Deckhouse Kubernetes Platform.
---

This subsection describes the architecture of the modules that are part of the Kubernetes & Scheduling subsystem of Deckhouse Kubernetes Platform (DKP).

The Kubernetes & Scheduling subsystem includes the following modules:

* [`control-plane-manager`](/modules/control-plane-manager/): Main module of the subsystem, responsible for [managing cluster control plane components](control-plane-management/).
* [`descheduler`](/modules/descheduler/): Analyzes the cluster state and evicts pods in accordance with the [active strategies](/modules/descheduler/#strategies).
* [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/): Automatically adjusts container resource requests and limits in pods based on actual consumption. The module architecture is described on the [corresponding page](../vpa.html).

This subsection also describes the architecture of the [control plane](control-plane/) and the [kubelet agent](kubelet/).
