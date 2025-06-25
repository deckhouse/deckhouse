---
title: "Adding and managing cloud nodes"
permalink: en/admin/configuration/platform-scaling/node/cloud-node.html
---

In Deckhouse Kubernetes Platform, cloud nodes can be of the following types:

- **CloudEphemeral** — temporary nodes that are automatically created and deleted;
- **CloudPermanent** — permanent nodes managed manually via `replicas`;
- (optional) **CloudStatic** — nodes created outside of Deckhouse but integrated into the cluster;
- (optional) **CloudHybrid** — nodes managed in coordination with external systems.

Below are instructions for adding and configuring each type.

## Adding CloudEphemeral nodes in a cloud cluster

CloudEphemeral nodes are automatically created and managed within the cluster using the Machine Controller Manager (MCM) or Cluster API (depending on configuration) — both components are part of the `node-manager` module in DKP.

To add nodes:

1. Ensure that the cloud provider module is enabled. For example: `cloud-provider-yandex`, `cloud-provider-openstack`, `cloud-provider-aws`.

1. Create an `InstanceClass` object with the machine configuration. This object describes the parameters of the virtual machines to be created in the cloud.

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
  
   The following parameters are specified:

   - `flavorName` — instance type (CPU/RAM);
   - `imageName` — OS image;
   - `rootDiskSize` — size of the root disk;
   - `mainNetwork` — cloud network for the instance.

1. Create a NodeGroup with the `CloudEphemeral` type. Example manifest:

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

1. Wait for the nodes to be launched and added automatically.

## Configuration for CloudEphemeral NodeGroups

NodeGroups of type `CloudEphemeral` are designed for automatic scaling by creating and removing virtual machines in the cloud using the Machine Controller Manager (MCM). This type of group is commonly used in cloud-based DKP clusters.

Node configuration is defined in the `cloudInstances` section and includes parameters for scaling, zoning, fault tolerance, and prioritization.

Example basic configuration:

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

## NodeGroup autoscaling

In Deckhouse Kubernetes Platform (DKP), node group autoscaling is implemented for NodeGroups of type `CloudEphemeral`. Scaling is performed based on resource demands (CPU and memory) by the `Cluster Autoscaler` component, which is part of the `node-manager` module.

Autoscaling is triggered only when there are Pending pods that cannot be scheduled on existing nodes due to insufficient resources (e.g., CPU or memory). In this case, `Cluster Autoscaler` attempts to add nodes based on the NodeGroup configuration.

Key scaling parameters are defined in the `cloudInstances` section of the NodeGroup resource:

- `minPerZone` — the minimum number of virtual machines per zone. This number is always maintained, even with no workload.
- `maxPerZone` — the maximum number of nodes that can be created per zone. This defines the upper scaling limit.
- `maxUnavailablePerZone` — limits the number of unavailable nodes during updates, deletions, or creation.
- `standby` — optional parameter that allows pre-provisioning standby nodes.
- `priority` — integer priority of the group. When scaling, `Cluster Autoscaler` prefers groups with a higher `priority` value. This allows you to define scaling order among multiple NodeGroups.

Example configuration of a NodeGroup with autoscaling:

```yaml
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 1         # Minimum number of nodes per zone.
    maxPerZone: 5         # Maximum number of nodes per zone.
    maxUnavailablePerZone: 1  # Number of nodes that can be updated/removed simultaneously.
    zones:
      - nova
      - supernova
      - hypernova
```

### Example autoscaling scenario

Assume the following NodeGroup configuration:

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

There is also a Deployment with the following configuration:

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

Each VM can host only one such Pod. Therefore, 3 replicas require 3 nodes — one in each zone.

Now let’s increase the number of replicas to 5. Two Pods will end up in `Pending` state. The Cluster Autoscaler will:

- Detect the situation;
- Calculate how much resource is missing;
- Decide to create two more nodes;
- Hand off the task to the Machine Controller Manager;
- Two new VMs will be created in the cloud and automatically join the cluster;
- The Pods will be scheduled onto the new nodes.

## Example NodeGroup definition

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

## How to allocate nodes to specific loads

