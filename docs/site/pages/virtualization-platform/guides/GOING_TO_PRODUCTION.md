---
title: Going to Production with DVP
permalink: en/virtualization-platform/guides/production.html
documentation_state: develop
description: Recommendations on preparing Deckhouse Virtualization Platform for a production environment.
lang: en
---

The following recommendations are essential for a production cluster and may be irrelevant for a testing or development one.

## Release channel and update mode

{% alert level="info" %}
Use `Early Access` or `Stable` release channel.
Configure the [auto-update window](/modules/deckhouse/usage.html#update-windows-configuration) or select [manual mode](/modules/deckhouse/usage.html#manual-update-confirmation).
{% endalert %}

Select the [release channel](../documentation/about/release-channels.html) and [update mode](../documentation/admin/update/update.html) in accordance with your requirements. The more stable the release channel is, the later new features appear in it.

If possible, use separate release channels for different clusters. For a development cluster, use a less stable release channel than for a testing or stage (pre-production) cluster.

We recommend using the `Early Access` or `Stable` release channel for production clusters.
If you have more than one cluster in a production environment, consider using separate release channels for them.
For example, you can use `Early Access` for the first cluster and `Stable` for the second.
If you can't use separate release channels, we recommend setting update windows so that they don't overlap.  

{% alert level="warning" %}
Even in very busy and critical clusters, it's recommended that you don't disable the release channel.
The best strategy is to configure scheduled updates.
In clusters that haven't been updated for six months or more, there may be bugs that, as a rule, have already been fixed in newer versions.
In such a case, it will be difficult to resolve the issue promptly.
{% endalert %}

The [update windows](/modules/deckhouse/configuration.html#parameters-update-windows) management lets you schedule automatic platform release updates during periods of low cluster activity.

## Kubernetes version

{% alert level="info" %}
Use the automatic [Kubernetes version selection](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) or set the version explicitly.
{% endalert %}

In most cases, we recommend opting for the automatic selection of the Kubernetes version.
In the platform, this behavior is set by default, but it can be changed with the [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter.
Upgrading the Kubernetes version in the cluster has no effect on applications and is done [consistently and securely](/modules/control-plane-manager/#version-control).

If automatic Kubernetes version selection is enabled, the platform can upgrade the Kubernetes version in the cluster when updating the platform release (when upgrading a minor version). If the Kubernetes version is explicitly specified in the [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter, the platform update may fail if the Kubernetes version used in the cluster is no longer supported.

If your application uses outdated resource versions or depends on a particular version of Kubernetes for some other reason,
check whether it's [supported](/products/virtualization-platform/documentation/about/requirements.html) and [set it explicitly](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#how-do-i-upgrade-the-kubernetes-version-in-a-cluster).

## Cluster architecture selection

Deckhouse Virtualization Platform supports several cluster architecture options — from single-node installations to distributed configurations. The choice of a specific option depends on the requirements: the need for quick test deployment, high availability, or isolation of system services from user workloads.

**Single-node cluster (Single Node / Edge)**: All management components, auxiliary services, and virtual machines are placed on a single server. Suitable for test environments and edge scenarios. Requires minimal resources but does not provide fault tolerance.

**Cluster with one master node and worker nodes**: One node performs management functions, virtual machines are placed on dedicated worker nodes. Suitable for small clusters where separation of system services and user workloads is required. Fault tolerance is absent.

**Three-node cluster (High Availability)**: Management components are distributed across three master nodes, which ensures fault tolerance of the control plane and continued operation if one of the nodes fails. User workloads can run on these same servers or on dedicated worker nodes. Recommended for production environments.

**Highly available distributed cluster**: Management components are deployed on three dedicated master nodes; if necessary, system services, monitoring, and ingress are moved to separate system or frontend nodes. User virtual machines run exclusively on worker hypervisors. Provides high availability, scalability, and isolation of user workloads from system services. Used in large clusters.

See also the [Architecture options](/products/virtualization-platform/documentation/about/architecture-options.html) section.

## Resource requirements

Before deploying a cluster, you need to plan the resources that may be required for its operation. To do this, answer a few questions:

- What workload is planned for the cluster?
- How many virtual machines are planned to be launched?
- Does the cluster require high availability mode?
- What type of storage will be used (SDS or external)?

Answers to these questions will help determine the required cluster architecture and node resources.

{% alert level="info" %}
When using software-defined storage (SDS) on nodes that participate in storage organization, you need to provide additional disks beyond the specified minimum requirements. These disks will be used by SDS to store virtual machine data.
{% endalert %}

Depending on the architecture, the following minimum resources are required for the platform to operate correctly:

| Architecture                                                             | Workload placement   | Master node          | Worker node         | System node          | Frontend node       |
|--------------------------------------------------------------------------|----------------------|----------------------|---------------------|----------------------|---------------------|
| Single-node platform<br/>(Single Node / Edge)                            | On a single node     | 3 vCPU<br/>10 GB RAM | —                   | —                    | —                   |
| Multi-node platform<br/>(1 master node + worker nodes)                   | On all nodes         | 6 vCPU<br/>6 GB RAM  | 2 vCPU<br/>4 GB RAM | —                    | —                   |
| Three-master platform<br/>(3 master nodes, High Availability)            | On all nodes         | 6 vCPU<br/>14 GB RAM | —                   | —                    | —                   |
| Platform with dedicated worker nodes<br/>(3 master nodes + worker nodes) | On worker nodes only | 5 vCPU<br/>11 GB RAM | 2 vCPU<br/>5 GB RAM | —                    | —                   |
| Distributed architecture                                                 | On worker nodes only | 4 vCPU<br/>9 GB RAM  | 1 vCPU<br/>2 GB RAM | 4 vCPU<br/>10 GB RAM | 1 vCPU<br/>2 GB RAM |

The number of virtual machines that can be run on nodes is limited by the `maxPods` parameter in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource properties. When planning the number of VMs, keep in mind that the `maxPods` limit includes all pods on the node: system pods, containerized workloads, and virtual machines. Each virtual machine occupies one pod in the cluster.

{% alert level="info" %}
Minimum requirements are specified for the basic platform configuration. When increasing the load, the number of virtual machines, or enabling additional modules, it may be necessary to increase node resources.
{% endalert %}

## Configuration features

### Cluster subnet planning

{% alert level="warning" %}
All cluster subnets must not overlap with each other.
{% endalert %}

Define IP address subnets for the cluster:

- **Node subnet**: Used by nodes to communicate with each other. This is the only subnet that physically exists in the organization's network and is routed in your infrastructure. Must be a real network in your datacenter.
- **Pod subnet** (`podSubnetCIDR`): Virtual network inside the cluster, used to assign IP addresses to Kubernetes pods (including system pods and containerized workloads).
- **Service subnet** (`serviceSubnetCIDR`): Virtual network inside the cluster, used to assign IP addresses to Kubernetes ClusterIP services for intra-cluster communication.
- **Virtual machine address subnets** (`virtualMachineCIDRs`): Virtual networks inside the cluster, used to assign IP addresses to virtual machines. Multiple subnets can be specified.

See also the [VM Network](/products/virtualization-platform/documentation/admin/platform-management/network/vm-network.html) section.

### Master nodes

{% alert level="info" %}
A cluster must include three master nodes with fast 400+ IOPS disks.
{% endalert %}

Always use three master nodes — this number ensures fault tolerance and allows safe updates of master nodes. There is no need for more master nodes, and two nodes will not provide quorum.

{% alert level="info" %}
If you need to run workloads (virtual machines) on control plane nodes, which is typical for **Single-node cluster (Single Node / Edge)** and **Three-node cluster (High Availability)** configurations, you need to configure tolerations in the virtual machine configuration or in the virtual machine class to allow VMs to be placed on master nodes.
{% endalert %}

See also the [Working with static nodes](/products/kubernetes-platform/documentation/v1/architecture/node.html#working-with-static-nodes) section.

### Frontend nodes

{% alert level="info" %}
Use two or more frontend nodes.

Use the `HostPort` inlet with an external load balancer.
{% endalert %}

Frontend nodes balance incoming traffic. Ingress controllers run on them. The NodeGroup of frontend nodes has the `node-role.deckhouse.io/frontend` label. For details on allocating nodes for specific load types, refer to [Advanced scheduling](/products/kubernetes-platform/documentation/v1/#advanced-scheduling).

Use more than one frontend node. Frontend nodes must be able to continue handling traffic even if one of them fails.

For example, if the cluster has two frontend nodes, each frontend node must handle the entire cluster load in case the second node fails. If the cluster has three frontend nodes, each frontend node must handle at least one and a half times the load increase.

The platform supports three ways of receiving traffic from the external world:

- `HostPort`: Installs an Ingress controller accessible on node ports via `hostPort`.
- `HostPortWithProxyProtocol`: Installs an Ingress controller accessible on node ports via `hostPort` and uses proxy-protocol to obtain the client's real IP address.
- `HostWithFailover`: Installs two Ingress controllers (primary and standby).

{% alert level="warning" %}
The `HostWithFailover` inlet is suitable for clusters with a single frontend node. It reduces downtime for the Ingress controller during updates. This inlet type is suitable for important development environments, however, it is **not recommended for production**.
{% endalert %}

### Monitoring nodes

{% alert level="info" %}
For high-load clusters, use two monitoring nodes equipped with fast disks.
{% endalert %}

Monitoring nodes are used to run Grafana, Prometheus, and other monitoring components. The [NodeGroup](/modules/node-manager/cr.html#nodegroup) for monitoring nodes has the `node-role.deckhouse.io/monitoring` label.

In high-load clusters with many alerts and large volumes of metrics, we recommend allocating dedicated nodes for monitoring. If this is not done, monitoring components will be placed on [system nodes](#system-nodes).

When allocating monitoring nodes, it's important to equip them with fast disks. You can do so by providing a dedicated `storageClass` on fast disks for all platform components (the [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-storageclass) global parameter) or allocating a dedicated `storageClass` to monitoring components only (the [storageClass](/modules/prometheus/configuration.html#parameters-storageclass) and [longtermStorageClass](/modules/prometheus/configuration.html#parameters-longtermstorageclass) parameters of the `prometheus` module).

If the cluster is initially created with nodes allocated for a specific type of workload (system nodes, monitoring nodes, etc.), we recommend that you explicitly specify the corresponding nodeSelector in the configuration of modules using persistent storage volumes. For example, for the `prometheus` module, this parameter is [nodeSelector](/modules/prometheus/configuration.html#parameters-nodeselector).

### System nodes

{% alert level="info" %}
Use two system nodes.
{% endalert %}

System nodes are used to run platform modules. The [NodeGroup](/modules/node-manager/cr.html#nodegroup) for system nodes has the `node-role.deckhouse.io/system` label.

Allocating two system nodes ensures that platform modules run without interfering with user applications in the cluster. For details on allocating nodes for specific load types, refer to [Advanced scheduling](/products/kubernetes-platform/documentation/v1/#advanced-scheduling).

Fast disks are recommended for platform components (the [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-storageclass) global parameter).

### Storage configuration

Configure one or more storage systems for virtual machine disks:

- **Software-defined storage (SDS)**:
  - `sds-local-volume`: Local storage based on LVM. High performance but without replication. Suitable for temporary data or when external backup is available.
  - `sds-replicated-volume`: Replicated storage based on DRBD. Provides fault tolerance through replication between nodes. Recommended for production.

- **External storage**: Ceph, NFS, TATLIN.UNIFIED (Yadro), Huawei Dorado, HPE 3par, iSCSI. Connected via corresponding CSI drivers.

{% alert level="info" %}
When using SDS on nodes participating in storage organization, you need to provide additional disks beyond the minimum requirements for node resources. The size of additional disks depends on the planned volume of virtual machine data.
{% endalert %}

See also the [Storage configuration](/products/virtualization-platform/documentation/admin/platform-management/storage/) section.

### Virtual machine class configuration

Create your own virtual machine class (one or more).

{% alert level="info" %}
The default `generic` class is not recommended for production.
{% endalert %}

Configure sizing policies to control VM resources:
- Use `type: Host` for identical nodes (same processor architecture).
- Use `type: Discovery` for different processor types.

Sizing policies limit the allowed VM resource configurations (number of cores, memory, core usage fraction) and prevent creating VMs with suboptimal configurations. The `coreFractions` parameter controls CPU overcommit: by setting the minimum core usage fraction, you guarantee each VM a corresponding portion of CPU and thereby limit the maximum allowed resource oversubscription level.

See also the [VirtualMachineClass settings](/products/virtualization-platform/documentation/admin/platform-management/virtualization/virtual-machine-classes.html#virtualmachineclass-settings) section.

## Access control

{% alert level="info" %}
Configure user authentication and access control. For production, it is recommended to use projects with role model configuration.
{% endalert %}

The platform supports managing internal users and groups, as well as integration with external authentication providers:

- **Internal users and groups**: Created through [User](/modules/user-authn/cr.html#user) and [Group](/modules/user-authn/cr.html#group) resources of the `user-authn` module.
- **External authentication providers**: LDAP, OIDC, GitHub, GitLab, Crowd, Bitbucket Cloud. Multiple providers can be connected simultaneously.

See also the [Integration with external authentication systems](/products/virtualization-platform/documentation/admin/platform-management/access-control/user-management.html) section.

Configure projects and access rights in accordance with the planned use of the cluster. Projects (the [Project](/modules/multitenancy-manager/cr.html#project) resource) provide isolated environments for creating user resources. Project settings allow you to set resource quotas, limit network interaction, and configure a security profile.

Access control is configured through a role model using the standard Kubernetes RBAC approach. You can use existing roles or create your own:

- **Manage roles**: For platform administrators. Allow configuring the cluster, managing virtual machines at the platform level, and creating projects (tenants) for users.
- **Use roles**: For project users. Allow managing resources (including virtual machines) within assigned projects.

See also the [Projects](/products/virtualization-platform/documentation/admin/platform-management/access-control/projects.html) and [Role model](/products/virtualization-platform/documentation/admin/platform-management/access-control/role-model.html) sections.


## Monitoring event notifications

{% alert level="info" %}
Configure alert delivery via the [internal](/modules/prometheus/faq.html#how-do-i-add-alertmanager) Alertmanager or connect an [external](/modules/prometheus/faq.html#how-do-i-add-an-additional-alertmanager) one.
{% endalert %}

Monitoring will work immediately after platform installation, but this is not enough for production. To receive notifications about incidents, configure the [built-in](/modules/prometheus/faq.html#how-do-i-add-alertmanager) Alertmanager in the platform or [connect your own](/modules/prometheus/faq.html#how-do-i-add-an-additional-alertmanager) Alertmanager.

Using the [CustomAlertmanager](/modules/prometheus/cr.html#customalertmanager) custom resource, you can configure sending notifications to [e-mail](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-emailconfigs), [Slack](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-slackconfigs), [Telegram](/modules/prometheus/usage.html#sending-alerts-to-telegram), via [webhooks](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-webhookconfigs), or by other means.

The list of all available alerts in the monitoring system is provided on a [separate documentation page](/products/kubernetes-platform/documentation/v1/reference/alerts.html).

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

{% alert level="warning" %}
Be sure to set up [etcd data backups](/products/virtualization-platform/documentation/admin/backup-and-restore.html#backing-up-etcd) — this is your last chance to restore the cluster in case of unforeseen events. Keep backups as *far away* from the cluster as possible.
{% endalert %}

The backups won't help if they don't work or if you don't know how to use them for recovery. We recommend compiling a [Disaster Recovery Plan](https://www.google.com/search?q=Disaster+Recovery+Plan) with specific steps and commands to restore the cluster from a backup.

This plan should be periodically updated and tested in drills.

## Community

{% alert level="info" %}
Follow the project news on [Telegram](https://t.me/deckhouse).
{% endalert %}

Join the [community](/community/) to stay up to date with important changes and news. You will be able to communicate with people engaged in the same work and avoid many typical problems.

The platform team knows how much effort it takes to organize a production cluster in Kubernetes. We will be glad if the platform allows you to realize your plans. Share your experience and inspire others to switch to Kubernetes.
