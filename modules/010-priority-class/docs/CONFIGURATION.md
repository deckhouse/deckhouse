---
title: "The priority-class module: configuration"
---

This module is enabled by **default**.

The Pod specification must contain the appropriate [priorityClassName](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#pod-priority).
It is essential to set the `priorityClassName` correctly. If in doubt, get your colleagues to help you.

> Any `priorityClassName` set to a Pod cannot lower its priority because the scheduler considers Pods without the `priority-class` as having the lowest (`develop`) priority.

Below is the list of priority classes set by the module (sorted by the priority, starting with the higher one).
**Caution!** Note that you cannot use the following PriorityClasses: `system-node-critical`, `system-cluster-critical`, `cluster-medium`, `cluster-low`.

| Priority Class            | Description                                                                                                                                                         | Value      |
|---------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| `system-node-critical`    | Cluster components that are must to be present on the node. This priority class fully protects components against [eviction by kubelet](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).<br>`node-exporter`, `csi`                             | 2000001000 |
| `system-cluster-critical` | Cluster components that are critical to its correct operation. This PriorityClass is mandatory for MutatingWebhooks and Extension API servers. It also fully protects components against [eviction by kubelet](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).<br>`kube-dns`, `coredns`, `kube-proxy`, `flannel`, `kube-api-server`, `kube-controller-manager`, `kube-scheduler`, `cluster-autoscaler`, `machine-controller-manager`.                             | 2000000000 |
| `production-high`         | Stateful applications in the production environment. Their unavailability leads to service downtime or data loss (postgresql, memcached, redis, mongo, etc.). | 9000       |
| `cluster-medium`          | Cluster components responsible for monitoring (alerts, diagnostic tools) and autoscaling. Monitoring tools help engineers assess the scale of incidents; autoscaling provides the necessary resources to applications.<br>`deckhouse`, `node-local-dns`, `kube-state-metrics`, `madison-proxy`, `node-exporter`, `trickster`, `grafana`, `kube-router`, `monitoring-ping`, `okmeter`, `smoke-mini`                      | 7000       |
| `production-medium`       | Main stateless applications in the production environment that are responsible for operating the service for end-users.                                                            | 6000       |
| `deployment-machinery`    | Cluster components that are responsible for deploying/building (helm, werf).<br>`kube-system/tiller-deploy`                                                                                 | 5000       |
| `production-low`          | Non-critical, secondary applications in the production environment (crons, admin dashboards, batch processing). For important batch or cron jobs, consider assigning them the production-medium priority.                                          | 4000       |
| `staging`                 | Staging environments for applications.                                                                                                                                    | 3000       |
| `cluster-low`             | Cluster components that are desirable but not essential for proper cluster operation. <br>`prometheus-operator`, `dashboard`, `dashboard-oauth2-proxy`, `cert-manager`, `prometheus`, `prometheus-longterm`                                                                              | 2000       |
| `develop` (default)       | Dev-environments for applications. The default class for a component (if other priority classes aren't set).                                                                               | 1000       |
| `standby`                 | This class is not intended for applications. It is used for system purposes (reserving nodes).                                                                       | -1         |
