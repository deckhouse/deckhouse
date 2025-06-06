---
title: "Node management"
permalink: en/admin/configuration/platform-scaling/node.html
---

## Overview

Node management in Deckhouse Kubernetes Platform (DKP) is implemented using the `node-manager` module. This module enables:

- Automatic node scaling based on load (autoscaling);
- Node updates and maintaining them in an up-to-date state;
- Simplified management of node group configurations via the NodeGroup CRD;
- Use of different types of nodes: permanent, ephemeral, cloud-based, or bare-metal.

> DKP can work with both bare-metal and cloud clusters, providing flexibility and scalability.

## Enabling the node-manager

You can enable or disable the module in several ways:

1. Via a ModuleConfig/node-manager resource:

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

1. Using a command:

   ```console
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable node-manager
   # or disable
   ```

1. Through the [Deckhouse web interface](https://deckhouse.io/products/kubernetes-platform/modules/console/stable/):

   - Go to the “Deckhouse → Modules” section;
   - Find the `node-manager` module and click on it;
   - Enable the “Module enabled” toggle switch.

## Automatic provisioning and updating

Deckhouse Kubernetes Platform (DKP) implements an automated mechanism for managing the node lifecycle based on NodeGroup objects. DKP supports both initial provisioning and updating of nodes when the configuration changes, in both cloud and bare-metal clusters (if integrated with `node-manager`).

How it works:

1. NodeGroup is the main object for managing node groups. It defines the node type, number of nodes, resource templates, and key parameters (e.g., kubelet settings, taints, etc.).
1. When a NodeGroup is created or modified, the `node-manager` module automatically brings the actual nodes in line with the specified configuration.
1. Updates happen automatically — outdated nodes are removed and new ones are created without user intervention.

Example: automatic kubelet version update.

1. The user modifies parameters in the `kubelet` section of a NodeGroup.
1. DKP detects that current nodes do not match the updated configuration.
1. New nodes with the updated configuration are created sequentially.
1. The old nodes are gradually removed from the cluster.

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

### Disruptive updates

Some updates, such as upgrading the `containerd` version or upgrading kubelet across multiple versions,
require node downtime and may cause brief unavailability of system components (*disruptive updates*).
The update mode of such cases is controlled by the [`disruptions.approvalMode`](../../../reference/cr/nodegroup/#nodegroup-v1-spec-disruptions-approvalmode) parameter:

- `Manual`: Manual approval mode for disruptive updates.
  When a disruptive update becomes available, a special alert is triggered.

  To approve the update, add the annotation `update.node.deckhouse.io/disruption-approved=` to each node in the group,
  as in the following example:

  ```shell
  sudo -i d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

  > **Important**. In this mode, nodes are not drained automatically.
  > If needed, perform a manual drain before applying the annotation.
  >
  > To avoid issues during drain, always use the `Manual` mode for master node groups.

- `Automatic`: Automatic approval of disruptive updates.

  In this mode, the node is automatically drained by default before the update is applied.
  You can modify this behavior using the [`disruptions.automatic.drainBeforeApproval`](../../../reference/cr/nodegroup/#nodegroup-v1-spec-disruptions-automatic-drainbeforeapproval) parameter in the node configuration.

- `RollingUpdate`: A mode when a new node is created with updated settings, and the old one is removed.
  This mode is applicable to cloud nodes only.

  During the update, an additional temporary node is added to the cluster.
  This can be useful when the cluster lacks enough resources
  to temporarily handle the workload from the node that is being updated.

## Node types and addition mechanics

In Deckhouse, nodes are categorized into the following types:

- **Static** — managed manually; `node-manager` neither scales nor recreates them.
- **CloudStatic** — created manually or via external tools, located in the same cloud integrated with one of the cloud provider modules.
- **CloudPermanent** — persistent nodes that are created and updated by `node-manager`.
- **CloudEphemeral** — temporary nodes that are created and scaled dynamically based on load.

Nodes are added to the cluster by creating a `NodeGroup` object, which describes the type, parameters, and configuration of a node group. For `CloudEphemeral` groups, DKP interprets this object and automatically creates the corresponding nodes, registering them in the Kubernetes cluster. For other types (such as `CloudPermanent` or `Static`), node creation and registration must be done manually or by external tools.

## Adding nodes to a bare-metal cluster

### Manual method

1. Enable the `node-manager` module.

1. Create a `NodeGroup` object of type `Static`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
   ```

   In this resource specification, set the node type to `Static`. For all `NodeGroup` objects in the cluster, a `bootstrap.sh` script will be automatically generated. This script is used to manually add nodes to the corresponding group. When adding nodes manually, you need to copy this script to the server and execute it.

   You can obtain the script in the Deckhouse web UI under the "Node Groups → Scripts" tab, or using the following `kubectl` command:

   ```console
   kubectl -n d8-cloud-instance-manager get secrets manual-bootstrap-for-worker -ojsonpath="{.data.bootstrap\.sh}"
   ```

   The script must be decoded from Base64 and then executed as `root`.

1. Once the script has run, the server will be added to the cluster as a node of the group it was bootstrapped for.

### Automated method

In DKP, it's possible to automatically add physical (bare-metal) servers to the cluster without manually running the installation script on each node. To do this, you need to:

1. Prepare the server (OS and network):
   - Install a supported operating system;
   - Configure the network and ensure the server is reachable via SSH;
   - Create a system user (e.g., `ubuntu`) for SSH access;
   - Ensure this user can execute commands using `sudo`.

1. Create an `SSHCredentials` object to provide access to the server. DKP uses this object to connect to servers via SSH. It includes:
   - A private SSH key;
   - The OS user;
   - The SSH port;
   - (Optionally) the `sudo` password, if required.

   Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: SSHCredentials
   metadata:
     name: static-nodes
   spec:
     privateSSHKey: |
       -----BEGIN OPENSSH PRIVATE KEY-----
       LS0tLS1CRUdJlhrdG...................VZLS0tLS0K
       -----END OPENSSH PRIVATE KEY-----
     sshPort: 22
     sudoPassword: password
     user: ubuntu
   ```

   > **Warning**. The private key must match the public key added to the `~/.ssh/authorized_keys` file on the server.
  
1. Create a `StaticInstance` object for each server:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
     labels:
       static-node: auto
   spec:
     address: 192.168.1.10
     credentialsRef:
       apiVersion: deckhouse.io/v1alpha1
       kind: SSHCredentials
       name: static-nodes
   ```

   A separate StaticInstance resource must be created for each server, but you can use the same SSHCredentials for access to multiple servers.

1. Create a `NodeGroup` with a description of how DKP will use these servers:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 3
       labelSelector:
         matchLabels:
           static-node: auto
     nodeTemplate:
       labels:
         node-role.deckhouse.io/worker: ""
   ```

   Here, parameters are added that describe how StaticInstances will be used: count specifies how many nodes will be added to this group; in `labelSelector`, rules are specified for selecting nodes.

After the node group is created, a script will appear to add servers to this group. DKP will wait for the necessary number of `StaticInstance` objects, matching the label selector, to appear in the cluster. Once such an object appears, DKP will retrieve the server's IP address and SSH connection parameters from the previously created manifests, connect to the server, and execute the `bootstrap.sh` script. After this, the server will be added to the specified group as a node.

## Adding nodes to a cloud cluster

### Adding CloudPermanent nodes to a cloud cluster

To add `CloudPermanent` nodes to a DKP cloud cluster:

1. Make sure the cloud provider module is enabled — for example, `cloud-provider-aws`, `cloud-provider-openstack`, `cloud-provider-yandex`, etc.

   You can verify this with the following command:

   ```console
   kubectl -n d8-system get modules
   ```

   Or check in the Deckhouse web interface.

1. Create a NodeGroup object of type `CloudPermanent`. These nodes are managed using Terraform, which is built into DKP. The configuration for such nodes resides in the `(Provider)ClusterConfiguration` object. You must edit it using the `dhctl` utility inside the installer container. Example:

   ```yaml
   nodeGroups:
   - name: cloud-permanent
     replicas: 2
     instanceClass:
       flavorName: m1.large
       imageName: ubuntu-22-04-cloud-amd64
       rootDiskSize: 20
       mainNetwork: default
     volumeTypeMap:
       nova: ceph-ssd
   ```

1. Specify instance template parameters. The fields inside `instanceClass` depend on the specific cloud provider. Below is an example for OpenStack:
   - `flavorName` — instance type (resources: CPU, RAM);
   - `imageName` — OS image;
   - `rootDiskSize` — size of the system disk (in GB);
   - `mainNetwork` — network name;
   - optionally: ETCD disk, zones, volume types, etc.

   For other clouds, the field names and structure may vary. Refer to the CRD documentation or the respective cloud provider’s documentation for accurate parameters.

1. Apply the configuration using `dhctl converge`. After editing the `(Provider)ClusterConfiguration`, run:

   ```console
   dhctl converge \
     --ssh-host <master node IP> \
     --ssh-user <username> \
     --ssh-agent-private-keys /tmp/.ssh/<key>
   ```

   This command will:
   - launch Terraform,
   - create the required virtual machines,
   - perform DKP installation on them (via `bootstrap.sh`),
   - register the nodes in the cluster.

1. Done — the new nodes will appear in the cluster automatically. You can view them by running:

   ```console
   kubectl get nodes
   ```

   Or via the Deckhouse web interface.

### Adding CloudEphemeral Nodes to a Cloud Cluster

CloudEphemeral nodes are automatically created and managed in the cluster using Machine Controller Manager (MCM) or Cluster API (depending on the configuration) — both components are part of the `node-manager` module in DKP.

To add nodes:

1. Ensure the cloud provider module is enabled. For example: `cloud-provider-yandex`, `cloud-provider-openstack`, `cloud-provider-aws`.

1. Create an `InstanceClass` object with the VM configuration. This object describes the parameters of virtual machines that will be created in the cloud:

   Example (for OpenStack):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: OpenStackInstanceClass
   metadata:
     name: worker-instance
   spec:
     flavorName: m1.medium
     imageName: ubuntu-22-04-cloud-amd64
     rootDiskSize: 20
     mainNetwork: default
   ```

   The following parameters are specified here:
   - `flavorName` — instance type (CPU/RAM);
   - `imageName` — operating system image;
   - `rootDiskSize` — root disk size (in GB);
   - `mainNetwork` — cloud network for the instance.

1. Create a NodeGroup of type `CloudEphemeral`. Example manifest:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: workers
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: OpenStackInstanceClass
         name: worker-instance
       minPerZone: 1
       maxPerZone: 3
       zones:
         - nova
     nodeTemplate:
       labels:
         node-role.deckhouse.io/worker: ""
       taints: []
   ```

1. Wait for the nodes to be automatically launched and added to the cluster.

## Node Group configuration

In Deckhouse, each group of nodes is configured using a `NodeGroup` object. This object defines the parameters of the nodes that DKP will create or connect to the cluster.

Default values for many fields (e.g., `nodeTemplate`, `kubelet`, `disruptions`, `taints`) can be specified using a `NodeGroupConfiguration` object. This is especially useful when multiple NodeGroups in a cluster share the same configuration.

This allows you to:

- Manage settings for all node groups centrally;
- Define consistent values without duplicating them in each NodeGroup;
- Change node parameters in the cluster without manually editing every NodeGroup object.

### General settings

Regardless of the infrastructure type (cloud or bare metal), a `NodeGroup` object includes a number of parameters that determine the behavior and characteristics of the nodes. Example structure:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    zones:
      - eu-west-1a
      - eu-west-1b
    minPerZone: 1
    maxPerZone: 2
    classReference:
      kind: AWSInstanceClass
      name: test
  nodeTemplate:
    labels:
      tier: test
```

## Settings for Static and CloudStatic Node Groups

Node groups of type `Static` and `CloudStatic` are designed to manage manually created nodes — both physical (bare-metal) and virtual (in the cloud, but without using DKP’s automated controllers). These nodes are connected manually or via `StaticInstance` resources and do not support automatic updates or scaling.

Configuration specifics:

- All update operations (kubelet version updates, restarts, node replacements) are performed manually or via external automation outside of DKP.

- It is recommended to explicitly specify the desired kubelet version to ensure consistency across nodes, especially if they are connected manually with different versions:

  ```yaml
  nodeTemplate:
    kubelet:
      version: "1.28"
  ```

- Nodes can be connected to the cluster manually or automatically, depending on the configuration:
  - Manually — the user downloads the bootstrap script, sets up the server, and runs the script manually.
  - Automatically (CAPS) — when using `StaticInstance` and `SSHCredentials`, DKP connects and configures nodes automatically.
  - Hybrid — a manually added node can be handed over to CAPS management by applying the annotation `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

If Cluster API Provider Static (CAPS) is enabled, the `staticInstances` section can be used in the NodeGroup. This allows DKP to automatically connect, configure, and (if needed) disconnect static nodes based on `StaticInstance` and `SSHCredentials` resources.

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static-workers
spec:
  nodeType: Static
  staticInstances:
    count: 2
    labelSelector:
      matchExpressions: []
      matchLabels:
        static-node: group-a
```

### Settings for CloudEphemeral Node Groups

Node groups of type `CloudEphemeral` are designed for automatic scaling by creating and removing virtual machines in the cloud using Machine Controller Manager (MCM). This type of group is widely used in cloud-based DKP clusters.

Node configuration is specified in the `cloudInstances` section and includes parameters for scaling, zoning, fault tolerance, and prioritization.

Basic configuration example:

```yaml
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 1
    maxPerZone: 5
    maxUnavailablePerZone: 1
    zones:
    - ru-central1-a
    - ru-central1-b
```

## Node Group autoscaling

In Deckhouse Kubernetes Platform (DKP), node group autoscaling is implemented for groups of type `CloudEphemeral`. Scaling is based on resource needs (CPU and memory) and is performed by the `Cluster Autoscaler` component, which is part of the `node-manager` module.

Autoscaling is triggered only when there are Pending pods that cannot be scheduled on existing nodes due to a lack of resources (e.g., CPU or memory). In such cases, `Cluster Autoscaler` attempts to add nodes based on the NodeGroup configuration.

Key autoscaling parameters are specified in the `cloudInstances` section of the NodeGroup resource:

- `minPerZone` — the minimum number of virtual machines in each zone. This number is always maintained even if there is no load.
- `maxPerZone` — the maximum number of nodes that can be created in each zone. This sets the upper scaling limit.
- `maxUnavailablePerZone` — limits the number of unavailable nodes during updates, deletion, or provisioning.
- `standby` — an optional parameter that allows pre-launching additional standby nodes.
- `priority` — an integer priority value. When scaling, `Cluster Autoscaler` prioritizes node groups with higher `priority` values. This is used to control the scaling order among multiple node groups.

Example node group configuration with autoscaling:

```yaml
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 1         # Minimum number of nodes per zone.
    maxPerZone: 5         # Maximum number of nodes per zone.
    maxUnavailablePerZone: 1  # Number of nodes that can be updated/deleted simultaneously.
    zones:
      - nova
      - supernova
      - hypernova
```

### Autoscaling scenario example

Suppose you have the following node group configuration:

```yaml
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: m4.large
    minPerZone: 1
    maxPerZone: 5
    zones:
      - nova
      - supernova
      - hypernova
```

You also have a Deployment with the following configuration:

```yaml
kind: Deployment
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        resources:
          requests:
            cpu: 1500m
            memory: 5Gi
```

Each VM can accommodate only one such pod. Therefore, to run 3 replicas, 3 nodes are required — one in each zone.

Now, let's increase the number of replicas to 5. Two pods will end up in a `Pending` state. The Cluster Autoscaler will:

- Detect the resource shortage;
- Calculate how many additional resources are needed;
- Decide to create 2 more nodes;
- Forward the task to the Machine Controller Manager;
- As a result, 2 new VMs will be created in the cloud and automatically joined to the cluster;
- The pending pods will then be scheduled onto the newly added nodes.

## Moving a node between NodeGroups

> **Warning:** When moving nodes between NodeGroups, the node will be wiped and re-bootstrapped. The Kubernetes `Node` object will be recreated.

1. Create a new `NodeGroup` resource, for example named `front`, which will manage the static node labeled `role: front`:

   ```yaml
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: front
   EOF
   ```

1. Change the `role` label of the existing `StaticInstance` from `worker` to `front`. This will allow the new `NodeGroup` named `front` to take control of the node:

   ```console
   kubectl label staticinstance static-worker-1 role=front --overwrite
   ```

1. Update the `worker` NodeGroup resource by decreasing the `count` parameter from `1` to `0`:

   ```console
   kubectl patch nodegroup worker -p '{"spec": {"staticInstances": {"count": 0}}}' --type=merge
   ```

## NodeGroup examples

### Cloud nodes

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    zones:
      - eu-west-1a
      - eu-west-1b
    minPerZone: 1
    maxPerZone: 2
    classReference:
      kind: AWSInstanceClass
      name: test
  nodeTemplate:
    labels:
      tier: test
```

### Static Nodes

For virtual machines on hypervisors or physical servers, use static nodes by specifying `nodeType: Static` in the NodeGroup.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

Nodes in such a group are added manually using pre-generated scripts or automatically via Cluster API Provider Static (CAPS).

### Example of a system NodeGroup

System nodes are intended for running system components. They are usually marked with specific labels and taints to prevent user pods from being scheduled on them. System nodes can be either static or cloud-based.

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

## NodeGroupConfiguration examples

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

### Setting a sysctl parameter

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: sysctl-tune.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "*"
  content: |
    sysctl -w vm.max_map_count=262144
```

### Adding a root certificate to the host

{% alert level="warning" %}
This example is intended for Ubuntu OS.  
The method for adding certificates to the system store may vary depending on the OS.  
Adjust the `bundles` and `content` fields accordingly when adapting the script to another OS.
{% endalert %}

{% alert level="warning" %}
For `containerd` to use the certificate, the service must be restarted after adding the certificate.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca.sh
spec:
  weight: 31
  nodeGroups:
  - '*'  
  bundles:
  - 'ubuntu-lts'
  content: |-
    CERT_FILE_NAME=example_ca
    CERTS_FOLDER="/usr/local/share/ca-certificates"
    CERT_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    ...
    -----END CERTIFICATE-----
    EOF
    )

    bb-event-on "ca-file-updated" "update-certs"

    update-certs() {
      update-ca-certificates
    }

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated
```

### Adding a certificate to the OS and containerd

{% alert level="warning" %}
This example is intended for Ubuntu OS.  
The method for adding certificates to the system store may vary depending on the OS.  
Adjust the `bundles` and `content` fields accordingly when adapting the script to another OS.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca-containerd..sh
spec:
  weight: 31
  nodeGroups:
  - '*'  
  bundles:
  - 'ubuntu-lts'
  content: |-
    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/usr/local/share/ca-certificates"
    CERT_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )
    CONFIG_CONTENT=$(cat <<EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
        ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
    )
    
    mkdir -p /etc/containerd/conf.d

    # bb-tmp-file - Create temp file function. More information: http://www.bashbooster.net/#tmp

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    CONFIG_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CONFIG_CONTENT}" > "${CONFIG_TMP_FILE}"  

    # bb-event           - Creating subscription for event function. More information: http://www.bashbooster.net/#event
    ## ca-file-updated   - Event name
    ## update-certs      - The function name that the event will call
    
    bb-event-on "ca-file-updated" "update-certs"
    
    update-certs() {          # Function with commands for adding a certificate to the store
      update-ca-certificates  # Restarting the containerd service is not required as this is done automatically in the script 032_configure_containerd.sh
    }

    # bb-sync-file                                - File synchronization function. More information: http://www.bashbooster.net/#sync
    ## "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"    - Destination file
    ##  ${CERT_TMP_FILE}                          - Source file
    ##  ca-file-updated                           - Name of event that will be called if the file changes.

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated   
      
    bb-sync-file \
      "/etc/containerd/conf.d/${REGISTRY_URL}.toml" \
      ${CONFIG_TMP_FILE} 
```
