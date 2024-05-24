---
title: "Managing nodes"
description: Deckhouse manages nodes of a Kubernetes cluster as a related group, configures and updates Kubernetes cluster node components, manages cluster scaling in the cloud, and manages Linux users on nodes.
---

## Primary functions

The `node-manager` module is responsible for managing nodes and has the following primary functions:
1. Managing multiple nodes as a related group (**NodeGroup**):
    * The ability to define metadata that are inherited by all nodes in the group.
    * Monitoring of a group as a single entity (grouping nodes on graphs by groups, grouping alerts about node unavailability, alerts about the unavailability of N or N% of nodes in a group).
2. **Chaos monkey** — the systemic termination of nodes. This feature tests the resilience of cluster elements and running applications.
3. Installing, updating, and configuring the node software (containerd, kubelet, etc.), connecting the node to the cluster:
    * Installing operating system (see the list of [supported OS](../../supported_versions.html#linux)) regardless of the infrastructure used (any cloud/any hardware).
    * The operating system's basic setup (disabling auto-update, installing the necessary packages, configuring logging parameters, etc.).
    * Configuring nginx (and the system for automatically updating the lsit of upstreams) to balance node (kubelet) requests over API servers.
    * Installing and configuring CRI containerd and Kubernetes, adding the node to the cluster.
    * Managing node updates and their downtime (disruptions):
        * Automatic determination of a valid minor Kubernetes version for a node group based on its settings (the kubernetesVersion parameter specified for a group), the default version for the whole cluster, and the current control-plane version (no nodes can be updated ahead of the control-plane update).
        * Only one node of a group can be updated at a time, and only if all the nodes in the group are available.
        * There are two types of node updates:
            * regular updates – always performed automatically;
            * disruption-involving updates (such as updating the kernel, switching docker versions, major change of the kubelet versions, etc.) – you can choose manual or automatic mode. If automatic disruptive updates are enabled, the node is drained before the update (this functionality can be disabled).
    * Monitoring the status and progress of the update.
4. Cluster scaling.
   * Autoscaling.

     Available with supported cloud providers ([learn more](#scaling-nodes-in-the-cloud)); not available for static nodes. Cloud providers support automatic creation or deletion of virtual machines, joining them to or disjoining them from the cluster.
     
   * Maintaining the desired number of nodes in a group.

     Available for both [cloud providers](#scaling-nodes-in-the-cloud) and static nodes (when using [Cluster API Provider Static](#working-with-static-nodes)).
5. Managing Linux users on nodes.

Nodes are managed through the [NodeGroup](cr.html#nodegroup) resource, and each node group performs specific tasks. Below are examples of node groups pooled by their tasks performed:
- a group of master nodes;
- a group of nodes for routing HTTP(s) traffic (front nodes);
- a group of monitoring-related nodes;
- a group of application nodes (the so-called worker nodes), etc.

The nodes belonging to the group have common parameters and are configured automatically according to the group's parameters. Deckhouse scales groups by adding, excluding, and updating their nodes. Both cloud and static (bare metal or virtual machine) nodes can be combined into a single group. It paves the way for the hybrid clusters, in which you can scale nodes on physical servers via cloud nodes.

Operations on the [cloud infrastructure](#working-with-nodes-on-supported-cloud-platforms) are performed by means provided by supported cloud providers. If there is no support for the desired cloud platform, you can use its resources as static nodes.

The [static nodes](#working-with-static-nodes) (e.g., bare metal servers) are handled by the Cluster API Provider Static (CAPS).

The following Managed Kubernetes services are supported (note that some service functionality may be unavailable):
- Google Kubernetes Engine (GKE);
- Elastic Kubernetes Service (EKS).

## Node types

The following node types that can be worked with within a node group (resource [NodeGroup](cr.html#nodegroup)) are supported:
- `CloudEphemeral` — such nodes are automatically ordered, created, and deleted in the configured cloud provider.
- `CloudPermanent` — they differ in that their configuration is not taken from the custom resource [nodeGroup](cr.html#nodegroup), but from a special resource `<PROVIDER>ClusterConfiguration` (for example, [AWSClusterConfiguration](../030-cloud-provider-aws/cluster_configuration.html) for AWS). Also, an important difference is that to apply node configuration, you need to run `dhctl converge` (by running Deckhouse installer). An example of a CloudPermanent node of a cloud cluster is a cluster master node.
- `CloudStatic` — a static node (created manually) hosted in the cloud integrated with one of the cloud providers. This node has the CSI running, and it is managed by the cloud-controller-manager. The `Node` object automatically gets the information about the cloud zone and region. Also, if a node gets deleted from the cloud, its corresponding Node object will be deleted in a cluster.
- `Static` — a static node hosted on a bare metal or virtual machine. In the case of a cloud environment, the `cloud-controller-manager` does not manage the node even if one of the cloud providers is enabled. [Learn more about working with static nodes...](#working-with-static-nodes)

## Node grouping and group management

Grouping and managing nodes as a related group mean that all nodes in the group will have the same metadata derived from the [`NodeGroup`](cr.html#nodegroup) custom resource.

The following monitoring patterns are available for node groups:
- with grouping of node parameters on the graphs for the group;
- with grouping of alerts about node unavailability;
- with alerts about unavailability of N nodes or N% of nodes in the group, etc.

## Automatic deploying, configuring and updating Kubernetes nodes

Automatic deployment (partially supported for *static/hybrid* nodes/clusters), configuration, and continuing software updates are supported for all cluster types (cloud or bare metal based).

### Deploying Kubernetes nodes

Deckhouse automatically deploys cluster nodes by performing the following **idempotent** operations:
- Configuring the OS and optimizing it for containerd and Kubernetes:
  - Installing the needed packages from the distribution's repository.
  - Configuring kernel parameters, logging, log rotation, and other system parameters.
- Installing the appropriate versions of containerd and kubelet; adding the node to the Kubernetes cluster.
- Configuring Nginx and updating the list of upstream resources for balancing node requests to the Kubernetes API.

### Keeping nodes up-to-date

Two types of updates can be applied to keep cluster nodes up to date:
- **Regular**. These updates are always applied automatically and do not cause the node to stop or reboot.
- **Disruptive**. An example of such updates is a kernel or containerd version update, a major kubelet version change, etc. For this type of updates you can choose manual or automatic mode ([disruptions](cr.html#nodegroup-v1-spec-disruptions) parameter section). In automatic mode, the node is gracefully drained before the update.

Only one node of the group is updated at a time, and only if all the nodes in the group are available.

The `node-manager` module has a set of built-in metrics for monitoring the update process, alerting about issues with the update, or when a decision to proceed needs to be made by an administrator.

## Working with nodes on supported cloud platforms

Each supported cloud provider can provision nodes in an automated manner. For this, you have to specify the required parameters for each node or a group of nodes.

Depending on the provider, these parameters can include:
- type of a node, the number of processor cores, the amount of RAM;
- disk size;
- security settings;
- connected networks;
- other parameters.

Creating, starting, and connecting virtual machines to the cluster are performed automatically.

### Scaling nodes in the cloud

Two methods of scaling nodes in a group are supported:
- **Automatic scaling**

  If there is a resource shortage or pods in the `Pending` state are present, nodes will be added to the group. If there is no load on one or more nodes, they will be removed from the cluster. Autoscaling takes into account the group [priority](cr.html#nodegroup-v1-spec-cloudinstances-priority) (the group with the highest priority will be scaled first).
  
  To enable automatic node scaling, specify *different* non-zero values for the [minimum](cr.html#nodegroup-v1-spec-cloudinstances-minperzone) and [maximum](cr.html#nodegroup-v1-spec-cloudinstances-maxperzone) number of nodes in the group.

- **Fixed number of nodes**

  In this case, Deckhouse will maintain the specified number of nodes (e.g., by provisioning new nodes when nodes fail).

  To set a fixed number of nodes in a group and disable automatic scaling, specify the *same* values for [minPerZone](cr.html#nodegroup-v1-spec-cloudinstances-minperzone) and [maxPerZone](cr.html#nodegroup-v1-spec-cloudinstances-maxperzone).

## Working with static nodes

When working with static nodes, some features of the `node-manager` module are limited:
- **No node provisioning.** Resources (bare metal servers, virtual machines, linked resources) are provisioned manually. Subsequent configuration of resources (connecting a node to the cluster, setting up monitoring, etc.) is performed either fully (similar to nodes in the cloud) or partly automatically.
- **No node autoscaling.** Maintaining the specified number of nodes in a group is supported when using [Cluster API Provider Static](#cluster-api-provider-static) (parameter [staticInstances.count](cr.html#nodegroup-v1-spec-staticinstances-count)). That is, Deckhouse will attempt to maintain the specified number of nodes in the group, cleaning up unnecessary nodes and setting up new ones as needed ( by picking them from [StaticInstance](cr.html#staticinstance) resources that are in *Pending* state).

Configuring/clearing up a node, joining it to a cluster, and disjoining it can be done in the following ways:
- **Manually** using the pre-made scripts.

  To configure the server (VM) and join a node to the cluster, a special bootstrap script must be downloaded and run. This script is generated for each group of static nodes (each `NodeGroup` resource). It is located in the `d8-cloud-instance-manager/manual-bootstrap-for-<NAME-NODEGROUP>` secret. See [an example](examples.html#manually) of how to add a static node to a cluster.
  
  To disjoin a cluster node and clean up the server (virtual machine), you need to run the `/var/lib/bashible/cleanup_static_node.sh` script (it is present on each static node). An example of decommissioning a cluster node and cleaning up the server can be found in [FAQ](faq.html#how-do-i-clean-up-a-static-node-manually).

- **Automatically** using [Cluster API Provider Static](#cluster-api-provider-static).

  > This feature is available as of Deckhouse version 1.54 and is currently under active development and testing.

  Cluster API Provider Static (CAPS) connects to the server (VM) using [StaticInstance](cr.html#staticinstance) and [SSHCredentials](cr.html#sshcredentials) resources, configures, and joins the node into the cluster.

  If necessary (for example, if the [StaticInstance](cr.html#staticinstance) resource associated with the server is deleted or the [number of group nodes](cr.html#nodegroup-v1-spec-staticinstances-count) is reduced), the Cluster API Provider Static connects to the cluster node, clears it, and disconnects it from the cluster.

### Cluster API Provider Static

> Cluster API Provider Static is available starting from Deckhouse version 1.54. The features described are under testing and active development. Functionality and resource specifications are subject to change. Keep this in mind when using it in production clusters.

Cluster API Provider Static (CAPS) is an implementation of a declarative management provider for static nodes (bare metal servers or virtual machines) for the Kubernetes [Cluster API](https://cluster-api.sigs.k8s.io/). Essentially, CAPS is an additional layer of abstraction to the existing Deckhouse functionality that provides automatic static node configuration and cleanup using scripts generated for each node group (see [Working with Static Nodes](#working-with-static-nodes)).

CAPS features are as follows:
- configuring a bare metal server (or virtual machine) to join a Kubernetes cluster;
- joining a node to the Kubernetes cluster;
- disjoining the node from the Kubernetes cluster;
- cleaning the bare metal server (or virtual machine) after disjoining the node from the Kubernetes cluster.

CAPS uses the following CustomResource when operating:
- **[StaticInstance](cr.html#staticinstance).** Each `StaticInstance` resource details a specific host (server, VM) that is managed using CAPS.
- **[SSHCredentials](cr.html#sshcredentials)**. Contains the SSH credentials required to connect to the host (`SSHCredentials` data is specified in the [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the `StaticInstance` resource).
- **[NodeGroup](cr.html#nodegroup)**. The [staticInstances](cr.html#nodegroup-v1-spec-staticinstances) parameter section defines the required number of nodes in the group and the filter for the `StaticInstance` resources that can be used in the group.

CAPS is enabled automatically if the NodeGroup has the [staticInstances](cr.html#nodegroup-v1-spec-staticinstances) parameter section filled in. If the `staticInstances` parameter section in the `NodeGroup` is empty, then setting up and cleaning up nodes for that group is done *manually* rather than using CAPS (see [adding a static node to a cluster](examples.html#manually) and [cleaning up a node](faq.html#how-do-i-clean-up-a-static-node-manually)).

The workflow for dealing with static nodes when using Cluster API Provider Static (CAPS) (see the [adding a node](examples.html#using-the-cluster-api-provider-static) example) is as follows:
1. **Preparing resources.**

   Before bringing a bare metal server or virtual machine under CAPS control, some preliminary steps may be necessary:
   - Preparing the storage system, adding mount points, and so on;
   - Installing the OS-specific packages. For example, installing the `ceph-common` package if the server uses CEPH volumes;
   - Configuring the network. For example, configuring the network between the server and the cluster nodes;
   - Configuring SSH access to the server, creating a user that is a member of sudoers group. A good practice is to create a separate user and unique keys for each server.
   
1. **Creating a [SSHCredentials](cr.html#sshcredentials) resource.**

   The `SSHCredentials` resource contains the parameters required by CAPS to connect to a server via SSH. A single `SSHCredentials` resource can be used to connect to multiple servers, but it is a good practice to create unique users and access keys to connect to each server. In this case, the `SSHCredentials` resource will also be unique for each server.

1. **Creating a [StaticInstance](cr.html#staticinstance) resource.**

   For every server (VM) in the cluster, an individual `StaticInstance` resource is created. It contains the IP address for connecting and a link to the `SSHCredentials` resource with the data to be used for connecting.

   The following is a list of possible `StaticInstance` states and its associated servers (VMs) and cluster nodes:
   - `Pending`. The server is not configured and there is no associated node in the cluster.
   - `Bootstraping`. The procedure for configuring the server (VM) and connecting the node to the cluster is in progress.
   - `Running`. The server is configured and the associated node is added to the cluster.
   - `Cleaning`. The procedure of cleaning up the server and disconnecting the node from the cluster is in progress.

1. **Creating a [NodeGroup](cr.html#nodegroup) resource.**

   When using CAPS, you have to focus on the [nodeType](cr.html#nodegroup-v1-spec-nodetype) parameter (must be `Static`) of the `NodeGroup` resource as well as the [staticInstances](cr.html#nodegroup-v1-spec-staticinstances) parameter section.

   The [staticInstances.labelSelector](cr.html#nodegroup-v1-spec-staticinstances-labelselector) parameter section defines a filter that CAPS applies to select the `StaticInstance` resources to be used for a group. The filter allows only certain `StaticInstance` to be used for specific node groups, and also allows a single `StaticInstance` to be used in different node groups. You can choose not to define a filter to use any available `StaticInstance` for a node group.   

   The [staticInstances.count](cr.html#nodegroup-v1-spec-staticinstances-count) parameter specifies the desired number of nodes in the group. When the parameter changes, CAPS starts adding or removing the desired number of nodes (this process runs in parallel).

Using the data in the [staticInstances](cr.html#nodegroup-v1-spec-staticinstances) parameter section, CAPS attempts to maintain the specified number of nodes in the group ([count](cr.html#nodegroup-v1-spec-staticinstances-count) parameter). If a node needs to be added to the group, CAPS selects the resource [StaticInstance](cr.html#nodegroup-v1-spec-staticinstances-labelselector) that matches the [filter](cr.html#staticinstance) and is in the `Pending` state, configures the server (VM), and joins the node to the cluster. If a node needs to be removed from the group, CAPS selects the [StaticInstance](cr.html#staticinstance) that is in the `Running` state, cleans up the server (VM) and disconnects the node from the cluster (the corresponding `StaticInstance` then goes to the `Pending` state and can be reused).

## Custom node settings

The [NodeGroupConfiguration](cr.html#nodegroupconfiguration) resource allows you to automate actions on group nodes. It supports running bash scripts on nodes (you can use the [bashbooster](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bashbooster) command set) as well as the [Go Template](https://pkg.go.dev/text/template) templating engine, and is a great way to automate operations such as:
- installing and configuring additional OS packages ([example of installing the plugin for kubectl](examples.html#installing-the-cert-manager-plugin-for-kubectl-on-master-nodes), [example of installing containerd with Nvidia GPU support](faq.html#how-to-use-containerd-with-nvidia-gpu-support))
- updating the OS kernel to a specific version ([example](faq.html#how-do-i-update-kernel-on-nodes));
- modifying OS parameters ([example of customizing the sysctl parameter](examples.html#tuning-sysctl-parameters));
- collecting information on a node and carrying out other similar tasks.

The `NodeGroupConfiguration` resource allows you to assign [priority](cr.html#nodegroupconfiguration-v1alpha1-spec-weight) to scripts being run or limit them to running on specific [node groups](cr.html#nodegroupconfiguration-v1alpha1-spec-nodegroups) and [OS types](cr.html#nodegroupconfiguration-v1alpha1-spec-bundles).

The script code is stored in the [content](cr.html#nodegroupconfiguration-v1alpha1-spec-content) of the resource. When a script is created on a node, the contents of the `content` parameter are fed into the [Go Template](https://pkg.go.dev/text/template) templating engine. The latter embeds an extra layer of logic when generating a script. When parsed by the templating engine, a context with a set of dynamic variables becomes available.

The following variables are supported by the templating engine: 
<ul>
<li><code>.cloudProvider</code> (for node groups of nodeType <code>CloudEphemeral</code> or <code>CloudPermanent</code>) — cloud provider dataset.
{% offtopic title="Example of data..." %}
```yaml
cloudProvider:
  instanceClassKind: OpenStackInstanceClass
  machineClassKind: OpenStackMachineClass
  openstack:
    connection:
      authURL: https://cloud.provider.com/v3/
      domainName: Default
      password: p@ssw0rd
      region: region2
      tenantName: mytenantname
      username: mytenantusername
    externalNetworkNames:
    - public
    instances:
      imageName: ubuntu-22-04-cloud-amd64
      mainNetwork: kube
      securityGroups:
      - kube
      sshKeyPairName: kube
    internalNetworkNames:
    - kube
    podNetworkMode: DirectRoutingWithPortSecurityEnabled
  region: region2
  type: openstack
  zones:
  - nova
```
{% endofftopic %}</li>
<li><code>.cri</code> — the CRI in use (starting with Deckhouse 1.49, only <code>Containerd</code> is supported).</li>
<li><code>.kubernetesVersion</code> — the Kubernetes version in use.</li>
<li><code>.nodeUsers</code> — the dataset with information about node users added via the <a href="cr.html#nodeuser">NodeUser</a>.
{% offtopic title="Example of data..." %}
```yaml
nodeUsers:
- name: user1
  spec:
    isSudoer: true
    nodeGroups:
    - '*'
    passwordHash: PASSWORD_HASH
    sshPublicKey: SSH_PUBLIC_KEY
    uid: 1050
```
{% endofftopic %}
</li>
<li><code>.nodeGroup</code> — node group dataset.
{% offtopic title="Example of data..." %}
```yaml
nodeGroup:
  cri:
    type: Containerd
  disruptions:
    approvalMode: Automatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: "Off"
  kubernetesVersion: "1.27"
  manualRolloutID: ""
  name: master
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudPermanent
  updateEpoch: "1699879470"
```
{% endofftopic %}</li>
</ul>    

{% raw %}
An example of using variables in a template:
```shell
{{- range .nodeUsers }}
echo 'Tuning environment for user {{ .name }}'
# Some code for tuning user environment
{{- end }}
```

An example of using bashbooster commands:
```shell
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
```

{% endraw %}
The script progress can be seen on the node in the bashible service log (`journalctl -u bashible.service`). The scripts themselves are located in the `/var/lib/bashible/bundle_steps/` directory of the node.

## Chaos Monkey

The instrument (you can enable it for each `NodeGroup` individually) for unexpected and random termination of nodes in a systemic manner. Chaos Monkey tests the resilience of cluster elements, applications, and infrastructure components.

## Monitoring 

Our `NodeGroup` implementation exports an availability metrics collected by Prometheus.

### What information does Prometheus collect, and in what form?

Metrics have the prefix `d8_node_group_`.

All the collected metrics have a label that allow you to identify the node group: `node_group_name`.

Metrics collected for each node group:
- `d8_node_group_ready` — the number of ready Kubernetes nodes in the group.
- `d8_node_group_nodes` — the number of Kubernetes nodes (in any state) in the group.
- `d8_node_group_instances` — the number of instances (in any state) in the group.
- `d8_node_group_desired` — the number of desired `Machines` in the group.
- `d8_node_group_min` — the minimal amount of instances in the group.
- `d8_node_group_max` — the maximum amount of instances in the group.
- `d8_node_group_up_to_date` — the number of up-to-date nodes in the group.
- `d8_node_group_standby` — the number of over provisioned instances (see [standby](cr.html#nodegroup-v1-spec-cloudinstances-standby) parameter) in the group.
- `d8_node_group_has_errors` — the boolean value equal to 1 if there are errors in the group.
