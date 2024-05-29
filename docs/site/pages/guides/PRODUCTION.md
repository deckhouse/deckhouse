---
title: Going to Production
permalink: en/guides/production.html
---

The following recommendations may be of less importance for a test or development cluster, but they may be critical for a production one.

## Release channel and update mode

{% alert %}
Use `Early Access` or `Stable` release channel. Configure [auto-update window](/documentation/v1/modules/002-deckhouse/usage.html#update-windows-configuration) or select [manual mode](/documentation/v1/modules/002-deckhouse/usage.html#manual-update-confirmation).
{% endalert %}

Select the [release channel](/documentation/v1/deckhouse-release-channels.html) and [update mode](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-releasechannel) that suit your needs. The more stable the release channel is, the later you will have the chance to use the new features.

If possible, use different release channels for clusters. Use a less stable update channel for a development cluster than for a testing cluster or stage (pre-production) cluster.  

We recommend using the `Early Access` or `Stable` release channel for production clusters. If you have more than one cluster in a production environment, consider using different release channels for them. For example, `Early Access` for one, and `Stable` for another. If the clusters use the same release channel, we recommend setting update windows so that they do not overlap.

{% alert level="warning" %}
Even in very busy and critical clusters, it is not a good idea to disable the use of the release channel. The best strategy is a scheduled update. If you are using a Deckhouse release in your cluster that has not received an update in over six months, you will have a hard time getting help quickly should a problem arise.
{% endalert %}

The [update windows](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-update-windows) management allows you to schedule automatic Deckhouse release updates when your cluster is not experiencing peak load.

## Kubernetes version

{% alert %}
Use the automatic [Kubernetes version selection](/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) or set the version explicitly.
{% endalert %}

In most cases, we recommend opting for the automatic selection of the Kubernetes version. In Deckhouse, this behavior is set by default, but it can be changed with the [kubernetesVersion](/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter. Upgrading the Kubernetes version in the cluster has no effect on applications and is done in a [consistent and secure fashion](/documentation/v1/modules/040-control-plane-manager/#version-control).

If the automatic Kubernetes version selection is enabled, Deckhouse can upgrade the Kubernetes version in the cluster together with the Deckhouse update (when upgrading a minor version). If the Kubernetes version in the [kubernetesVersion](/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter is set explicitly, Deckhouse may not upgrade to a newer version at some point if the Kubernetes version used in the cluster is no longer supported.

You must decide for yourself whether to use automatic version selection or set a specific version and update it manually every now and then.

If your application uses outdated versions of resources or depends on a particular version of Kubernetes for some other reason, check whether it is [supported](/documentation/v1/supported_versions.html) and [set it explicitly](/documentation/v1/deckhouse-faq.html#how-do-i-upgrade-the-kubernetes-version-in-a-cluster).  

## Resource requirements

{% alert %}
Use at least 4 CPUs / 8GB RAM for infrastructure nodes. For master and monitoring nodes, fast disks are recommended.
{% endalert %}

The following resource minimums are recommended for infrastructure nodes, depending on their role in the cluster:
- **Master node** — 4 CPU, 8GB RAM, 60 GB of disk space for the cluster and etcd data on a fast disk (400+ IOPS);  
- **Frontend node** — 2 CPU, 4GB RAM, 50 GB of disk space;
- **Monitoring node** (for high-load clusters) — 4 CPU, 8GB RAM, 50 GB of disk space on a fast disk (400+ IOPS).
- **System node**:
  - 2 CPU, 4 RAM, 50 GB of disk space — if there are dedicated monitoring nodes in the cluster;
  - 4 CPU, 8 RAM, 50 GB of disk space on a fast disk (400+ IOPS) — if there are no dedicated monitoring nodes in the cluster.
- **Worker nodes** — the requirements are similar to those for the master node, but largely depend on the nature of the load running on the nodes.

Estimates of the resources required for the clusters to run:
- **Regular cluster**: 3 master nodes, 2 frontend nodes, 2 system nodes. Such a configuration requires **at least 24 CPUs and 48GB RAM** along with fast 400+ IOPS disks for the master nodes.
- **High-load cluster** (with dedicated monitoring nodes): 3 master nodes, 2 frontend nodes, 2 system nodes, 2 monitoring nodes. Such a configuration requires **at least 28 CPUs and 64GB RAM** along with fast 400+ IOPS disks for the master and monitoring nodes.
- We recommend setting up a dedicated [storageClass](/documentation/v1/deckhouse-configure-global.html#parameters-storageclass) on the fast disks for Deckhouse components.
- Add worker nodes to this, taking into account the nature of the workloads.

## Things to consider when configuring

### Master nodes

{% alert %}
Three master nodes with fast 400+ IOPS disks are highly recommended for a cluster.
{% endalert %}

Use three master nodes in all cases, as they are sufficient for fault tolerance. Also, with three nodes, you can safely update the cluster's control plane as well as the master nodes. Extra master nodes are not needed, while 2 nodes (or any even number) do not make a quorum.

The master node configuration for cloud clusters can be configured using the [masterNodeGroup](/documentation/v1/modules/030-cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup) parameter.

Reference:
- [How do I add a master node to a cluster...](/documentation/v1/modules/040-control-plane-manager/faq.html#how-do-i-add-a-master-nodes-to-a-cloud-cluster-single-master-to-a-multi-master)
- [Working with static nodes...](/documentation/latest/modules/040-node-manager/#working-with-static-nodes)

### Frontend nodes

{% alert %}
Use two or more frontend nodes.

Use inlet `LoadBalancer` for OpenStack-based clouds and cloud services where automatic balancer ordering is not supported (AWS, GCP, Azure, etc.). Use inlet  `HostPort` with an external load balancer for bare metal or vSphere.
{% endalert %}

Frontend nodes are used for balancing incoming traffic. Such nodes are allocated for Ingress controllers. The [NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup) of the frontend nodes has a `node-role.deckhouse.io/frontend` label. Read more about [allocating nodes for specific load types...](/documentation/v1/#advanced-scheduling)

Use more than one frontend node. Frontend nodes must be able to still handle traffic even if one of the frontend nodes fails.

For example, if the cluster has two frontend nodes, each frontend node must be able to handle the entire cluster load in case the second frontend node fails. If the cluster has three frontend nodes, each frontend node must be able to handle a load that is at least one and a half times higher.

Select the [inlet type](/documentation/v1/modules/402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-inlet) (it defines the way the traffic comes in).  

When deploying a cluster using Deckhouse in a cloud infrastructure where provisioning of load balancers is supported (e.g., OpenStack-based clouds, AWS, GCP, Azure, etc.), use the `LoadBalancer` or `LoadBalancerWithProxyProtocol` inlet.

In environments where automatic load balancer provisioning is not supported (bare metal clusters, vSphere, custom OpenStack solutions), use the `HostPort` or `HostPortWithProxyProtocol` inlet. In this case, you can either add some A&#8209;records to DNS for the corresponding domain or use an external load-balancing service (e.g., Cloudflare, Qrator solutions, or configure metallb).

{% alert level="warning" %}
The `HostWithFailover` inlet is suitable for clusters with a single frontend node. It reduces the time that the Ingress controller is unavailable during updates. This type of inlet is suitable for important development environments, but **not recommended for production**.
{% endalert %}

The algorithm for choosing an inlet:

![The algorithm for choosing an inlet]({{ assets["guides/going_to_production/ingress-inlet.svg"].digest_path }})

### Monitoring nodes

{% alert %}
For high-load clusters, use two monitoring nodes equipped with fast disks.
{% endalert %}

Monitoring nodes are used to run Grafana, Prometheus, and other monitoring components. The [NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup) for monitoring nodes has the `node-role.deckhouse.io/monitoring` label attached.

In high-load clusters, where many alerts are generated and many metrics are collected, we recommend allocating dedicated nodes for monitoring. If not, monitoring components will be deployed to [system nodes](#system-nodes).

When allocating monitoring nodes, it is important to allocate fast disks to them. You can do so by providing a dedicated `storageClass` on fast disks for all Deckhouse components (global parameter [storageClass](/documentation/v1/deckhouse-configure-global.html#parameters-storageclass)) or allocate a dedicated `storageClass` to monitoring components only ([storageClass](/documentation/v1/modules/300-prometheus/configuration.html#parameters-storageclass) and [longtermStorageClass](/documentation/v1/modules/300-prometheus/configuration.html#parameters-longtermstorageclass) parameters of the `prometheus` module).

### System nodes

{% alert %}
Dedicate two system nodes.
{% endalert %}

System nodes are used to run Deckhouse modules. Their [NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup) has the `node-role.deckhouse.io/system` label.

Set two nodes to be system nodes. This way, Deckhouse modules will run on them without interfering with user applications in the cluster. Read more about [allocating nodes to specific load types...](/documentation/v1/#advanced-scheduling).

It is recommended to provide the Deckhouse components with fast disks (the [storageClass](/documentation/v1/deckhouse-configure-global.html#parameters-storageclass) global parameter).

## Configuring alerts

{% alert %}
You can send alerts using the [internal](/documentation/v1/modules/300-prometheus/faq.html#how-do-i-add-alertmanager) Alertmanager or connect the [external](/documentation/v1/modules/300-prometheus/faq.html#how-do-i-add-an-additional-alertmanager) one.
{% endalert %}

Monitoring will work out of the box once Deckhouse is installed, however, it is not enough for production clusters. Configure the Alertmanager [built in](/documentation/v1/modules/300-prometheus/faq.html#how-do-i-add-alertmanager) Deckhouse  or [connect your](/documentation/v1/modules/300-prometheus/faq.html#how-do-i-add-an-additional-alertmanager) own Alertmanager to receive incident notifications.

Using the [CustomAlertmanager](/documentation/v1/modules/300-prometheus/cr.html#customalertmanager) custom resource, you can configure sending alerts to an [e-mail](/documentation/v1/modules/300-prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-emailconfigs), [Slack](/documentation/v1/modules/300-prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-slackconfigs), [Telegram](/documentation/v1/modules/300-prometheus/usage.html#sending-alerts-to-telegram), via the [webhook](/documentation/v1/modules/300-prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-webhookconfigs), or by other means.

## Collecting logs

{% alert %}
[Configure](/documentation/v1/modules/460-log-shipper/) centralized log collection.
{% endalert %}

Set up centralized log collection from system and user applications using the [log-shipper](/documentation/v1/modules/460-log-shipper/) module.

All you have to do is to create a custom resource specifying *what to collect*: [ClusterLoggingConfig](/documentation/v1/modules/460-log-shipper/cr.html#clusterloggingconfig) or [PodLoggingConfig](/documentation/v1/modules/460-log-shipper/cr.html#podloggingconfig); and create a custom resource that specifies where to *send* the collected logs: [ClusterLogDestination](/documentation/v1/modules/460-log-shipper/cr.html#clusterlogdestination).

Reference:
- [Grafana Loki example](/documentation/v1/modules/460-log-shipper/examples.html#getting-logs-from-all-cluster-pods-and-sending-them-to-loki)
- [Logstash example](/documentation/v1/modules/460-log-shipper/examples.html#simple-logstash-example)
- [Splunk example](/documentation/v1/modules/460-log-shipper/examples.html#splunk-integration)

## Backups

{% alert %}
Set up [etcd backups](/documentation/v1/modules/040-control-plane-manager/faq.html#how-do-make-etcd-backup). Have a backup plan ready at all times.
{% endalert %}

We strongly advise you to set up [etcd backups](/documentation/v1/modules/040-control-plane-manager/faq.html#how-do-make-etcd-backup) as a bare minimum. This will be your last chance to restore the cluster should things go awry. Keep these backups as *away* from your cluster as possible.

The backups won't help if they don't work or if you don't know how to use them to recover the cluster. The best practice is to compile a [Disaster Recovery Plan](https://www.google.com/search?q=Disaster+Recovery+Plan) with specific steps and commands to restore the cluster from a backup.

This plan should be periodically updated and tested in drills.

## Community

{% alert %}
Follow the project channel on [Telegram](https://t.me/deckhouse) for news and updates.
{% endalert %}

Join the [community](https://deckhouse.io/community/about.html) to keep up with important news and developments. This will help you to share experiences with people who are doing the same thing as you are and avoid typical problems.

The Deckhouse team knows what it takes to do production in Kubernetes. We'd love to see you succeed with Deckhouse. Share your success stories and inspire others to switch to Kubernetes.