{% alert level="warning" %}
You cannot use the `deckhouse.io` domain in `labels` and `taints` keys of the `NodeGroup`. It is reserved for Deckhouse components. Please, use the `dedicated` or `dedicated.client.com` keys.
{% endalert %}

There are two ways to solve this problem:

1. You can set labels to `NodeGroup`'s `spec.nodeTemplate.labels`, to use them in the `Pod`'s [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) or [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) parameters. In this case, you select nodes that the scheduler will use for running the target application.
1. You cat set taints to `NodeGroup`'s `spec.nodeTemplate.taints` and then remove them via the `Pod`'s [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) parameter. In this case, you disallow running applications on these nodes unless those applications are explicitly allowed.

{% alert level="info" %}
Deckhouse tolerates the `dedicated` by default, so we recommend using the `dedicated` key with any `value` for taints on your dedicated nodes.️

To use custom keys for `taints` (e.g., `dedicated.client.com`), you must add the key's value to the [modules.placement.customTolerationKeys](../../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys) parameters. This way, deckhouse can deploy system components (e.g., `cni-flannel`) to these dedicated nodes.
{% endalert %}

## How to speed up node provisioning on the cloud when scaling applications horizontally

The most efficient way is to have some extra nodes "ready". In this case, you can run new application replicas on them almost instantaneously. The obvious disadvantage of this approach is the additional maintenance costs related to these nodes.

Here is how you should configure the target `NodeGroup`:

1. Specify the number of "ready" nodes (or a percentage of the maximum number of nodes in the group) using the `cloudInstances.standby` paramter.
1. If there are additional service components on nodes that are not handled by Deckhouse (e.g., the `filebeat` DaemonSet), you can specify the percentage of node resources they can consume via the `standbyHolder.overprovisioningRate` parameter.
1. This feature requires that at least one group node is already running in the cluster. In other words, there must be either a single replica of the application, or the `cloudInstances.minPerZone` parameter must be set to `1`.

An example:

```yaml
cloudInstances:
  maxPerZone: 10
  minPerZone: 1
  standby: 10%
  standbyHolder:
    overprovisioningRate: 30%
```

## Configuring CloudEphemeral nodes using NodeGroupConfiguration

Additional settings for cloud nodes can be defined using `NodeGroupConfiguration` objects. These allow you to:

- Modify OS parameters (e.g., `sysctl`);
- Add root certificates;
- Configure trust for private image registries, etc.

`NodeGroupConfiguration` is applied to new nodes at creation time, including `CloudEphemeral` nodes.
  
> `NodeGroupConfiguration` can only be applied to nodes with specific OS images by specifying the appropriate `bundle`. For example:
>
> - `ubuntu-lts`
> - `centos-7`
> - `rocky-linux`
> - `*` — applies to all.

### Example NodeGroupConfiguration definitions

#### Setting a sysctl parameter

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

#### Adding a root certificate to the host

{% alert level="warning" %}
This example is for Ubuntu OS.  
The method for adding certificates to the trust store may differ depending on the OS.  
When adapting the script to another OS, adjust the `bundles` and `content` fields accordingly.
{% endalert %}

{% alert level="warning" %} 
To use the certificate in `containerd`, you must restart the service after adding the certificate.
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

    # bb-event           - Creating subscription for event function. More information: http://www.bashbooster.net/#event
    ## ca-file-updated   - Event name
    ## update-certs      - The function name that the event will call
    
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

#### Adding a certificate to the OS and containerd

{% alert level="warning" %} 
This example is for Ubuntu OS.  
The method for adding certificates to the trust store may differ depending on the OS.  
When adapting the script to another OS, update the `bundles` and `content` parameters accordingly.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca-containerd.sh
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
    ...
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

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    CONFIG_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CONFIG_CONTENT}" > "${CONFIG_TMP_FILE}"  

    bb-event-on "ca-file-updated" "update-certs"
    
    update-certs() {
      update-ca-certificates  # containerd restart is handled by script 032_configure_containerd.sh
    }

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated   
      
    bb-sync-file \
      "/etc/containerd/conf.d/${REGISTRY_URL}.toml" \
      ${CONFIG_TMP_FILE} 
