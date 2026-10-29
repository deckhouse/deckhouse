---
title: Going to production
permalink: en/guides/production.html
lang: en
description: Recommendations for preparing Deckhouse Platform cluster for production environment.
---

The following recommendations may be of less importance for a test or development cluster, but they may be critical for a production one.

## Release channel and update mode

{% alert %}
Use the `EarlyAccess` or `Stable` [release channel](../glossary.html#release-channel). Configure [auto-update window](/modules/deckhouse/usage.html#update-windows-configuration) or select [manual mode](/modules/deckhouse/usage.html#manual-update-confirmation).
{% endalert %}

Select the [release channel](../reference/release-channels.html) and [update mode](/modules/deckhouse/configuration.html#parameters-update-mode) that suit your needs. The more stable the release channel is, the later you will have the chance to use the new features.

If possible, use different release channels for clusters. Use a less stable update channel for a development cluster than for a testing cluster or stage (pre-production) cluster.

Use the `EarlyAccess` or `Stable` release channel for production clusters.

If you have more than one cluster in a production environment, make sure that the clusters update at different times. In this case, a new version is installed in one cluster first, and you have time to check it before the other clusters update. To spread the updates over time, set non-overlapping [update windows](/modules/deckhouse/configuration.html#parameters-update-windows) for the clusters or connect them to different release channels. For example, if one cluster gets updates from the `EarlyAccess` channel and another one from the `Stable` channel, a new version reaches the second cluster later.

{% alert level="warning" %}
Even in very busy and critical clusters, it is not a good idea to disable the use of the release channel. The best strategy is a scheduled update. If you are using a Deckhouse Platform release in your cluster that has not received an update in over six months, you will have a hard time getting help quickly should a problem arise.
{% endalert %}

The [update windows](/modules/deckhouse/configuration.html#parameters-update-windows) management allows you to schedule automatic platform [release](../glossary.html#dp-release) updates when your cluster is not experiencing peak load.

## Kubernetes version

{% alert %}
Use the automatic [Kubernetes version selection](../reference/api/cr.html#clusterconfiguration-kubernetesversion) or set the version explicitly.
{% endalert %}

In most cases, opt for the automatic selection of the Kubernetes version. In the platform, this behavior is set by default, but it can be changed with the [kubernetesVersion](../reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter. Upgrading the Kubernetes version in the cluster has no effect on applications and is done in a [consistent and secure fashion](/modules/control-plane-manager/#version-control).

If the automatic Kubernetes version selection is enabled, the platform can upgrade the Kubernetes version in the cluster together with the platform update (when upgrading a [minor version](../glossary.html#minor-version)). If the Kubernetes version in the [kubernetesVersion](../reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter is set explicitly, the platform may not upgrade to a newer version at some point if the Kubernetes version used in the cluster is no longer supported.

You must decide for yourself whether to use automatic version selection or set a specific version and update it manually every now and then.

If your application uses outdated versions of resources or depends on a particular version of Kubernetes for some other reason, check whether it is [supported](../reference/supported_versions.html) and [set it explicitly](../admin/configuration/platform-scaling/control-plane/updating-and-versioning.html).

## Resource requirements

{% alert %}
Use at least 4 CPUs / 8 GB RAM for [infrastructure nodes](../glossary.html#infrastructure-node) (2 CPUs / 4 GB RAM for [frontend nodes](../glossary.html#frontend-node)). For master and [monitoring nodes](../glossary.html#monitoring-node), fast disks are recommended.
{% endalert %}

The following resource minimums are recommended for infrastructure nodes, depending on their role in the cluster:

- **[Master node](../glossary.html#master-node)** — 8 CPU, 16GB RAM, 60 GB of disk space for the cluster and etcd data on a fast disk (400+ IOPS);
- **Frontend node** — 2 CPU, 4GB RAM, 50 GB of disk space;
- **Monitoring node** (for high-load clusters) — 6 CPU, 12GB RAM, <a href="#storage">50 / 150*</a> GB of disk space on a fast disk (400+ IOPS).
- **[System node](../glossary.html#system-node)**:
  - 4 CPU, 8 GB RAM, <a href="#storage">50 / 150*</a> GB of disk space — if there are dedicated monitoring nodes in the cluster;
  - 8 CPU, 16 GB RAM, <a href="#storage">60 / 160*</a> GB of disk space on a fast disk (400+ IOPS) — if there are no dedicated monitoring nodes in the cluster.
- **[Worker node](../glossary.html#worker-node)** — the requirements are similar to those for the master node, but largely depend on the nature of the load running on the node (nodes).

Additional recommendations:<span id="storage"></span>

- If system PVCs (of the `prometheus`, `upmeter` modules and others) are stored on the node's local disk, additionally allocate at least 100 GB of free space. The second value of the disk space for monitoring and system nodes above (for example, 150 in "50 / 150") includes these 100 GB.
- For system services (kubelet) and system pods on each worker node, reserve at least 1 CPU and 2 GB of RAM.
- For all nodes, use fast disks with performance of at least 400 IOPS.

Estimates of the resources required for the clusters to run:

- **Regular cluster**: 3 master nodes, 2 frontend nodes, 2 system nodes. Such a configuration requires **at least 44 CPUs and 88 GB RAM** along with fast 400+ IOPS disks for the master nodes.
- **High-load cluster** (with dedicated monitoring nodes): 3 master nodes, 2 frontend nodes, 2 system nodes, 2 monitoring nodes. Such a configuration requires **at least 48 CPUs and 96 GB RAM** along with fast 400+ IOPS disks for the master and monitoring nodes.
- Set up a dedicated [storageClass](../reference/api/global.html#parameters-modules-storageclass) on the fast disks for platform components.
- Add worker nodes to this, taking into account the nature of the workloads.

Also read [the instructions](hardware-requirements.html) on the hardware requirements for cluster resources, which describes in detail how to select the necessary resources depending on the expected load.

## Things to consider when configuring

### Master nodes

{% alert %}
Three master nodes with fast 400+ IOPS disks are highly recommended for a cluster.
{% endalert %}

Use three master nodes in all cases, as they are sufficient for fault tolerance. Also, with three nodes, you can safely update the cluster's control plane as well as the master nodes. Extra master nodes are not needed, while 2 nodes (or any even number) do not make a quorum.

The master node configuration for [cloud clusters](../glossary.html#cloud-cluster) can be configured using the [masterNodeGroup](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup) parameter.

Reference:

- [How do I add a master node to a cluster...](../admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#adding-master-nodes-in-a-cloud-cluster)
- [Working with static nodes...](/modules/node-manager/#working-with-static-nodes)

### Frontend nodes

{% alert %}
Use two or more frontend nodes.

Use [inlet](../glossary.html#inlet) `LoadBalancer` for OpenStack-based clouds and cloud services where automatic balancer ordering is not supported (AWS, GCP, Azure, etc.). Use inlet  `HostPort` with an external load balancer for bare metal or vSphere.
{% endalert %}

Frontend nodes are used for balancing incoming traffic. Such nodes are allocated for [Ingress controllers](../glossary.html#ingress-controller). The [NodeGroup](/modules/node-manager/cr.html#nodegroup) of the frontend nodes has a `node-role.deckhouse.io/frontend` label. Read more about [allocating nodes for specific load types...](../admin/configuration/#advanced-scheduling)

Use more than one frontend node. Frontend nodes must be able to still handle traffic even if one of the frontend nodes fails.

For example, if the cluster has two frontend nodes, each frontend node must be able to handle the entire cluster load in case the second frontend node fails. If the cluster has three frontend nodes, each frontend node must be able to handle a load that is at least one and a half times higher.

Select the [inlet type](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-inlet) (it defines the way the traffic comes in).

When deploying the platform in a cloud infrastructure where provisioning of load balancers is supported (e.g., OpenStack-based clouds, AWS, GCP, Azure, etc.), use the `LoadBalancer` or `LoadBalancerWithProxyProtocol` inlet.

In environments where automatic load balancer provisioning is not supported (bare metal clusters, vSphere, custom OpenStack solutions), use the `HostPort` or `HostPortWithProxyProtocol` inlet. In this case, you can either add some A&#8209;records to DNS for the corresponding domain or use an external load-balancing service (e.g., Cloudflare, Qrator solutions, or configure metallb).

{% alert level="warning" %}
The `HostWithFailover` inlet is suitable for clusters with a single [frontend node](../glossary.html#frontend-node). It reduces the time that the Ingress controller is unavailable during updates. This type of inlet is suitable for important development environments, but **not recommended for production**.
{% endalert %}

The algorithm for choosing an inlet:

![The algorithm for choosing an inlet](../images/guides/going_to_production/ingress-inlet.svg)

### Monitoring nodes

{% alert %}
For high-load clusters, use two [monitoring nodes](../glossary.html#monitoring-node) equipped with fast disks.
{% endalert %}

Monitoring nodes are used to run Grafana, Prometheus, and other monitoring components. The [NodeGroup](/modules/node-manager/cr.html#nodegroup) for monitoring nodes has the `node-role.deckhouse.io/monitoring` label attached.

In high-load clusters, where many alerts are generated and many metrics are collected, allocate dedicated nodes for monitoring. If not, monitoring components will be deployed to [system nodes](#system-nodes).

When allocating monitoring nodes, it is important to allocate fast disks to them. You can do so by providing a dedicated `storageClass` on fast disks for all platform components (global parameter [storageClass](../reference/api/global.html#parameters-modules-storageclass)) or allocate a dedicated `storageClass` to monitoring components only [storageClass](/modules/prometheus/configuration.html#parameters-storageclass) and [longtermStorageClass](/modules/prometheus/configuration.html#parameters-longtermstorageclass) parameters of the `prometheus` module.

If the cluster is initially created with nodes allocated for a specific type of workload ([system nodes](../glossary.html#system-node), nodes for monitoring, etc.), explicitly specify the corresponding `nodeSelector` parameter in the module configuration for modules that use persistent storage volumes (for example, for the `prometheus` module). For the `prometheus` module, this parameter is [nodeSelector](/modules/prometheus/configuration.html#parameters-nodeselector).

### System nodes

{% alert %}
Dedicate two system nodes.
{% endalert %}

System nodes are used to run platform modules. Their [NodeGroup](/modules/node-manager/cr.html#nodegroup) has the `node-role.deckhouse.io/system` label.

Set two nodes to be system nodes. This way, platform modules will run on them without interfering with user applications in the cluster. Read more about [allocating nodes to specific load types...](../admin/configuration/#advanced-scheduling).

Provide the platform components with fast disks (the [storageClass](../reference/api/global.html#parameters-modules-storageclass) global parameter).

### Cluster subnets

{% alert %}
Plan the cluster subnets before installation, because you cannot change them afterwards.
{% endalert %}

You set the pod subnet ([`podSubnetCIDR`](/modules/control-plane-manager/configuration.html#parameters-network-podsubnetcidr)), the service subnet ([`serviceSubnetCIDR`](/modules/control-plane-manager/configuration.html#parameters-network-servicesubnetcidr)), and the node subnet size ([`podSubnetNodeCIDRPrefix`](/modules/control-plane-manager/configuration.html#parameters-network-podsubnetnodecidrprefix)) when you install the cluster. Select the pod and service subnets so that they do not overlap with each other or with the node network. Changing these parameters in a running cluster can break it, so the platform blocks such changes. If you need other subnets, recreate the cluster.

The node subnet size defines how many pods fit on a node. By default, the [pod limit per node](../glossary.html#pod-limit-per-node), the [`kubelet.maxPods`](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-maxpods) parameter of the [node group](../glossary.html#node-group), is calculated from this size. The mapping between the subnet size and the pod limit is listed in the [Pod limit per node](high-pod-density.html#pod-limit-per-node) section.

### Storage

{% alert %}
Use [replicated storage](../glossary.html#replicated-storage) for production data and set a default StorageClass.
{% endalert %}

Workloads keep persistent data in PersistentVolume volumes, so connect at least one of the [supported storage types](../admin/configuration/storage/) to the cluster. For production, use storage with data replication between nodes, for example, [replicated storage based on DRBD](../admin/configuration/storage/sds/lvm-replicated.html), so the data stays available if a node fails. If you use [software-defined storage](../glossary.html#sds) (SDS), add dedicated disks for data to its nodes.

Set the default StorageClass in the [`defaultClusterStorageClass`](../reference/api/global.html#parameters-defaultclusterstorageclass) global parameter. Volumes that do not specify a storage class explicitly get this class.

### Virtualization

If virtual machines (VMs) will run in the cluster, follow the recommendations in this section together with the other recommendations of this guide.

#### Node requirements for VMs

A cluster with VMs must meet the following requirements:

- VMs run on physical servers (bare metal).
- CPUs of nodes for VMs support Intel-VT (VMX) or AMD-V (SVM) instructions, and hardware virtualization is enabled in BIOS or UEFI settings.
- All cluster nodes run the same Linux kernel version.

The same kernel version is necessary for stable VM [live migration](../glossary.html#live-migration), because system calls and resource handling differ between kernel versions.

{% alert level="warning" %}
Use virtual machines instead of physical servers as nodes only for demonstration purposes. In this case, enable nested virtualization.
{% endalert %}

#### Number of VMs per node

The number of VMs per node is limited by the [pod limit](#cluster-subnets), which VMs, containers, and [system components](../glossary.html#system-component) share. A running VM with disks usually occupies two pods, because besides the VM pod, a service pod runs on the node and attaches the [VM disks](../glossary.html#vm-disk) and images. It serves disks added with the [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) resource and all disks of a VM with paravirtualization enabled (the [`.spec.enableParavirtualization`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-enableparavirtualization) parameter, enabled by default). During live migration, the same pods start on the target node, so take them into account when you select the node subnet size.

#### VM subnets

VMs get IP addresses in the main cluster network from the subnets listed in the [`virtualMachineCIDRs`](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) parameter of the `virtualization` module. Select these subnets so that they do not overlap with the node network or the [cluster subnets](#cluster-subnets), otherwise address conflicts occur.

Specify a subnet by its start address aligned to the mask, for example, `192.168.1.192/27`, not `192.168.1.190/27`. You can specify several subnets and add new ones later, but you cannot delete a subnet while [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resources hold addresses from it.

The `virtualMachineCIDRs` parameter is optional. Without it, VMs cannot work in the main cluster network or get addresses with the VirtualMachineIPAddress resource, so you can connect them only to additional networks. VM network configuration is described in the [Virtual machine networking](../admin/configuration/network/vm-network.html) section.

#### Live migration network

By default, live migration traffic goes through the main node network, competes with workloads for bandwidth, and is limited to 640 MiB/s per migration. To keep migrations from affecting workloads and to make them faster, send them to a dedicated network in the [`liveMigration.network`](/modules/virtualization/configuration.html#parameters-livemigration-network) parameter of the `virtualization` module, where the platform does not limit the speed by default. This requires the [`sdn`](/modules/sdn/) module to be enabled and the [SystemNetwork](/modules/sdn/cr.html#systemnetwork) resource to be in the `Ready` state. The configuration is described in the [Dedicated migration network](../admin/configuration/platform-scaling/node/vm-node-maintenance.html#dedicated-migration-network) section.

#### Running VMs on master nodes

You need to run VMs on [master nodes](../glossary.html#master-node) if the cluster has no separate nodes for workloads, for example, in a [single-node cluster](../glossary.html#single-node-cluster). To run VMs on master nodes, remove the taints from the `master` node group in the [`nodeTemplate.taints`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-taints) parameter or set tolerations for the selected VMs. Set tolerations in a VM with the [`.spec.tolerations`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-tolerations) parameter or in a VM class with the [`.spec.tolerations`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-tolerations) parameter. Removing the taints lets any workload run on master nodes, and tolerations let only the VMs that have them run there.

#### Storage for VM disks

VM disks and images are stored in PersistentVolume volumes, so select storage for them according to the recommendations in the [Storage](#storage) section. If VM disks will be stored in software-defined storage, take their size into account when you add disks to the nodes of this storage.

Live migration of VMs with local disks is available only in commercial Deckhouse Platform [editions](../glossary.html#edition), where the disks move together with the VM. In Deckhouse Platform Open, a VM with a local disk cannot move to another node, so you have to stop it during node maintenance.

#### Image storage

The platform stores [VM images](../glossary.html#vm-image) in the internal image storage ([DVCR](../glossary.html#dvcr)), which runs as a single instance on one persistent volume. The storage class and size of this volume are set in the [`dvcr.storage`](/modules/virtualization/configuration.html#parameters-dvcr-storage) parameter of the `virtualization` module. After the volume is created, you can only increase its size, and you cannot change its storage class, so select the class in advance. Use replicated storage for DVCR so that the images stay available if a node fails.

#### VM classes

Create your own VM classes (the [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) resource) and do not use the `generic` class for production workloads. The platform creates the `generic` class during installation with the Nehalem CPU model. This model supports the smallest instruction set, so VMs of the `generic` class start on any node but do not get the instructions of modern CPUs.

Mark your main class with the `virtualmachineclass.virtualization.deckhouse.io/is-default-class` annotation set to `true`, and the platform will set it for VMs that do not specify a class. Do not set this annotation on the `generic` class, because it can be lost when the module is updated.

The CPU type of a class is set by the [`.spec.cpu.type`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu-type) parameter and defines the nodes between which VMs can migrate. You cannot change it after you create the class, so select the type in advance:

- `Host` gives a VM an instruction set close to the node CPU. VMs migrate only between nodes with similar CPUs. Migration between Intel and AMD CPUs or between different CPU generations is not possible.
- `HostPassthrough` passes the node CPU to a VM without changes. VMs migrate only to nodes with exactly the same CPU.
- `Discovery` builds a common instruction set for the selected nodes and suits a cluster with different CPUs. The set is fixed when the class is created and does not change when nodes are added to the cluster.

The other CPU types are described in the [Virtual processor of virtual machines](../admin/configuration/virtualization/vm-classes-cpu.html) section.

The [`.spec.sizingPolicies`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-sizingpolicies) parameter sets the [sizing policies](../glossary.html#sizing-policy) of a class. The policies limit the number of cores, the memory size, and the [core fraction](../glossary.html#vm-core-fraction) for VMs of this class. The core fraction is the part of physical core CPU time that the platform guarantees to a VM. The allowed fraction values in the `coreFractions` parameter define how strong [CPU oversubscription](../glossary.html#cpu-oversubscription) can be. Oversubscription configuration is described in the [CPU oversubscription for virtual machines](../admin/configuration/virtualization/cpu-oversubscription.html) section.

## Access control

{% alert %}
Configure user authentication and grant permissions with the granular role-based model. Create projects for teams.
{% endalert %}

You can create users and groups in the cluster with the [User](/modules/user-authn/cr.html#user) and [Group](/modules/user-authn/cr.html#group) resources of the `user-authn` module or get them from external authentication providers. The platform supports providers that use OIDC, SAML, and LDAP, as well as GitHub, GitLab, Atlassian Crowd, and Bitbucket Cloud. You can connect several providers to the cluster at the same time. Provider configuration is described in the [Integration with external authentication providers](../admin/configuration/access/authentication/external-authentication-providers.html) section, and creating users in the cluster is described in the [Local authentication](../admin/configuration/access/authentication/local.html) section.

To isolate team workloads from each other, create projects for the teams (the [Project](/modules/multitenancy-manager/cr.html#project) resource of the `multitenancy-manager` module). A project is created from a template that sets resource limits, network isolation, and a security profile. Project management is described in the [Project management](../admin/multitenancy/project-management.html) section.

Grant permissions with the [granular role-based model](/modules/user-authz/#granular-role-based-model) of the `user-authz` module. The model uses standard Kubernetes RBAC resources and the roles that the module creates. Each role acts in one of four scopes:

- **Namespace.** The `d8:namespace:<LEVEL>` roles grant permissions to application users in one namespace and are assigned with a RoleBinding resource.
- **Project.** The `d8:project:<LEVEL>` roles grant permissions to teams in all namespaces of a project at once and are assigned with a [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) resource.
- **Subsystem.** The `d8:subsystem:<SUBSYSTEM>:<LEVEL>` roles grant permissions to administrators of a part of the platform, for example, the network, and are assigned with a ClusterRoleBinding resource.
- **Entire platform.** The `d8:system:<LEVEL>` roles grant permissions to platform administrators and are assigned with a ClusterRoleBinding resource.

Here, `<LEVEL>` is the access level, for example, `viewer` or `manager`, and `<SUBSYSTEM>` is the subsystem name.

The basic role-based model, built on the ClusterAuthorizationRule and AuthorizationRule resources, still works, but its support will be discontinued in future releases.

## Configuring alerts

{% alert %}
You can send alerts using the [internal](/modules/prometheus/faq.html#how-do-i-add-alertmanager) Alertmanager or connect the [external](/modules/prometheus/faq.html#how-do-i-add-an-additional-alertmanager) one.
{% endalert %}

Monitoring will work once the platform is installed, however, it is not enough for production clusters. Configure the Alertmanager [built in](/modules/prometheus/faq.html#how-do-i-add-alertmanager) the platform or [connect your](/modules/prometheus/faq.html#how-do-i-add-an-additional-alertmanager) own Alertmanager to receive incident notifications.

Using the [CustomAlertmanager](/modules/prometheus/cr.html#customalertmanager) custom resource, you can configure sending alerts to an [e-mail](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-emailconfigs), [Slack](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-slackconfigs), [Telegram](/modules/prometheus/usage.html#sending-alerts-to-telegram), via the [webhook](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-webhookconfigs), or by other means.

For the list of all available alerts in the Deckhouse Platform monitoring system, refer to the [corresponding documentation page](../reference/alerts.html).

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

The backups won't help if they don't work or if you don't know how to use them to recover the cluster. Compile a Disaster Recovery Plan (DRP) with specific steps and commands to restore the cluster from a backup. The restore procedures are described in the [Backup and restore](../admin/configuration/backup/backup-and-restore.html) section.

This plan should be periodically updated and tested in drills.

## Community

{% alert %}
Follow the project channel on [Telegram](https://t.me/deckhouse) for news and updates.
{% endalert %}

Join the [community](https://deckhouse.io/community/about.html) to keep up with important news and developments. This will help you to share experiences with people who are doing the same thing as you are and avoid typical problems.

Running production in Kubernetes takes effort. Share your experience with Deckhouse Platform in the community to help others switch to Kubernetes.
