---
title: Going to Production with DVP
permalink: en/virtualization-platform/guides/production.html
documentation_state: develop
description: Recommendations on preparing Deckhouse Virtualization Platform for a production environment.
lang: en
---

The following recommendations are essential for a production cluster, but may be irrelevant for a testing or development one.

## Release channel and update mode

{% alert %}
Use `Early Access` or `Stable` release channel.
Configure the [auto-update window](/modules/deckhouse/usage.html#update-windows-configuration) or select [manual mode](/modules/deckhouse/usage.html#manual-update-confirmation).
{% endalert %}

Select the [release channel](../documentation/about/release-channels.html) and [update mode](../documentation/admin/update/update.html) that suit your needs.
The more stable the release channel is, the longer you will have to wait before you can use new features.

If possible, use separate release channels for clusters.
For a development cluster, you may want to use a less stable release channel than for a testing or stage (pre-production) cluster.

We recommend using the `Early Access` or `Stable` release channel for production clusters.
If you have more than one cluster in a production environment, consider using separate release channels for them.
For example, you can use `Early Access` for the first cluster and `Stable` for the second.
If you can't use separate release channels, we recommend setting update windows so that they don't overlap.  

{% alert level="warning" %}
Even in very busy and critical clusters, it's recommended that you don't disable the release channel.
The best strategy is to configure scheduled updates.
If you are using a Deckhouse release in your cluster that hasn't been updated in over six months,
it may contain bugs that have long been eliminated in newer versions.
In such a case, you make it difficult, if not impossible, to address issues promptly.
{% endalert %}

The [update windows](/modules/deckhouse/configuration.html#parameters-update-windows) management lets you schedule automatic Deckhouse release updates during periods of low cluster activity.

## Kubernetes version

{% alert %}
Use the automatic [Kubernetes version selection](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) or set the version explicitly.
{% endalert %}

In most cases, we recommend opting for the automatic selection of the Kubernetes version.
In Deckhouse, this behavior is set by default, but it can be changed with the [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter.
Upgrading the Kubernetes version in the cluster has no effect on applications and is done [consistently and securely](/modules/control-plane-manager/#version-control).

If the automatic Kubernetes version selection is enabled,
Deckhouse can upgrade the Kubernetes version in the cluster together with the Deckhouse update (when upgrading a minor version).
If the Kubernetes version in the [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter is set explicitly,
Deckhouse may refuse to upgrade to a newer version at some point if the Kubernetes version used in the cluster is no longer supported.

If your application uses outdated resource versions or depends on a particular version of Kubernetes for some other reason,
check whether it's [supported](/products/virtualization-platform/documentation/about/requirements.html) and [set it explicitly](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/updating-and-versioning.html).

## Resource requirements

{% alert %}
Use at least 4 CPUs and 8 GB RAM for infrastructure nodes.
For master and monitoring nodes, fast disks are recommended.
Note that when using software-defined storage, additional disks will be required to store data on the nodes.
{% endalert %}

The following resource minimums are recommended for infrastructure nodes, depending on their cluster role:

- **Master node**: 4 CPU, 8 GB RAM, 60 GB of disk space on a fast disk (400+ IOPS)
- **Frontend node**: 2 CPU, 4 GB RAM, 50 GB of disk space
- **Monitoring node** (for high-load clusters): 4 CPU, 8 GB RAM, 50 GB of disk space on a fast disk (400+ IOPS)
- **System node**:
  - 2 CPU, 4 GB RAM, 50 GB of disk space — if there are dedicated monitoring nodes in the cluster
  - 4 CPU, 8 GB RAM, 60 GB of disk space on a fast disk (400+ IOPS) — if there are no dedicated monitoring nodes in the cluster.
- **Worker node**: The requirements are similar to those for the master node,
  but largely depend on the nature of the workload running on a node.

Estimates of the resources required for the clusters to run:

- **Regular cluster**: 3 master nodes, 2 frontend nodes, and 2 system nodes.
  Such a configuration requires **at least 24 CPUs and 48 GB of RAM** along with fast 400+ IOPS disks for the master nodes.
- **High-load cluster** (with dedicated monitoring nodes): 3 master nodes, 2 frontend nodes, 2 system nodes, and 2 monitoring nodes.
  Such a configuration requires **at least 28 CPUs and 64 GB of RAM** along with fast 400+ IOPS disks for the master and monitoring nodes.
- We recommend setting up a dedicated [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-storageclass) on fast disks for the Deckhouse components.
- Add worker nodes to this, taking into account the workload conditions.

## Things to consider

### Master nodes

{% alert %}
A cluster must include three master nodes with fast 400+ IOPS disks.
{% endalert %}

Always use three master nodes to ensure fault tolerance and safe updates.
Any extra master nodes aren't necessary, and using 2 nodes (or any other even number) prevents achieving quorum.

Related guides:

- [Working with static nodes...](/products/kubernetes-platform/documentation/v1/architecture/node.html#working-with-static-nodes)

### Frontend nodes

{% alert %}
Use two or more frontend nodes.

Use the `HostPort` inlet with an external load balancer.
{% endalert %}

Frontend nodes are used for balancing incoming traffic.
Such nodes are allocated for Ingress controllers.
The NodeGroup of the frontend nodes has the `node-role.deckhouse.io/frontend` label.
For details on allocating nodes for specific load types, refer to [Advanced scheduling](/products/kubernetes-platform/documentation/v1/#advanced-scheduling).

Use more than one frontend node.
Frontend nodes must be able to continue handling traffic even if one of them fails.

For example, if the cluster has two frontend nodes,
each of them must be able to handle the entire cluster load in case the second frontend node fails.
If the cluster has three frontend nodes,
each of them must be able to handle a load that is at least one and a half times higher.

The Deckhouse platform supports three sources of incoming traffic:

- `HostPort`: Installs an Ingress controller accessible on node ports via `hostPort`.
- `HostPortWithProxyProtocol`: Installs an Ingress controller accessible on node ports via `hostPort`.
  It uses the proxy protocol to obtain the client's real IP address.
- `HostWithFailover`: Installs two Ingress controllers, a primary one and a standby.

{% alert level="warning" %}
The `HostWithFailover` inlet is suitable for clusters with a single frontend node.
It reduces downtime for the Ingress controller during updates.
This inlet type is suitable for important development environments, however, it is **not recommended for production**.
{% endalert %}

For details on network configuration, refer to [VM Network](/products/virtualization-platform/documentation/admin/platform-management/network/vm-network.html).

### Monitoring nodes

{% alert %}
For high-load clusters, use two monitoring nodes equipped with fast disks.
{% endalert %}

Monitoring nodes are used to run Grafana, Prometheus, and other monitoring components.
The [NodeGroup](/modules/node-manager/cr.html#nodegroup) for monitoring nodes has the `node-role.deckhouse.io/monitoring` label.

In high-load clusters, where many alerts are generated and many metrics are collected,
we recommend allocating dedicated nodes for monitoring.
Otherwise, monitoring components will be deployed to [system nodes](#system-nodes).

When allocating monitoring nodes, it's important to equip them with fast disks.
You can do so by providing a dedicated `storageClass` on fast disks for all Deckhouse components (the [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-storageclass) global parameter)
or allocating a dedicated `storageClass` to monitoring components only
([storageClass](/modules/prometheus/configuration.html#parameters-storageclass) and [longtermStorageClass](/modules/prometheus/configuration.html#parameters-longtermstorageclass) parameters of the `prometheus` module).

If the cluster is initially created with nodes allocated for a specific type of workload, such as system nodes, monitoring nodes, and so on,
we recommend that you explicitly specify the corresponding nodeSelector in the configuration of modules using persistent storage volumes.
For example, for the `prometheus` module, this parameter is [nodeSelector](/modules/prometheus/configuration.html#parameters-nodeselector).

### System nodes

{% alert %}
Use two system nodes.
{% endalert %}

System nodes are used to run Deckhouse modules.
Their [NodeGroup](/modules/node-manager/cr.html#nodegroup) has the `node-role.deckhouse.io/system` label.

Allocate two system nodes.
This way, Deckhouse modules will run on them without interfering with user applications in the cluster.
For details on allocating nodes for specific load types, refer to [Advanced scheduling](/products/kubernetes-platform/documentation/v1/#advanced-scheduling).

Note that fast disks are recommended for the Deckhouse components (the [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-storageclass) global parameter).

## Monitoring alerts

{% alert %}
You can configure alerts using the [internal Alertmanager](/modules/prometheus/faq.html#how-do-i-add-alertmanager) or connect the [external one](/modules/prometheus/faq.html#how-do-i-add-an-additional-alertmanager).
{% endalert %}

Monitoring will work out of the box once Deckhouse is installed, but it's not enough for production clusters.
To receive alerts about incidents, configure the [built-in](/modules/prometheus/faq.html#how-do-i-add-alertmanager) Deckhouse Alertmanager or [connect your own](/modules/prometheus/faq.html#how-do-i-add-an-additional-alertmanager) Alertmanager.

Using the [CustomAlertmanager](/modules/prometheus/cr.html#customalertmanager) custom resource, you can set up alerts to be sent to [e-mail](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-emailconfigs), [Slack](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-slackconfigs), [Telegram](/modules/prometheus/usage.html#sending-alerts-to-telegram), via [webhooks](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-webhookconfigs), or by other means.

For the list of all available alerts in the monitoring system, refer to the [corresponding documentation page](/products/kubernetes-platform/documentation/v1/reference/alerts.html).

<!-- ## Logging

{% alert %}
Configure centralized logging using the [log-shipper](/modules/log-shipper/) module.
{% endalert %}

Configure centralized logging from system and user applications using the [log-shipper](/modules/log-shipper/) module.

All you have to do is to create a custom resource specifying *what to collect* ([ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) or [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig))
and another custom resource specifying *where to send* the collected logs: [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination).

Related guides:

- [Grafana Loki example](/modules/log-shipper/examples.html#getting-logs-from-all-cluster-pods-and-sending-them-to-loki)
- [Logstash example](/modules/log-shipper/examples.html#simple-logstash-example)
- [Splunk example](/modules/log-shipper/examples.html#splunk-integration)
-->

## Backups

{% alert %}
Set up [etcd backups](/products/virtualization-platform/documentation/admin/backup-and-restore.html#backing-up-etcd).
Write up a recovery plan.
{% endalert %}

We strongly recommend that you set up [etcd backups](/products/virtualization-platform/documentation/admin/backup-and-restore.html#backing-up-etcd).
This will be your last chance to restore the cluster should things go awry.
Keep these backups as *far away* from your cluster as possible.

The backups won't help if they don't work or if you don't know how to use them for recovery.
The best practice is compiling a [Disaster Recovery Plan](https://www.google.com/search?q=Disaster+Recovery+Plan)
with specific steps and commands to restore the cluster from a backup.

This plan should be periodically updated and tested in drills.

## Community

{% alert %}
Follow the project channel on [Telegram](https://t.me/deckhouse) for news and updates.
{% endalert %}

To keep up with important news and updates, join the [community](/community/).
This will help you share experience with people who are doing the same thing as you are and avoid typical mistakes.

The Deckhouse team understands the challenges of managing a production cluster in Kubernetes.
We'd love to see you succeed with Deckhouse.
Share your success stories and inspire others to switch to Kubernetes.
