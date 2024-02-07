---
title: "Managing nodes: examples"
description: Examples of managing Kubernetes cluster nodes. Examples of creating a node group. Examples of automating the execution of arbitrary settings on a node.
---

Below are some examples of `NodeGroup` description, as well as installing the cert-manager plugin for kubectl and setting the sysctl parameter.

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

## Adding a static node to a cluster

<span id='an-example-of-the-static-nodegroup-configuration'></span>

Adding a static node can be done manually or using the Cluster API Provider Static.

### Manually

Follow the steps below to add a new static node (e.g., VM or bare metal server) to the cluster:

1. For [CloudStatic nodes](../040-node-manager/cr.html#nodegroup-v1-spec-nodetype) in the following cloud providers, refer to the steps outlined in the documentation:
   - [For AWS](../030-cloud-provider-aws/faq.html#adding-cloudstatic-nodes-to-a-cluster)
   - [For GCP](../030-cloud-provider-gcp/faq.html#adding-cloudstatic-nodes-to-a-cluster)
   - [For YC](../030-cloud-provider-yandex/faq.html#adding-cloudstatic-nodes-to-a-cluster)
1. Use the existing one or create a new [NodeGroup](cr.html#nodegroup) custom resource (see the [example](#static-nodes) for the `NodeGroup` called `worker`). The [nodeType](cr.html#nodegroup-v1-spec-nodetype) parameter for static nodes in the NodeGroup must be `Static` or `CloudStatic`.
1. Get the Base64-encoded script code to add and configure the node.

   Here is how you can get Base64-encoded script code to add a node to the `worker` NodeGroup:

   ```shell
   NODE_GROUP=worker
   kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} -o json | jq '.data."bootstrap.sh"' -r
   ```

1. Pre-configure the new node according to the specifics of your environment. For example:
   - Add all the necessary mount points to the `/etc/fstab` file (NFS, Ceph, etc.);
   - Install the necessary packages (e.g., `ceph-common`);
   - Configure network connectivity between the new node and the other nodes of the cluster.
1. Connect to the new node over SSH and run the following command, inserting the Base64 string you got in step 2:

   ```shell
   echo <Base64-CODE> | base64 -d | bash
   ```

### Using the Cluster API Provider Static

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
   kubectl create -f - <<EOF
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
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
   spec:
     # Specify the IP address of the static node server.
     address: "<SERVER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

1. Create a [NodeGroup](cr.html#nodegroup) resource in the cluster:

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
   EOF
   ```

### Using the Cluster API Provider Static and label selector filters

This example shows how you can use filters in the StaticInstance [label selector](cr.html#nodegroup-v1-spec-staticinstances-labelselector) to group static nodes and use them in different NodeGroups. Here, two node groups (`front` and `worker`) are used for different tasks. Each group includes nodes with different characteristics — the `front` group has two servers and the `worker` group has one.

1. Prepare the required resources (3 servers or virtual machines) and create the `SSHCredentials` resource in the same way as step 1 and step 2 [of the example](#using-the-cluster-api-provider-static).

1. Create two [NodeGroup](cr.html#nodegroup) in the cluster (from this point on, use `kubectl` configured to manage the cluster):

   ```shell
   kubectl create -f - <<EOF
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
   kubectl create -f - <<EOF
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
