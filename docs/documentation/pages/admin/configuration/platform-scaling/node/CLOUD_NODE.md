---
title: "Adding and managing cloud nodes"
permalink: en/admin/configuration/platform-scaling/node/cloud-node.html
description: "Manage cloud nodes in Deckhouse Kubernetes Platform including CloudEphemeral, CloudPermanent, and CloudStatic nodes. Auto-scaling, node lifecycle management, and cloud provider integration."
---

In Deckhouse Kubernetes Platform (DKP), cloud nodes can be of the following types:

- **CloudEphemeral** — temporary nodes that are automatically created and deleted.
- **CloudPermanent** — permanent nodes managed manually via `replicas`.
- **CloudStatic** — static cloud nodes. The machines are created manually or by external tools and DKP connects them to the cluster and manages them just like the regular nodes.

Below are instructions for adding and configuring each type.

## Adding CloudEphemeral nodes in a cloud cluster

CloudEphemeral nodes are automatically created and managed within the cluster using the Machine Controller Manager (MCM) or Cluster API (depending on configuration) — both components are part of the [`node-manager`](/modules/node-manager/) module in DKP.

To add nodes:

1. Ensure that the cloud provider module is enabled. For example: [`cloud-provider-yandex`](/modules/cloud-provider-yandex/), [`cloud-provider-openstack`](/modules/cloud-provider-openstack/), [`cloud-provider-aws`](/modules/cloud-provider-aws/).

1. Create an [InstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) object with the machine configuration. This object describes the parameters of the virtual machines to be created in the cloud.

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

   - `flavorName`: Instance type (CPU/RAM).
   - `imageName`: OS image.
   - `rootDiskSize`: Size of the root disk.
   - `mainNetwork`: Cloud network for the instance.

