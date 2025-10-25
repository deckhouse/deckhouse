---
title: "Node management basics in Deckhouse"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/node-management.html
---

Deckhouse Virtualization Platform (DVP) supports the full lifecycle of node management:

- Automatic node scaling based on workload.
- Node updates and maintenance to keep them up to date.
- Centralized configuration management for node groups using the NodeGroup CRD.
- Support for various types of nodes: permanent, ephemeral, or bare-metal.

{% alert level="info" %}
DVP can operate on bare-metal clusters, providing flexibility and scalability.
{% endalert %}

Node groups allow logical segmentation of the cluster infrastructure. In DVP, the following [NodeGroup](/modules/node-manager/cr.html#nodegroup) roles are commonly used:

- `master`: Control plane nodes.
- `front`: Nodes for routing HTTP(S) traffic.
- `monitoring`: Nodes for hosting monitoring components.
- `worker`: Nodes for user applications.
- `system`: Dedicated nodes for system components.

Each group can have centralized configuration settings, including the Kubernetes version, resources, taints, labels, kubelet parameters, and more.

## Enabling the node management mechanism

Node management is implemented via the [`node-manager`](/modules/node-manager/) module, which can be enabled or disabled in several ways:

1. Using the ModuleConfig/node-manager resource:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: node-manager
   spec:
     version: 2
     enabled: true
     settings:
       earlyOomEnabled: true
       instancePrefix: kube
       mcmEmergencyBrake: false
   ```

1. Using the command:

   ```shell
   d8 system module enable node-manager
   # Or disable.
   ```

1. Using the [Deckhouse web interface](/modules/console/):

   - Go to the "Deckhouse → Modules" section.
   - Find the `node-manager` module and click on it.
   - Toggle the "Module enabled" switch.

## Automatic deployment and updates

Deckhouse Virtualization Platform (DVP) provides an automated mechanism for managing the lifecycle of nodes based on [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources. DVP supports both initial node provisioning and updates when configuration changes, for bare-metal clusters (if the `node-manager` module is enabled).

How it works:

1. A NodeGroup is the primary resource for managing groups of nodes. It defines the node type, number of nodes, resource templates, and key parameters (e.g., kubelet settings, taints, etc.).
1. When a NodeGroup is created or modified, the `node-manager` module automatically reconciles the node state with the specified configuration.
1. Updates occur without user intervention — outdated nodes are removed, and new ones are created automatically.

Let's take a look at automatic updates using the example of a kubelet version upgrade.

1. The user updates the `kubelet` section in the NodeGroup specification.
1. DVP detects that current nodes do not match the new configuration.
1. New nodes with the updated settings are created sequentially.
1. Old nodes are gradually removed from the cluster.

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker-cloud
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: AnotherCloudInstanceClass
         name: my-class
    ```

## Basic node and OS configuration

When nodes are created and joined to the cluster, DVP automatically performs a series of actions required for proper cluster operation:

- Installing and configuring a supported operating system.
- Disabling automatic package updates.
- Setting up logging and system parameters.
- Installing necessary packages and utilities.
- Configuring the `nginx` component to balance traffic between `kubelet` and API servers.
- Installing and configuring the container runtime (`containerd`) and `kubelet`.
- Registering the node with the Kubernetes cluster.

These actions are performed automatically when using `bootstrap.sh` or when connecting nodes via [StaticInstance](/modules/node-manager/cr.html#staticinstance) and [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) resources.

### Updates that require node downtime

Some updates — for example, upgrading `containerd` or kubelet across multiple versions — require node downtime and may cause short-term disruption of system components (*disruptive updates*).  
The application mode for such updates is configured via the `disruptions.approvalMode` parameter:

- `Manual`: Manual approval mode for disruptive updates.  
  When a disruptive update is available, the [`NodeRequiresDisruptionApprovalForUpdate`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#node-manager-noderequiresdisruptionapprovalforupdate) alert is triggered.

  To approve the update, add the annotation `update.node.deckhouse.io/disruption-approved=` to each node in the group. Example:

  ```shell
  d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

  > **Important**: In this mode, the node is not drained automatically.  
  > If needed, perform the drain manually before applying the annotation.  
  >
  > To avoid issues during draining,
  > always use the `Manual` mode for master node groups.

- `Automatic`: Automatic approval mode for disruptive updates.  

  In this mode, the node is drained automatically before applying the update by default.  
  This behavior can be changed using the `disruptions.automatic.drainBeforeApproval` parameter in the node group settings.

- `RollingUpdate`: A mode in which a new node with updated settings is created and the old one is removed.

  In this mode, an additional node is created in the cluster during the update.  
  This can be useful if the cluster lacks sufficient resources to temporarily relocate workloads from the updating node.

## Example of a system NodeGroup

System nodes are dedicated to running system components.  
They are typically isolated using labels and taints to prevent user pods from being scheduled on them.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: Static
```

## Example NodeGroupConfiguration descriptions

### Installing the cert-manager plugin for kubectl on master nodes

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-cert-manager-plugin.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "master"
  content: |
    if [ -x /usr/local/bin/kubectl-cert_manager ]; then
      exit 0
    fi
    curl -L https://github.com/cert-manager/cert-manager/releases/download/v1.7.1/kubectl-cert_manager-linux-amd64.tar.gz -o - | tar -zxvf - kubectl-cert_manager
    mv kubectl-cert_manager /usr/local/bin
```

## Werf configuration for ignoring the Ready status of a node group

[Werf](https://werf.io) checks the `Ready` status of resources and, if available, waits for the value to become `True`.

Creating (or updating) a NodeGroup resource in the cluster may take a significant amount of time (until all nodes become ready). When using werf (e.g., in CI/CD), this can lead to a build timeout.

To make werf ignore the NodeGroup status, add the following annotations to the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```

## Settings for Static NodeGroups

Node groups with types Static are intended for managing manually created nodes. These nodes are connected manually or via [StaticInstance](/modules/node-manager/cr.html#staticinstance) and do not support automatic updates or scaling.

Configuration specifics:

- All update operations (e.g., kubelet updates, node restarts, replacements) must be performed manually or through external automation tools outside of DVP.

- It is recommended to explicitly set the desired kubelet version to ensure consistency across nodes, especially if they are added with different versions manually:

  ```yaml
  nodeTemplate:
     kubelet:
       version: "1.28"
  ```

- Node registration to the cluster can be performed either manually or automatically, depending on the configuration:
  - **Manual**: The user downloads the bootstrap script, configures the server, and runs the script manually.
  - **Automatic (CAPS)**: When using [StaticInstance](/modules/node-manager/cr.html#staticinstance) and [SSHCredentials](/modules/node-manager/cr.html#sshcredentials), DVP automatically connects and configures the nodes.
  - **Hybrid approach**: A manually added node can be handed over to CAPS by using the annotation `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

If the Cluster API Provider Static (CAPS) is enabled, the NodeGroup resource can use the `staticInstances` section. This allows DVP to automatically connect, configure, and, if necessary, clean up static nodes based on StaticInstance and SSHCredentials resources.

> In a [NodeGroup](/modules/node-manager/cr.html#nodegroup) of type Static, you can explicitly specify the number of nodes using the `spec.staticInstances.count` parameter. This allows you to define the expected number of nodes — DVP uses this value for state monitoring and automation.

## Running DVP on an arbitrary node

To run DVP on an arbitrary node, configure the `deckhouse` module with the appropriate [`nodeSelector`](/modules/deckhouse/configuration.html) parameter and **do not** specify `tolerations`. The required `tolerations` will be set automatically in this case.

{% alert level="warning" %}
Only use nodes of type **Static** to run DVP. Avoid using a `NodeGroup` that contains only a single node for running DVP.
{% endalert %}

Example module configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    nodeSelector:
      node-role.deckhouse.io/deckhouse: ""
```
