---
title: "The extended-monitoring module: configuration"
force_searchable: true
---

<!-- SCHEMA -->

## How to use `extended-monitoring-exporter`

Attach the `extended-monitoring.deckhouse.io/enabled` label to the Namespace to enable the export of extended monitoring metrics. You can do it by:
- adding the appropriate helm-chart to the project (recommended method);
- adding it to `.gitlab-ci.yml` (kubectl patch/create);
- attaching it manually (`kubectl label namespace my-app-production extended-monitoring.deckhouse.io/enabled=""`).
- configuring via [namespace-configurator](/documentation/v1/modules/600-namespace-configurator/) module.

Any of the methods above would result in the emergence of the default metrics (+ any custom metrics with the `threshold.extended-monitoring.deckhouse.io/` prefix) for all supported Kubernetes objects in the target namespace. Note that monitoring is enabled automatically for a number of [non-namespaced](#non-namespaced-kubernetes-objects) Kubernetes objects described below.

You can also add custom labels with the specified value to `threshold.extended-monitoring.deckhouse.io/something` Kubernetes objects, e.g., `kubectl label pod test threshold.extended-monitoring.deckhouse.io/disk-inodes-warning=30`.
In this case, the label value will replace the default one.

You can disable monitoring on a per-object basis by adding the `extended-monitoring.deckhouse.io/enabled=false` label to it. Thus, the default labels will also be disabled (as well as label-based alerts).

### Standard labels and supported Kubernetes objects

Below is the list of labels used in Prometheus Rules and their default values.

**Note,** that all the labels start with the `threshold.extended-monitoring.deckhouse.io/` prefix. The value specified in a label is a number that sets the alert trigger threshold.

For example, the label `threshold.extended-monitoring.deckhouse.io/5xx-warning: "5"` on the Ingress resource changes the alert threshold from 10% (default) to 5%.

#### Non-namespaced Kubernetes objects

Non-namespaced Kubernetes objects do not need labels on the namespace, and monitoring on them is enabled by default when the module is enabled.

##### Node

| Label                                   | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 70             |
| disk-bytes-critical                     | int (percent) | 80             |
| disk-inodes-warning                     | int (percent) | 90             |
| disk-inodes-critical                    | int (percent) | 95             |
| load-average-per-core-warning           | int           | 3              |
| load-average-per-core-critical          | int           | 10             |

> **Caution!** These labels **do not** apply to `imagefs` (`/var/lib/docker` by default) and `nodefs` (`/var/lib/kubelet` by default) volumes.
The thresholds for these volumes are configured completely automatically according to the kubelet's [eviction thresholds](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).
The default values are available [here](https://github.com/kubernetes/kubernetes/blob/743e4fba6339237cc8d5c11413f76ea54b4cc3e8/pkg/kubelet/apis/config/v1beta1/defaults_linux.go#L22-L27); for more info, see the [exporter](https://github.com/deckhouse/deckhouse/blob/main/modules/340-monitoring-kubernetes/images/kubelet-eviction-thresholds-exporter/).

#### Namespaced Kubernetes objects

##### Pod

| Label                                   | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 85             |
| disk-bytes-critical                     | int (percent) | 95             |
| disk-inodes-warning                     | int (percent) | 85             |
| disk-inodes-critical                    | int (percent) | 90             |

##### Ingress

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| 5xx-warning            | int (percent) | 10            |
| 5xx-critical           | int (percent) | 20            |

##### Deployment

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

The threshold implies the number of unavailable replicas **in addition** to [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable). This threshold will be triggered if the number of unavailable replicas is greater than `maxUnavailable` by the amount specified. Suppose `replicas-not-ready` is 0. In this case, the threshold will be triggered if the number of unavailable replicas is greater than `maxUnavailable`. If `replicas-not-ready` is set to 1, then the threshold will be triggered if the number of unavailable replicas is greater than `maxUnavailable` + 1. This way, you can fine-tune this parameter for specific Deployments (that may be unavailable) in the namespace with the extended monitoring enabled to avoid getting excessive alerts.

##### StatefulSet

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

The threshold implies the number of unavailable replicas **in addition** to [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (see the comments on [Deployment](#deployment)).

##### DaemonSet

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

The threshold implies the number of unavailable replicas **in addition** to [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (see the comments on [Deployment](#deployment)).

##### CronJob

Note that only the deactivation using the `extended-monitoring.deckhouse.io/enabled=false` label is supported.

### How does it work?

The module exports specific Kubernetes object labels to Prometheus. It allows you to improve Prometheus rules by adding the thresholds for triggering alerts.
Using metrics that this module exports, you can, e.g., replace the "magic" constants in rules.

Before:

```text
(
  kube_statefulset_status_replicas - kube_statefulset_status_replicas_ready
)
> 1
```

After:

```text
(
  kube_statefulset_status_replicas - kube_statefulset_status_replicas_ready
)
> on (namespace, statefulset)
(
  max by (namespace, statefulset) (extended_monitoring_statefulset_threshold{threshold="replicas-not-ready"})
)
```