1. Create a [NodeGroup](/modules/node-manager/cr.html#nodegroup) with the `CloudEphemeral` type. Example manifest:

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

NodeGroups of CloudEphemeral type are designed for automatic scaling by creating and removing virtual machines in the cloud using the Machine Controller Manager (MCM). This type of groups is commonly used in cloud-based DKP clusters.

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

## Modifying the cloud provider configuration in a cluster

The configuration of the cloud provider used in a cloud or hybrid cluster is stored in the `<PROVIDER_NAME>ClusterConfiguration` structure, where `<PROVIDER_NAME>` is the name/code of the provider. For example, for the OpenStack provider, the structure is called [OpenStackClusterConfiguration]({% if site.mode == 'module' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/products/kubernetes-platform/documentation/v1/{% endif %}/modules/cloud-provider-openstack/cluster_configuration.html).

Regardless of the cloud provider used, its settings can be modified using the following command:

```shell
d8 platform edit provider-cluster-configuration
```

## NodeGroup autoscaling

In Deckhouse Kubernetes Platform (DKP), node group autoscaling is performed based on resource demands (CPU and memory) by the `Cluster Autoscaler` component, which is part of the [`node-manager`](/modules/node-manager/) module.

Autoscaling is triggered only when there are Pending pods that cannot be scheduled on existing nodes due to insufficient resources (e.g., CPU or memory). In this case, `Cluster Autoscaler` attempts to add nodes based on the NodeGroup configuration.

Key scaling parameters are defined in the [`cloudInstances`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances) section of the NodeGroup resource:

- `minPerZone`: The minimum number of virtual machines per zone. This number is always maintained, even with no workload.
- `maxPerZone`: The maximum number of nodes that can be created per zone. This defines the upper scaling limit.
- `maxUnavailablePerZone`: Limits the number of unavailable nodes during updates, deletions, or creation.
- `standby`: Optional parameter that allows pre-provisioning standby nodes.
- `priority`: Integer priority of the group. When scaling, `Cluster Autoscaler` prefers groups with a higher `priority` value. This allows you to define scaling order among multiple NodeGroups.

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

In this case, launching all replicas will require 3 nodes — one in each zone.

Now let's increase the number of replicas to 5. As a result, two Pods will enter the `Pending` state.

`Cluster Autoscaler`:

- Detects the situation.
- Calculates the amount of lacking resources.
- Decides to create two more nodes.
- Hands off the task to the Machine Controller Manager.
- Two new VMs will be created in the cloud and automatically join the cluster.
- The Pods will be scheduled onto the new nodes.

### Allocating nodes for specific workloads

{% alert level="warning" %}
You cannot use the `deckhouse.io` domain in `labels` and `taints` keys of the [NodeGroup](/modules/node-manager/cr.html#nodegroup). It is reserved for DKP components. Use the `dedicated` or `dedicated.client.com` keys instead.
{% endalert %}

There are two ways to solve this problem:

1. You can set labels to [NodeGroup](/modules/node-manager/cr.html#nodegroup) `spec.nodeTemplate.labels`, to use them in the `Pod`'s [`spec.nodeSelector`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) or [`spec.affinity.nodeAffinity`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) parameters. In this case, you select nodes that the scheduler will use for running the target application.
1. You cat set taints to NodeGroup's `spec.nodeTemplate.taints` and then remove them via the `Pod`'s [`spec.tolerations`](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) parameter. In this case, you disallow running applications on these nodes unless those applications are explicitly allowed.

{% alert level="info" %}
DKP tolerates the `dedicated` key by default, so we recommend using the `dedicated` key with any value for taints on your dedicated nodes.️

To use custom keys for taints (e.g., `dedicated.client.com`), you must add the key's value to the `modules.placement.customTolerationKeys` parameter. This way, Deckhouse can deploy system components (e.g., `cni-flannel`) to these dedicated nodes.
{% endalert %}

### Accelerating node provisioning in the cloud during horizontal application scaling

To speed up the launch of new application replicas during automatic horizontal scaling, it is recommended to maintain a certain number of pre-provisioned (standby) nodes in the cluster.  
This allows new application Pods to be scheduled quickly without waiting for node creation and initialization.
Keep in mind that having standby nodes increases infrastructure costs.

The target [NodeGroup](/modules/node-manager/cr.html#nodegroup) configuration should be as follows:

1. Specify the absolute number of pre-provisioned nodes (or a percentage of the maximum number of nodes in the group) using the `cloudInstances.standby` parameter.
1. If there are additional service components on nodes that are not handled by Deckhouse (e.g., the `filebeat` DaemonSet), you can specify the percentage of node resources they can consume via the `standbyHolder.overprovisioningRate` parameter.
1. This feature requires that at least one group node is already running in the cluster. In other words, there must be either a single replica of the application, or the `cloudInstances.minPerZone` parameter must be set to `1`.

Example:

```yaml
cloudInstances:
  maxPerZone: 10
  minPerZone: 1
  standby: 10%
  standbyHolder:
    overprovisioningRate: 30%
```

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

## Configuring CloudEphemeral nodes using NodeGroupConfiguration

Additional settings for cloud nodes can be defined using [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) objects. These allow you to:

- Modify OS parameters (e.g., `sysctl`).
- Add root certificates.
- Configure trust for private image registries, etc.

NodeGroupConfiguration is applied to new nodes at creation time, including CloudEphemeral nodes.
  
{% alert level="info" %}  
NodeGroupConfiguration applies only to nodes with a specified operating system image (`bundle`).  
You can set the `bundle` value to a specific name (e.g., `ubuntu-lts`, `centos-7`, `rocky-linux`) or use `*` to apply the configuration to all OS images.  
{% endalert %}

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

- This example is for Ubuntu OS.  
  The method for adding certificates to the trust store may differ depending on the OS.  
  When adapting the script to another OS, adjust the `bundles` and `content` fields accordingly.
- To use the certificate in `containerd`, you must restart the `containerd` service after adding the certificate.

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
    
    update-certs() {          # Function with commands for adding a certificate to the store
      update-ca-certificates
    }

    # bb-tmp-file - Creating temp file function. More information: http://www.bashbooster.net/#tmp
    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    # bb-sync-file                                - File synchronization function. More information: http://www.bashbooster.net/#sync
    ## "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"    - Destination file
    ##  ${CERT_TMP_FILE}                          - Source file
    ##  ca-file-updated                           - Name of event that will be called if the file changes.

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

### Kernel update on nodes

#### Debian-based distros

Create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource by specifying the target kernel version in the `desired_version` variable of the shell script (the resource's `spec.content` parameter):

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

Create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource by specifying the target kernel version in the `desired_version` variable of the shell script (the resource's `spec.content` parameter):

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

## Adding CloudPermanent nodes to a cloud cluster

To add CloudPermanent nodes to a DKP cloud cluster:

1. Make sure the cloud provider module is enabled. For example: [`cloud-provider-yandex`](/modules/cloud-provider-yandex/), [`cloud-provider-openstack`](/modules/cloud-provider-openstack/), [`cloud-provider-aws`](/modules/cloud-provider-aws/), etc.

   You can check this by running the following command:

   ```shell
   d8 k -n d8-system get modules
   ```

   Or view it in the Deckhouse web interface.

1. Create a [NodeGroup](/modules/node-manager/cr.html#nodegroup) object with the `CloudPermanent` type. Nodes of this type are managed via Terraform, which is integrated into DKP. The configuration for such nodes is located in the `(Provider)ClusterConfiguration` object. You edit this configuration using the `dhctl` utility from the installation container. Example:

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
   - `flavorName`: Instance type (resources: CPU, RAM).
   - `imageName`: OS image.
   - `rootDiskSize`: Size of the root disk (in GB).
   - `mainNetwork`: Network name.
   - If needed: etcd disk, zones, volume types, etc.

   For other cloud providers, the field names and structure may differ. For specific fields, refer to the CRD definition or the documentation for the corresponding cloud provider.

1. Apply the configuration using `dhctl converge`. After editing the `(Provider)ClusterConfiguration`, run:

   ```shell
   dhctl converge \
     --ssh-host <master node IP> \
     --ssh-user <username> \
     --ssh-agent-private-keys /tmp/.ssh/<key>
   ```

   This command will:

   - Launch Terraform.
   - Create the required virtual machines.
   - Install DKP on them (using `bootstrap.sh`).
   - Register the nodes in the cluster.

1. Done — the new nodes will automatically appear in the cluster.  
   You can view them by running:

   ```shell
   d8 k get nodes
   ```

   The list of newly created nodes is also available in the Deckhouse web interface.

Deckhouse Kubernetes Platform can run on top of Managed Kubernetes services (e.g., GKE and EKS).  
In such cases, the [`node-manager`](/modules/node-manager/) module provides node configuration management and automation,  
but its capabilities may be limited by the respective cloud provider's API.

## Adding a CloudStatic node to a cluster

Adding a static node can be done manually or using the Cluster API Provider Static (CAPS).

### Manually

Follow the steps below to add a new static node (e.g., VM or bare metal server) to the cluster:

1. For [CloudStatic nodes](/modules/node-manager/cr.html#nodegroup) in the following cloud providers, refer to the steps outlined in the documentation:
   - [For AWS](/modules/cloud-provider-aws/faq.html#adding-cloudstatic-nodes-to-a-cluster)
   - [For GCP](/modules/cloud-provider-gcp/faq.html#adding-cloudstatic-nodes-to-a-cluster)
   - [For YC](/modules/cloud-provider-yandex/faq.html#adding-cloudstatic-nodes-to-a-cluster)
1. Use the existing [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource or create a new one. The `nodeType` parameter for static nodes in the NodeGroup must be set to `Static` or `CloudStatic`.
1. Get the Base64-encoded script code to add and configure the node.

   Here is how you can get Base64-encoded script code to add a node to the `worker` NodeGroup:

   ```shell
   NODE_GROUP=worker
   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} -o json | jq '.data."bootstrap.sh"' -r
   ```

1. Pre-configure the new node according to the specifics of your environment. For example:
   - Add all the necessary mount points to the `/etc/fstab` file (NFS, Ceph, etc.).
   - Install the necessary packages.
   - Configure network connectivity between the new node and the other nodes of the cluster.
1. Connect to the new node over SSH and run the following command, inserting the Base64 string you got in step 3:

   ```shell
   echo <Base64-CODE> | base64 -d | bash
   ```

### Using the Cluster API Provider Static

A brief example of adding a static node to a cluster using Cluster API Provider Static (CAPS):

1. Prepare the necessary resources.

   * Allocate a server (or a virtual machine), configure networking connectivity, etc. If required, install specific OS packages and add the mount points on the node.

   * Create a user (`caps` in the example below) capable of executing `sudo` by running the following command **on the server**:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   * Allow the user to run `sudo` commands without entering a password. For this, add the following line to the `sudo` configuration **on the server** (you can either edit the `/etc/sudoers` file or run the `sudo visudo` command, or use some other method):

     ```text
     caps ALL=(ALL) NOPASSWD: ALL
     ```

   * Generate a pair of SSH keys with an empty passphrase **on the server**:

     ```shell
     ssh-keygen -t rsa -f caps-id -C "" -N ""
     ```

     The public and private keys of the `caps` user will be stored in the `caps-id.pub` and `caps-id` files in the current directory on the server.

   * Add the generated public key to the `/home/caps/.ssh/authorized_keys` file of the `caps` user by executing the following commands in the keys directory **on the server**:

     ```shell
     mkdir -p /home/caps/.ssh 
     cat caps-id.pub >> /home/caps/.ssh/authorized_keys 
     chmod 700 /home/caps/.ssh 
     chmod 600 /home/caps/.ssh/authorized_keys
     chown -R caps:caps /home/caps/
     ```

1. Create an [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) resource in the cluster:

   Run the following command in the user key directory **on the server** to encode the private key to Base64:

   ```shell
   base64 -w0 caps-id
   ```

   On any computer with `d8 k` configured to manage the cluster, create an environment variable with the value of the Base64-encoded private key you generated in the previous step:

   ```shell
    CAPS_PRIVATE_KEY_BASE64=<BASE64-ENCODED PRIVATE KEY>
   ```

   To create an [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) resource in the cluster (note that from this point on, you have to use `d8 k` configured to manage the cluster), run the following command:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: SSHCredentials
   metadata:
     name: credentials
   spec:
     user: caps
     privateSSHKey: "${CAPS_PRIVATE_KEY_BASE64}"
   EOF
   ```

1. Create a [StaticInstance](cr.html#staticinstance) resource in the cluster and specify the IP address of the static node server:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-worker-1
     labels:
       role: worker
   spec:
     # Specify the IP address of the static node server.
     address: "<SERVER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

1. Create a [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource in the cluster. The value of `count` defines a number of `staticInstances` which fall under the `labelSelector` that will be added to the cluster. In this example it's `1`:

   > The `labelSelector` field in the NodeGroup resource is immutable. To update the `labelSelector`, you need to create a new NodeGroup and move the static nodes into it by changing their labels.

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: worker
   EOF
   ```

### Using Cluster API Provider Static for multiple node groups

This example shows how you can use filters in the [StaticInstance](/modules/node-manager/cr.html#staticinstance) `label selector` to group static nodes and use them in different NodeGroups. Here, two node groups (`front` and `worker`) are used for different tasks. Each group includes nodes with different characteristics — the `front` group has two servers and the `worker` group has one.

1. Prepare the required resources (3 servers or virtual machines) and create the [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) resource the same way as on step 1 and step 2 [of the example](#using-the-cluster-api-provider-static).

1. Create two [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources in the cluster (from this point on, use `d8 k` configured to manage the cluster):

   > The `labelSelector` field in the `NodeGroup` resource is immutable. To update the `labelSelector`, you need to create a new NodeGroup and move the static nodes into it by changing their labels.

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 2
       labelSelector:
         matchLabels:
           role: front
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: worker
   EOF
   ```

1. Create [StaticInstance](/modules/node-manager/cr.html#staticinstance) resources in the cluster and specify the valid IP addresses of the servers:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-1
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP1>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-2
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP2>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-worker-1
     labels:
       role: worker
   spec:
     address: "<SERVER-WORKER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

## Adding master nodes in a cloud cluster

To add master nodes in a cloud cluster:

1. Make sure the [`control-plane-manager`](/modules/control-plane-manager/) module is enabled.

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

   ```shell
   dhctl converge \
     --ssh-host <master node IP> \
     --ssh-user <username> \
     --ssh-agent-private-keys /tmp/.ssh/<key>
   ```

## Using NodeGroup with priority

The `priority` parameter of the [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resource allows you to define the order in which nodes are provisioned in the cluster.  
For example, you can configure the cluster to first provision *spot-node* instances and fall back to regular nodes if spot capacity runs out.  
Or you can instruct the system to prioritize larger nodes when available, and use smaller ones when resources are limited.

Example of creating two NodeGroups using spot-node instances:

```yaml
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-spot
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker-spot
    maxPerZone: 5
    minPerZone: 0
    priority: 50
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker
    maxPerZone: 5
    minPerZone: 0
    priority: 30
  nodeType: CloudEphemeral
```

In the example above, the `Cluster Autoscaler` will first attempt to provision a spot-node.
If it fails to add the node to the cluster within 15 minutes, the `worker-spot` NodeGroup will be paused for 20 minutes, and the `Cluster Autoscaler` will start provisioning nodes from the `worker` NodeGroup.

If a new node is needed again after 30 minutes, `Cluster Autoscaler` will again attempt `worker-spot` first, then fall back to `worker`.

Once the `worker-spot` NodeGroup reaches its maximum (5 nodes in the example), all further nodes will be provisioned from the `worker` NodeGroup.

The node templates (labels/taints) for `worker` and `worker-spot` NodeGroups should be identical or at least compatible with the workload that triggers scaling.

## NodeGroup states and their interpretation

**Ready** — the NodeGroup contains the minimum required number of scheduled nodes with the status `Ready` for all zones.

Example 1. A group of nodes in the `Ready` state:

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

Example 2. A group of nodes in the `Not Ready` state:

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

**Updating**: A node group contains at least one node in which there is an annotation with the prefix `update.node.deckhouse.io` (for example, `update.node.deckhouse.io/waiting-for-approval`).

**WaitingForDisruptiveApproval**: A node group contains at least one node that has an annotation `update.node.deckhouse.io/disruption-required` and
there is no annotation `update.node.deckhouse.io/disruption-approved`.

**Scaling**: Calculated only for node groups with the type `CloudEphemeral`. The state `True` can be in two cases:

1. When the number of nodes is less than the *target number of nodes* in the group, i.e. when it is necessary to increase the number of nodes in the group.
1. When a node is marked for deletion or the number of nodes is greater than the *target number of nodes*, i.e. when it is necessary to reduce the number of nodes in the group.

The *target number of nodes* is the sum of all replicas in the node group.

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

**Error**: Contains the last error that occurred when creating a node in a node group.
