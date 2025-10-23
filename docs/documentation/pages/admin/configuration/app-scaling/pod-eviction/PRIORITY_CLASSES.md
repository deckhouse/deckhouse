---
title: "Priority classes"
permalink: en/admin/configuration/app-scaling/pod-eviction/priority-classes.html
description: "Configure pod priority classes in Deckhouse Kubernetes Platform. Pod eviction policies, resource allocation priorities, and cluster resource management optimization."
---

## Pod priorities (Priority Classes)

### Available Priority Classes

DKP creates a set of predefined PriorityClass objects in the cluster and assigns them to Deckhouse components and user applications. The Kubernetes scheduler takes pod priority into account when allocating resources: if there are not enough resources, pods with lower priorities will be evicted first.

For example, if pods are assigned `priorityClassName: production-low` and the cluster runs out of resources, the scheduler will first evict pods with `priorityClassName: develop`, then `cluster-low`, and so on. Therefore, when selecting a priority class, you should consider the type of application and the environment in which it runs. If a pod has no specified priority, the scheduler treats it as the lowest.

{% alert level="danger" %}
Do not use the `system-node-critical`, `system-cluster-critical`, `cluster-medium`, or `cluster-low` priority classes, as they are reserved for critical cluster components.
{% endalert %}

Below is a table of available priority classes.

| Priority Class             | Description                                                                                                                                                                                                                  | Value        |
|---------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
| **system-node-critical**   | Cluster components that must be present on the node. Also fully protected from eviction by kubelet. <br/> **Examples:** node-exporter, csi, and others.                                                                      | 2000001000   |
| **system-cluster-critical**| Cluster components without which the cluster cannot function properly. This PriorityClass must be assigned to MutatingWebhooks and Extension API servers. Also fully protected from eviction by kubelet. <br/> **Examples:** kube-dns, kube-proxy, cni-flannel, cni-cilium, and others. | 2000000000   |
| **production-high**        | Stateful applications in production environments whose absence leads to complete service unavailability or data loss. <br/> **Examples:** PostgreSQL, Memcached, Redis, MongoDB, and others.                                  | 9000         |
| **cluster-medium**         | Cluster components responsible for monitoring (alerts, diagnostics) and autoscaling. Without monitoring it's impossible to assess incidents; without autoscaling, applications may lack necessary resources. <br/> **Examples:** deckhouse, node-local-dns, grafana, upmeter, and others. | 7000         |
| **production-medium**      | Main stateless applications in production that serve end-users.                                                                                                                        | 6000         |
| **deployment-machinery**   | Cluster components used for build and deploy processes.                                                                                                                                | 5000         |
| **production-low**         | Production applications (cron jobs, admin panels, batch processes) that can be temporarily unavailable. If batch/cron jobs cannot be interrupted, they should be classified as `production-medium`.                        | 4000         |
| **staging**                | Staging environments for applications.                                                                                                                                                  | 3000         |
| **cluster-low**            | Non-critical but desirable cluster components. <br/> **Examples:** dashboard, cert-manager, prometheus, and others.                                                                   | 2000         |
| **develop (default)**      | Development environments for applications. Default priority class if none is specified.                                                                                                | 1000         |
| **standby**                | Not intended for applications. Used internally to reserve nodes.                                                                                                                       | -1           |
