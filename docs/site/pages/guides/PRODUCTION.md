---
title: Going to production
permalink: en/guides/production.html
description: Recommendations for preparing Deckhouse Platform cluster for production environment.
layout: sidebar-guides
---

The following recommendations may be of less importance for a test or development cluster, but they may be critical for a production one.

## Release channel and update mode

{% alert %}
Use the `EarlyAccess` or `Stable` release channel. Configure [auto-update window](/modules/deckhouse/usage.html#update-windows-configuration) or select [manual mode](/modules/deckhouse/usage.html#manual-update-confirmation).
{% endalert %}

Select the [release channel](/products/kubernetes-platform/documentation/v1/reference/release-channels.html) and [update mode](/modules/deckhouse/configuration.html#parameters-update-mode) that suit your needs. The more stable the release channel is, the later you will have the chance to use the new features.

If possible, use different release channels for clusters. Use a less stable update channel for a development cluster than for a testing cluster or stage (pre-production) cluster.

Use the `EarlyAccess` or `Stable` release channel for production clusters. If you have more than one cluster in a production environment, consider using different release channels for them. For example, `EarlyAccess` for one, and `Stable` for another. If the clusters use the same release channel, set update windows so that they do not overlap.

{% alert level="warning" %}
Even in very busy and critical clusters, it is not a good idea to disable the use of the release channel. The best strategy is a scheduled update. If you are using a Deckhouse Platform (DP) release in your cluster that has not received an update in over six months, you will have a hard time getting help quickly should a problem arise.
{% endalert %}

The [update windows](/modules/deckhouse/configuration.html#parameters-update-windows) management allows you to schedule automatic DP release updates when your cluster is not experiencing peak load.

## Kubernetes version

{% alert %}
Use the automatic [Kubernetes version selection](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) or set the version explicitly.
{% endalert %}

In most cases, opt for the automatic selection of the Kubernetes version. In DP, this behavior is set by default, but it can be changed with the [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter. Upgrading the Kubernetes version in the cluster has no effect on applications and is done in a [consistent and secure fashion](/modules/control-plane-manager/#version-control).

If the automatic Kubernetes version selection is enabled, DP can upgrade the Kubernetes version in the cluster together with the DP update (when upgrading a minor version). If the Kubernetes version in the [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter is set explicitly, DP may not upgrade to a newer version at some point if the Kubernetes version used in the cluster is no longer supported.

You must decide for yourself whether to use automatic version selection or set a specific version and update it manually every now and then.

If your application uses outdated versions of resources or depends on a particular version of Kubernetes for some other reason, check whether it is [supported](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html) and [set it explicitly](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/updating-and-versioning.html).

## Resource requirements

{% alert %}
Use at least 4 CPUs / 8 GB RAM for infrastructure nodes (2 CPUs / 4 GB RAM for frontend nodes). For master and monitoring nodes, fast disks are recommended.
{% endalert %}

The following resource minimums are recommended for infrastructure nodes, depending on their role in the cluster:

- **Master node** — 8 CPU, 16GB RAM, 60 GB of disk space for the cluster and etcd data on a fast disk (400+ IOPS);
- **Frontend node** — 2 CPU, 4GB RAM, 50 GB of disk space;
- **Monitoring node** (for high-load clusters) — 6 CPU, 12GB RAM, <a href="#storage">50 / 150*</a> GB of disk space on a fast disk (400+ IOPS).
- **System node**:
  - 4 CPU, 8 GB RAM, <a href="#storage">50 / 150*</a> GB of disk space — if there are dedicated monitoring nodes in the cluster;
  - 8 CPU, 16 GB RAM, <a href="#storage">60 / 160*</a> GB of disk space on a fast disk (400+ IOPS) — if there are no dedicated monitoring nodes in the cluster.
- **Worker node** — the requirements are similar to those for the master node, but largely depend on the nature of the load running on the node (nodes).

Additional recommendations:<span id="storage"></span>

- If system PVCs (of the `prometheus`, `upmeter` modules and others) are stored on the node's local disk, additionally allocate at least 100 GB of free space. The second value of the disk space for monitoring and system nodes above (for example, 150 in "50 / 150") includes these 100 GB.
- For system services (kubelet) and system pods on each worker node, reserve at least 1 CPU and 2 GB of RAM.
- For all nodes, use fast disks with performance of at least 400 IOPS.

Estimates of the resources required for the clusters to run:

- **Regular cluster**: 3 master nodes, 2 frontend nodes, 2 system nodes. Such a configuration requires **at least 44 CPUs and 88 GB RAM** along with fast 400+ IOPS disks for the master nodes.
- **High-load cluster** (with dedicated monitoring nodes): 3 master nodes, 2 frontend nodes, 2 system nodes, 2 monitoring nodes. Such a configuration requires **at least 48 CPUs and 96 GB RAM** along with fast 400+ IOPS disks for the master and monitoring nodes.
- Set up a dedicated [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass) on the fast disks for DP components.
- Add worker nodes to this, taking into account the nature of the workloads.

Also read [the instructions](./hardware-requirements.html) on the hardware requirements for cluster resources, which describes in detail how to select the necessary resources depending on the expected load.

## Things to consider when configuring

### Master nodes

{% alert %}
Three master nodes with fast 400+ IOPS disks are highly recommended for a cluster.
{% endalert %}

Use three master nodes in all cases, as they are sufficient for fault tolerance. Also, with three nodes, you can safely update the cluster's control plane as well as the master nodes. Extra master nodes are not needed, while 2 nodes (or any even number) do not make a quorum.

The master node configuration for cloud clusters can be configured using the [masterNodeGroup](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup) parameter.

Reference:

- [How do I add a master node to a cluster...](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#adding-master-nodes-in-a-cloud-cluster)
- [Working with static nodes...](/modules/node-manager/#working-with-static-nodes)

### Frontend nodes

{% alert %}
Use two or more frontend nodes.

Use inlet `LoadBalancer` for OpenStack-based clouds and cloud services where automatic balancer ordering is not supported (AWS, GCP, Azure, etc.). Use inlet  `HostPort` with an external load balancer for bare metal or vSphere.
{% endalert %}

Frontend nodes are used for balancing incoming traffic. Such nodes are allocated for Ingress controllers. The [NodeGroup](/modules/node-manager/cr.html#nodegroup) of the frontend nodes has a `node-role.deckhouse.io/frontend` label. Read more about [allocating nodes for specific load types...](/products/kubernetes-platform/documentation/v1/admin/configuration/#advanced-scheduling)

Use more than one frontend node. Frontend nodes must be able to still handle traffic even if one of the frontend nodes fails.

For example, if the cluster has two frontend nodes, each frontend node must be able to handle the entire cluster load in case the second frontend node fails. If the cluster has three frontend nodes, each frontend node must be able to handle a load that is at least one and a half times higher.

Select the [inlet type](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-inlet) (it defines the way the traffic comes in).

When deploying a cluster using DP in a cloud infrastructure where provisioning of load balancers is supported (e.g., OpenStack-based clouds, AWS, GCP, Azure, etc.), use the `LoadBalancer` or `LoadBalancerWithProxyProtocol` inlet.

In environments where automatic load balancer provisioning is not supported (bare metal clusters, vSphere, custom OpenStack solutions), use the `HostPort` or `HostPortWithProxyProtocol` inlet. In this case, you can either add some A&#8209;records to DNS for the corresponding domain or use an external load-balancing service (e.g., Cloudflare, Qrator solutions, or configure metallb).

{% alert level="warning" %}
The `HostWithFailover` inlet is suitable for clusters with a single frontend node. It reduces the time that the Ingress controller is unavailable during updates. This type of inlet is suitable for important development environments, but **not recommended for production**.
{% endalert %}

The algorithm for choosing an inlet:

![The algorithm for choosing an inlet](/images/guides/going_to_production/ingress-inlet.svg)

### Monitoring nodes

{% alert %}
For high-load clusters, use two monitoring nodes equipped with fast disks.
{% endalert %}

Monitoring nodes are used to run Grafana, Prometheus, and other monitoring components. The [NodeGroup](/modules/node-manager/cr.html#nodegroup) for monitoring nodes has the `node-role.deckhouse.io/monitoring` label attached.

In high-load clusters, where many alerts are generated and many metrics are collected, allocate dedicated nodes for monitoring. If not, monitoring components will be deployed to [system nodes](#system-nodes).

When allocating monitoring nodes, it is important to allocate fast disks to them. You can do so by providing a dedicated `storageClass` on fast disks for all DP components (global parameter [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass)) or allocate a dedicated `storageClass` to monitoring components only [storageClass](/modules/prometheus/configuration.html#parameters-storageclass) and [longtermStorageClass](/modules/prometheus/configuration.html#parameters-longtermstorageclass) parameters of the `prometheus` module.

If the cluster is initially created with nodes allocated for a specific type of workload (system nodes, nodes for monitoring, etc.), explicitly specify the corresponding `nodeSelector` parameter in the module configuration for modules that use persistent storage volumes (for example, for the `prometheus` module). For the `prometheus` module, this parameter is [nodeSelector](/modules/prometheus/configuration.html#parameters-nodeselector).

### System nodes

{% alert %}
Dedicate two system nodes.
{% endalert %}

System nodes are used to run DP modules. Their [NodeGroup](/modules/node-manager/cr.html#nodegroup) has the `node-role.deckhouse.io/system` label.

Set two nodes to be system nodes. This way, DP modules will run on them without interfering with user applications in the cluster. Read more about [allocating nodes to specific load types...](/products/kubernetes-platform/documentation/v1/admin/configuration/#advanced-scheduling).

Provide the DP components with fast disks (the [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass) global parameter).

## Configuring alerts

{% alert %}
You can send alerts using the [internal](/modules/prometheus/faq.html#how-do-i-add-alertmanager) Alertmanager or connect the [external](/modules/prometheus/faq.html#how-do-i-add-an-additional-alertmanager) one.
{% endalert %}

Monitoring will work once DP is installed, however, it is not enough for production clusters. Configure the Alertmanager [built in](/modules/prometheus/faq.html#how-do-i-add-alertmanager) DP or [connect your](/modules/prometheus/faq.html#how-do-i-add-an-additional-alertmanager) own Alertmanager to receive incident notifications.

Using the [CustomAlertmanager](/modules/prometheus/cr.html#customalertmanager) custom resource, you can configure sending alerts to an [e-mail](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-emailconfigs), [Slack](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-slackconfigs), [Telegram](/modules/prometheus/usage.html#sending-alerts-to-telegram), via the [webhook](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-webhookconfigs), or by other means.

For the list of all available alerts in the Deckhouse Platform monitoring system, refer to the [corresponding documentation page](/products/kubernetes-platform/documentation/v1/reference/alerts.html).

## Collecting logs

{% alert %}
[Configure](/modules/log-shipper/) centralized log collection.
{% endalert %}

Set up centralized log collection from system and user applications using the [log-shipper](/modules/log-shipper/) module.

All you have to do is to create a custom resource specifying *what to collect*: [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) or [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig); and create a custom resource that specifies where to *send* the collected logs: [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination).

Reference:

- [Grafana Loki example](/modules/log-shipper/examples.html#getting-logs-from-all-cluster-pods-and-sending-them-to-loki)
- [Logstash example](/modules/log-shipper/examples.html#simple-logstash-example)
- [Splunk example](/modules/log-shipper/examples.html#splunk-integration)

## Backups

{% alert %}
Set up [etcd backups](/modules/control-plane-manager/faq.html#how-to-manually-backup-etcd). Have a backup plan ready at all times.
{% endalert %}

At a minimum, set up [etcd backups](/modules/control-plane-manager/faq.html#how-to-manually-backup-etcd). This will be your last chance to restore the cluster should things go awry. Keep these backups as *away* from your cluster as possible.

The backups won't help if they don't work or if you don't know how to use them to recover the cluster. Compile a Disaster Recovery Plan (DRP) with specific steps and commands to restore the cluster from a backup. The restore procedures are described in the [Backup and restore](/products/kubernetes-platform/documentation/v1/admin/configuration/backup/backup-and-restore.html) section.

This plan should be periodically updated and tested in drills.

## Community

{% alert %}
Follow the project channel on [Telegram](https://t.me/deckhouse) for news and updates.
{% endalert %}

Join the [community](https://deckhouse.io/community/about.html) to keep up with important news and developments. This will help you to share experiences with people who are doing the same thing as you are and avoid typical problems.

Running production in Kubernetes takes effort. Share your experience with DP in the community to help others switch to Kubernetes.