```

### How do I update kernel on nodes

#### Debian-based distros

Create a `Node Group Configuration` resource by specifying the desired kernel version in the `desired_version` variable of the shell script (the resource's spec.content parameter):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="5.15.0-53-generic"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-apt-install "linux-image-${desired_version}"
```

#### CentOS-based distros

Create a `Node Group Configuration` resource by specifying the desired kernel version in the `desired_version` variable of the shell script (the resource's spec.content parameter):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="3.10.0-1160.42.2.el7.x86_64"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-dnf-install "kernel-${desired_version}"
```

## Adding CloudPermanent тodes to a сloud сluster

To add `CloudPermanent` nodes to a DKP cloud cluster:

1. Make sure the cloud provider module is enabled. For example: `cloud-provider-aws`, `cloud-provider-openstack`, `cloud-provider-yandex`, etc.

   You can check this by running the following command:

   ```console
   kubectl -n d8-system get modules
   ```

   Or view it in the Deckhouse web interface.

1. Create a `NodeGroup` object with the type `CloudPermanent`. Nodes of this type are managed via Terraform, which is integrated into DKP. The configuration for such nodes is located in the `(Provider)ClusterConfiguration` object. You must edit this configuration using the `dhctl` utility from the installation container. Example:

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

1. Specify the instance template parameters. The fields inside the `instanceClass` section depend on the specific cloud provider. Below is an example for OpenStack:
   - `flavorName` — instance type (resources: CPU, RAM);
   - `imageName` — OS image;
   - `rootDiskSize` — size of the root disk (in GB);
   - `mainNetwork` — network name;
   - if needed: ETCD disk, zones, volume types, etc.

   For other cloud providers, the field names and structure may differ. Refer to the CRD definition or the documentation for the corresponding cloud provider for the actual fields.

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
   - install DKP on them (using `bootstrap.sh`),
   - register the nodes in the cluster.

1. Done — the new nodes will automatically appear in the cluster.  
   You can view them by running:

   ```console
   kubectl get nodes
   ```

   Or in the Deckhouse web interface.

Deckhouse Kubernetes Platform can run on top of Managed Kubernetes services (e.g., GKE and EKS).  
In such cases, the `node-manager` module provides node configuration management and automation,  
but its capabilities may be limited by the respective cloud provider's API.

## Adding master nodes in a cloud cluster

To add master nodes in a cloud cluster:

1. Make sure the `control-plane-manager` module is enabled.

1. Open the `ClusterConfiguration` file (e.g., `OpenStackClusterConfiguration`).

1. Add or update the `masterNodeGroup` section:

   ```yaml
   masterNodeGroup:
     replicas: 3
     instanceClass:
       flavorName: m1.medium
       imageName: ubuntu-22-04-cloud-amd64
       rootDiskSize: 20
       mainNetwork: default
   ```

1. Apply the changes using `dhctl converge`:

   ```console
   dhctl converge \
     --ssh-host <master node IP> \
     --ssh-user <username> \
     --ssh-agent-private-keys /tmp/.ssh/<key>
   ```

## How to interpret Node Group states?

**Ready** — the node group contains the minimum required number of scheduled nodes with the status ```Ready``` for all zones.

Example 1. A group of nodes in the ``Ready`` state:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

Example 2. A group of nodes in the ``Not Ready`` state:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

**Updating** — a node group contains at least one node in which there is an annotation with the prefix `update.node.deckhouse.io` (for example, `update.node.deckhouse.io/waiting-for-approval`).

**WaitingForDisruptiveApproval** - a node group contains at least one node that has an annotation `update.node.deckhouse.io/disruption-required` and
there is no annotation `update.node.deckhouse.io/disruption-approved`.

**Scaling** — calculated only for node groups with the type `CloudEphemeral`. The state `True` can be in two cases:

1. When the number of nodes is less than the *desired number of nodes* in the group, i.e. when it is necessary to increase the number of nodes in the group.
1. When a node is marked for deletion or the number of nodes is greater than the *desired number of nodes*, i.e. when it is necessary to reduce the number of nodes in the group.

The *desired number of nodes* is the sum of all replicas in the node group.

Example. The desired number of nodes is 2:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
...
  desired: 2
...
```

**Error** — contains the last error that occurred when creating a node in a node group.
