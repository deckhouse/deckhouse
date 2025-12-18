---
title: "Managing nodes: examples"
description: Examples of managing Kubernetes cluster nodes. Examples of creating a node group. Examples of automating the execution of arbitrary settings on a node.
---

Below are some examples of NodeGroup description, as well as installing the cert-manager plugin for `kubectl` and setting the `sysctl` parameter.

## Examples of the `NodeGroup` configuration

<span id='an-example-of-the-nodegroup-configuration'></span>

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

### Static nodes

<span id='an-example-of-the-static-nodegroup-configuration'></span>

Use `nodeType: Static` for physical servers and VMs on Hypervisors.

An example:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

Adding nodes to such a group is done [manually](#manually) using the pre-made scripts.

You can also use a method that [adds static nodes using the Cluster API Provider Static](#using-the-cluster-api-provider-static).

### System nodes

<span id='an-example-of-the-static-nodegroup-for-system-nodes-configuration'></span>

Below is an example of a system node group manifest.

When describing a NodeGroup with Static nodes, specify the value `Static` in the `nodeType` field and use the [`staticInstances`](./cr.html#nodegroup-v1-spec-staticinstances) field to describe the parameters for provisioning static machines to the cluster.

When describing a NodeGroup with CloudEphemeral type cloud nodes, specify the value `CloudEphemeral` in the `nodeType` field and use the [`cloudInstances`](./cr.html#nodegroup-v1-spec-cloudinstances) field to describe the parameters for provisioning the cloud-based VMs.

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
  # Example for Static nodes.
  nodeType: Static
  staticInstances:
    count: 2
    labelSelector:
      matchLabels:
        role: system
  # Example for CloudEphemeral nodes.
  # nodeType: CloudEphemeral
  # cloudInstances:
  #   classReference:
  #     kind: YandexInstanceClass
  #     name: large
  #   maxPerZone: 2
  #   minPerZone: 1
  #   zones:
  #   - ru-central1-d
```

### Nodes with GPU

{% alert level="info" %}
GPU-node management is available in the Enterprise edition only.
{% endalert %}

GPU nodes require the **NVIDIA driver** and the **NVIDIA Container Toolkit**. There are two ways to install the driver:

1. **Manual installation** — the administrator installs the driver before the node joins the cluster.
1. **Automation via `NodeGroupConfiguration`** (more details in the
   [Step-by-step procedure for adding a GPU node to the cluster](/modules/node-manager/faq.html#step-by-step-procedure-for-adding-a-gpu-node-to-the-cluster) section).

After the driver is detected and the NodeGroup includes the `spec.gpu` section,
`node-manager` enables full GPU support by deploying **NFD**, **GFD**, **NVIDIA Device
Plugin**, **DCGM Exporter**, and, if required, **MIG Manager**.

{% alert level="info" %}
GPU nodes are usually tainted (e.g. `node-role=gpu:NoSchedule`) so that
regular workloads don’t land there by accident. A workload that needs a GPU just adds the matching `tolerations`
and `nodeSelector`.
{% endalert %}

See the full field reference in the
[NodeGroup CR documentation](/modules/node-manager/cr.html#nodegroup-v1-spec-gpu).

Below are examples of NodeGroup manifests for typical GPU operating modes (Exclusive,
TimeSlicing, MIG).

#### Exclusive mode (one Pod — one GPU)

Each Pod gets an entire physical GPU; the cluster exposes the `nvidia.com/gpu` resource.

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: gpu-exclusive
spec:
  nodeType: Static
  gpu:
    sharing: Exclusive
  nodeTemplate:
    labels:
      node-role/gpu: ""
    taints:
    - key: node-role
      value: gpu
      effect: NoSchedule
```

#### Time-slicing (4 partitions)

The GPU is time-sliced: up to four Pods can share one card sequentially.
Suitable for experiments, CI, and light inference workloads.

Pods still request the `nvidia.com/gpu` resource.

```yaml
spec:
  gpu:
    sharing: TimeSlicing
    timeSlicing:
      partitionCount: 4
```

#### MIG (`all-1g.5gb` profile)

A hardware-partitioned GPU (A100, A30, etc.) is split into independent
instances. The scheduler exposes resources like `nvidia.com/mig-1g.5gb`.

For a complete list of supported GPUs and their profiles you can see it by using
[instruction](/modules/node-manager/faq.html#how-to-view-available-mig-profiles-in-a-cluster).

```yaml
spec:
  gpu:
    sharing: MIG
    mig:
      partedConfig: all-1g.5gb
```

#### Smoke-test Job (CUDA **vectoradd**)

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: cuda-vectoradd
spec:
  template:
    spec:
      restartPolicy: OnFailure
      nodeSelector:
        node-role/gpu: ""
      tolerations:
      - key: node-role
        value: gpu
        effect: NoSchedule
      containers:
      - name: cuda-vectoradd
        image: nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda11.7.1-ubuntu20.04
        resources:
          limits:
            nvidia.com/gpu: 1
```

This Job runs NVIDIA’s **vectoradd** CUDA sample.
If the Pod finishes with `Succeeded`, the GPU is present and configured correctly.

## Adding a static node to a cluster

<span id='an-example-of-the-static-nodegroup-configuration'></span>

Adding a static node can be done manually or using the Cluster API Provider Static.

### Manually

Follow the steps below to add a new static node (e.g., VM or bare metal server) to the cluster:

1. For [CloudStatic nodes](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype) in the following cloud providers, refer to the steps outlined in the documentation:
   - [For AWS](/modules/cloud-provider-aws/faq.html#adding-cloudstatic-nodes-to-a-cluster)
   - [For GCP](/modules/cloud-provider-gcp/faq.html#adding-cloudstatic-nodes-to-a-cluster)
   - [For YC](/modules/cloud-provider-yandex/faq.html#adding-cloudstatic-nodes-to-a-cluster)
1. Use the existing one or create a new [NodeGroup](cr.html#nodegroup) custom resource (see the [example](#static-nodes) for the `NodeGroup` called `worker`). The [nodeType](cr.html#nodegroup-v1-spec-nodetype) parameter for static nodes in the NodeGroup must be `Static` or `CloudStatic`.
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

{% alert level="warning" %}
If you have previously increased the number of master nodes in the cluster in the NodeGroup `master` (parameter [`spec.staticInstances.count`](/modules/node-manager/cr.html#nodegroup-v1-spec-staticinstances-count)), before adding regular nodes using CAPS, [make sure](/modules/control-plane-manager/faq.html#how-do-i-add-a-master-node-to-a-static-or-hybrid-cluster) that they will not be "captured".
{% endalert %}

A brief example of adding a static node to a cluster using [Cluster API Provider Static (CAPS)](./#cluster-api-provider-static):

1. Prepare the necessary resources.

   * Allocate a server (or a virtual machine), configure networking, etc. If required, install specific OS packages and add the mount points on the node.

   * Create a user (`caps` in the example below) and add it to sudoers by running the following command **on the server**:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   * Allow the user to run sudo commands without having to enter a password. For this, add the following line to the sudo configuration **on the server** (you can either edit the `/etc/sudoers` file, or run the `sudo visudo` command, or use some other method):

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

1. Create a [SSHCredentials](cr.html#sshcredentials) resource in the cluster:

   Run the following command in the user key directory **on the server** to encode the private key to Base64:

   ```shell
   base64 -w0 caps-id
   ```

   On any computer with `kubectl` configured to manage the cluster, create an environment variable with the value of the Base64-encoded private key you generated in the previous step:

   ```shell
    CAPS_PRIVATE_KEY_BASE64=<BASE64-ENCODED PRIVATE KEY>
   ```

   Create a `SSHCredentials` resource in the cluster (note that from this point on, you have to use `kubectl` configured to manage the cluster):

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

1. Create a [StaticInstance](cr.html#staticinstance) resource in the cluster; specify the IP address of the static node server:

   ```shell
   d8 k create -f - <<EOF
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

1. Create a [NodeGroup](cr.html#nodegroup) resource in the cluster. Value of `count` defines number of `staticInstances`  which fall under the `labelSelector` that will be bootstrapped and joined into the `nodeGroup`, in this example this is `1`:

   > The `labelSelector` field in the `NodeGroup` resource is immutable. To update the `labelSelector`, you need to create a new `NodeGroup` and move the static nodes into it by changing their labels.

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

   > If it is necessary to add nodes to an existing node group, specify the desired number in the `.spec.count` field of the NodeGroup.

### Using Cluster API Provider Static for multiple node groups

This example shows how you can use filters in the StaticInstance [label selector](cr.html#nodegroup-v1-spec-staticinstances-labelselector) to group static nodes and use them in different NodeGroups. Here, two node groups (`front` and `worker`) are used for different tasks. Each group includes nodes with different characteristics — the `front` group has two servers and the `worker` group has one.

1. Prepare the required resources (3 servers or virtual machines) and create the `SSHCredentials` resource in the same way as step 1 and step 2 [of the example](#using-the-cluster-api-provider-static).

1. Create two [NodeGroup](cr.html#nodegroup) in the cluster (from this point on, use `kubectl` configured to manage the cluster):

   > The `labelSelector` field in the `NodeGroup` resource is immutable. To update the `labelSelector`, you need to create a new `NodeGroup` and move the static nodes into it by changing their labels.

   ```shell
   d8 k create -f - <<EOF
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

1. Create [StaticInstance](cr.html#staticinstance) resources in the cluster and specify the valid IP addresses of the servers:

   ```shell
   d8 k create -f - <<EOF
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

### Cluster API Provider Static: Moving Instances Between Node Groups

{% alert level="warning" %}
During the process of transferring instances between node groups, the instance will be cleaned and re-bootstrapped, and the `Node` object will be recreated.
{% endalert %}

This section describes the process of moving static instances between different node groups (NodeGroup) using the Cluster API Provider Static (CAPS). The process involves modifying the NodeGroup configuration and updating the labels of the corresponding StaticInstance.

#### Initial Configuration

Assume that there is already a NodeGroup named `worker` in the cluster, configured to manage one static instance with the label `role: worker`.

**`NodeGroup` worker:**

```yaml
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
```

**`StaticInstance` static-worker-1:**

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: static-worker-1
  labels:
    role: worker
spec:
  address: "192.168.1.100"
  credentialsRef:
    kind: SSHCredentials
    name: credentials
```

#### Steps to Move an Instance Between Node Groups

##### 1. Create a New `NodeGroup` for the Target Node Group

Create a new NodeGroup resource, for example, named `front`, which will manage a static instance with the label `role: front`.

```shell
d8 k create -f - <<EOF
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

##### 2. Update the Label on the `StaticInstance`

Change the `role` label of the existing StaticInstance from `worker` to `front`. This will allow the new NodeGroup `front` to manage this instance.

```shell
d8 k label staticinstance static-worker-1 role=front --overwrite
```

##### 3. Decrease the Number of Static Instances in the Original `NodeGroup`

Update the NodeGroup resource `worker` by reducing the `count` parameter from `1` to `0`.

```shell
d8 k patch nodegroup worker -p '{"spec": {"staticInstances": {"count": 0}}}' --type=merge
```

## An example of the `NodeUser` configuration

```yaml
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: testuser
spec:
  uid: 1100
  sshPublicKeys:
    - "<SSH_PUBLIC_KEY>"
  passwordHash: <PASSWORD_HASH>
  isSudoer: true
```

## An example of the `NodeGroupConfiguration` configuration

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

### Tuning sysctl parameters

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
Example is given for Ubuntu OS.  
The method of adding certificates to the store may differ depending on the OS.  

Change the [bundles](cr.html#nodegroupconfiguration-v1alpha1-spec-bundles) and [content](cr.html#nodegroupconfiguration-v1alpha1-spec-content) parameters to adapt the script to a different OS.
{% endalert %}

{% alert level="warning" %}
To use the certificate in `containerd` (including pulling containers from a private repository), a restart of the service is required after adding the certificate.
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

### Adding the ability to download images from insecure container registry to containerd

The ability to download images from an insecure container registry is enabled using the `insecure_skip_verify` parameter in the containerd configuration file. For more information, see the [How to add configuration for an additional registry](faq.html#how-to-add-configuration-for-an-additional-registry).
