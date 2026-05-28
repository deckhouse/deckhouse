---
title: Hybrid cluster with vSphere
permalink: en/admin/integrations/hybrid/vsphere-hybrid.html
search: hybrid with vSphere
description: Preparation for hybrid integration with VMware vSphere in Deckhouse Kubernetes Platform.
---

The following describes the process of adding worker nodes from vSphere to an existing static Deckhouse Kubernetes Platform (DKP) cluster.

Integration with vSphere uses the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module. It provides interaction between DKP and vCenter, retrieval of information about virtual machines, work with placement parameters, and integration with vSphere infrastructure capabilities.

This section describes three ways to add worker nodes:

- **Automatic node creation in vSphere**. DKP creates virtual machines through the vSphere API. VM parameters are defined by the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resource, and the required number of nodes and placement zones are defined by the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) type.
- **Connecting manually created nodes through CAPS**. A virtual machine is created by the user in advance, and DKP connects to it over SSH through Cluster API Provider Static. This uses NodeGroup resources with the `Static` type, as well as SSHCredentials and StaticInstance resources.
- **Connecting manually created nodes through a bootstrap script**. A virtual machine is created by the user in advance and connected to the cluster using the DKP bootstrap script. This scenario uses the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html) type.

## Prerequisites for vSphere

Before you begin, make sure that the following conditions are met:

- The cluster was created with the [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype) parameter.
- Network connectivity is configured between the network of static nodes and the network of virtual machines in vSphere.
- vSphere nodes added to the cluster have access to the Kubernetes API, DNS, and the required addresses according to the [Network interaction](../../../../reference/network_interaction.html) and [Network policy configuration](../../configuration/network/policy/configuration.html) sections.
- The requirements from the [Connection and authorization in VMware vSphere](../virtualization/vsphere/authorization.html) section are met:
  - Access to vCenter is configured.
  - The vSphere account with the required privileges is prepared.
  - A virtual machine template is prepared.
  - Networks, Datastore, region tags, and zone tags are configured.
- When using Cilium with pod traffic tunneling, the [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) mode is selected according to the network connectivity between sites.

## Adding automatically created nodes

To connect an already running static cluster to vCenter, use the [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) resource of the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module.

In the `spec.settings` parameter, specify access parameters for vCenter, network settings, region and zone tags, and SSH keys that will be added to the created virtual machines.

An example configuration and description of the available parameters are provided in the [module examples](/modules/cloud-provider-vsphere/examples.html) and in the section describing the [module settings](/modules/cloud-provider-vsphere/configuration.html).

1. Create a file with ModuleConfig for the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module. For example, `vsphere-mc.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-vsphere
   spec:
     version: 2
     enabled: true
     settings:
       host: "<VCENTER_FQDN>"
       username: "<USERNAME@DOMAIN.LOCAL>"
       password: "<PASSWORD>"
       insecure: true
       vmFolderPath: "<FOLDER_PATH_UNDER_DATACENTER>"
       regionTagCategory: "<TAG_CATEGORY_FOR_REGION>"
       zoneTagCategory: "<TAG_CATEGORY_FOR_ZONE>"
       region: "<REGION_TAG_NAME_ON_DATACENTER>"
       zones:
         - "<ZONE_TAG_NAME_ON_CLUSTER>"
       internalNetworkNames:
         - "<PORT_GROUP_NAME_FOR_INTERNAL_IP>"
       sshKeys:
         - "<SSH_PUBLIC_KEY_ONE_LINE>"
   ```

   Parameter values:

   - `host`: vCenter address.
   - `username`, `password`: vSphere user credentials.
   - `insecure`: Disables verification of the vCenter TLS certificate.
   - `vmFolderPath`: Folder where virtual machines will be created.
   - `regionTagCategory`, `zoneTagCategory`: Region and zone tag categories.
   - `region`: Region tag.
   - `zones`: List of zones where nodes can be created.
   - `internalNetworkNames`: List of vSphere networks for connecting created nodes.
   - `sshKeys`: Public SSH keys that will be added to the created virtual machines.

1. Apply the module configuration:

   ```shell
   d8 k apply -f vsphere-mc.yaml
   ```

1. Wait for the `cloud-provider-vsphere` module to become ready:

   ```shell
   d8 k get moduleconfig cloud-provider-vsphere
   d8 k get pods -n d8-cloud-provider-vsphere -o wide
   ```

1. Create a file with the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) and [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources with the `nodeType: CloudEphemeral` value. For example, `vsphere-instance.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VsphereInstanceClass
   metadata:
     name: ephemeral
   spec:
     numCPUs: 2
     memory: 4096
     rootDiskSize: 40
     template: "<PATH_TO_TEMPLATE_FROM_DATACENTER>"
     mainNetwork: "<PORT_GROUP_NAME>"
     datastore: "<DATASTORE_OR_FOLDER/DATASTORE>"
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: ephemeral
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: VsphereInstanceClass
         name: ephemeral
       maxPerZone: 1
       minPerZone: 1
       zones:
         - "<ZONE_TAG_NAME_ON_CLUSTER>"
     disruptions:
       approvalMode: Automatic
   ```

   Where:

   - [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) describes the parameters of the virtual machine that will be created in vSphere.
   - [NodeGroup](/modules/node-manager/cr.html#nodegroup) describes the node group that DKP must maintain in the cluster.
   - `nodeType: CloudEphemeral` means that nodes will be created automatically through the cloud provider.
   - `cloudInstances.classReference` points to VsphereInstanceClass.
   - `cloudInstances.zones` must contain zones from the `zones` list in ModuleConfig.

1. Apply the manifest:

   ```shell
   d8 k apply -f vsphere-instance.yaml
   ```

   After the manifest is applied, DKP will start creating a virtual machine in vSphere. After the VM boots, kubelet will connect to the Kubernetes API, and the new node will appear in the cluster.

1. Check the node status:

   ```shell
   d8 k get nodes -o wide
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                             STATUS   ROLES                  AGE   VERSION
   static-master-0                  Ready    control-plane,master   1h    v1.33.10
   ephemeral-1ca02a5b-7588b-k89dc   Ready    ephemeral              10m   v1.33.10
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. If VM creation fails, check the Machine and MachineSet objects and the machine-controller-manager logs:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

   Also check the cluster events:

   ```shell
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

## Adding manually created nodes through CAPS

Before you begin, make sure that the following conditions are met:

- The [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module is enabled and configured.
- The `cloud-provider-vsphere` module components are in the `Running` state:

  ```shell
  d8 k -n d8-cloud-provider-vsphere get pods -o wide
  ```

- StorageClasses for vSphere have been created in the cluster:
  
  ```shell
  d8 k get sc
  ```

- A virtual machine that will be connected to the cluster has been created in vSphere.
- The virtual machine name in vSphere, the local-hostname value in the metadata, and the hostname inside the operating system match.
- The following parameter is set in the VM advanced parameters in vSphere:

  ```text
  disk.EnableUUID = TRUE
  ```

- The virtual machine is connected to the network specified in the [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) parameter of the `cloud-provider-vsphere` module configuration.
- The virtual machine has administrative SSH access for initial configuration of the user that CAPS will use to connect to the node, or such a user has already been created in advance.
- The SSH user can run commands through `sudo` without entering a password.
- The virtual machine has the required base packages installed for the supported OS. For RED OS, install `which` and the package manager in advance if they are missing.

{% offtopic title="Passing metadata to a VM through vSphere..." %}
For preliminary virtual machine configuration, you can use cloud-init through vSphere metadata. For example, metadata can be used to configure the hostname, network, and SSH keys.

In this case, set the following VM advanced parameters:

```text
guestinfo.metadata = <BASE64_ENCODED_METADATA>
guestinfo.metadata.encoding = base64
```

Example `metadata.json` file:

```json
{
  "instance-id": "vsphere-worker-caps",
  "local-hostname": "vsphere-worker-caps",
  "public-keys-data": "<SSH_PUBLIC_KEY>",
  "network": {
    "version": 2,
    "ethernets": {
      "id0": {
        "match": {
          "driver": "vmxnet3"
        },
        "set-name": "ens192",
        "dhcp4": true
      }
    }
  }
}
```

Where:

- `instance-id`: Virtual machine identifier.
- `local-hostname`: Node hostname inside the operating system.
- `public-keys-data`: Public SSH key for accessing the virtual machine.
- `network`: Network settings that will be applied inside the virtual machine.

To get the value for the `guestinfo.metadata` parameter, run:

```shell
METADATA_B64="$(base64 -w0 metadata.json)"
echo "$METADATA_B64"
```

Using `guestinfo.metadata` is not a mandatory CAPS requirement. The main requirement is that by the time the StaticInstance resource is created, the virtual machine is available over SSH, has the correct hostname, and the connection user can run commands through `sudo` without a password.
{% endofftopic %}

1. On the master node, set the variables for the NodeGroup being created and the virtual machine being connected:

   ```shell
   export NODE_GROUP="vsphere-caps"
   export NODE_NAME="vsphere-worker-caps"
   export NODE_SSH_IP="<NODE_IP>"
   export CAPS_USER="caps"
   ```

   Where:

   - `NODE_GROUP`: Name of the NodeGroup to which the node will be added.
   - `NODE_NAME`: Name of the node being connected. It must match the hostname inside the operating system and the VM name in vSphere.
   - `NODE_SSH_IP`: IP address of the virtual machine available over SSH.
   - `CAPS_USER`: User that CAPS will use to connect to the virtual machine.

1. On the master node, create a NodeGroup:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: ${NODE_GROUP}
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: ${NODE_GROUP}
   EOF
   ```

   This scenario uses `nodeType: Static` because the virtual machine has already been created manually, and CAPS will only connect to it over SSH and configure it.

1. Make sure that the NodeGroup has been created and synchronized:

   ```shell
   d8 k get nodegroup ${NODE_GROUP}
   d8 k describe nodegroup ${NODE_GROUP}
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME           TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   vsphere-caps   Static   0       0       0                                                               1m    True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. On the master node, generate the SSH key that CAPS will use to connect to the virtual machine:

   ```shell
   ssh-keygen -t ed25519 \
     -f /dev/shm/${NODE_GROUP}-id \
     -C "" \
     -N ""
   ```

   {% alert level="info" %}
   The key is created with an empty passphrase because CAPS must use it automatically.
   {% endalert %}

1. On the master node, create an [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) resource:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha2
   kind: SSHCredentials
   metadata:
     name: ${NODE_GROUP}
   spec:
     user: ${CAPS_USER}
     privateSSHKey: "$(base64 -w0 /dev/shm/${NODE_GROUP}-id)"
   EOF
   ```

   The SSHCredentials resource stores the username and private SSH key that CAPS will use to connect to the virtual machine.

1. Make sure that the SSHCredentials resource has been created:

   ```shell
   d8 k get sshcredentials
   d8 k describe sshcredentials ${NODE_GROUP}
   ```

1. On the master node, print the public part of the SSH key:

   ```shell
   cat /dev/shm/${NODE_GROUP}-id.pub
   ```

   This key will be needed in the next step to configure the user on the virtual machine being connected.

1. On the virtual machine being connected, create the user that CAPS will use to configure the node. Run the commands on the virtual machine being connected, specifying the public SSH key obtained in the previous step:

   ```shell
   export CAPS_USER="caps"
   export KEY='<SSH_PUBLIC_KEY>'

   useradd -m -s /bin/bash ${CAPS_USER}
   usermod -aG sudo ${CAPS_USER}

   echo "${CAPS_USER} ALL=(ALL) NOPASSWD: ALL" | EDITOR='tee -a' visudo

   mkdir -p /home/${CAPS_USER}/.ssh
   echo "${KEY}" > /home/${CAPS_USER}/.ssh/authorized_keys

   chown -R ${CAPS_USER}:${CAPS_USER} /home/${CAPS_USER}
   chmod 700 /home/${CAPS_USER}/.ssh
   chmod 600 /home/${CAPS_USER}/.ssh/authorized_keys
   ```

   {% alert level="info" %}
   The `KEY` value must be specified in quotes because the public SSH key contains spaces.
   {% endalert %}

   {% alert level="info" %}
   For operating systems of the Astra Linux family, when using the Parsec mandatory integrity control module, additionally set the maximum integrity level for the user:

   ```shell
   pdpl-user -i 63 ${CAPS_USER}
   ```

   {% endalert %}

1. On the master node, check that the CAPS user can connect to the virtual machine over SSH and run commands through `sudo` without a password:

   ```shell
   ssh -i /dev/shm/${NODE_GROUP}-id ${CAPS_USER}@${NODE_SSH_IP} \
     'hostname; sudo -n true; echo OK'
   ```

   The output must contain the node name and the `OK` line.

1. On the master node, create a [StaticInstance](/modules/node-manager/cr.html#staticinstance) resource for the virtual machine being connected:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha2
   kind: StaticInstance
   metadata:
     name: ${NODE_NAME}
     labels:
       role: ${NODE_GROUP}
   spec:
     address: "${NODE_SSH_IP}"
     credentialsRef:
       kind: SSHCredentials
       name: ${NODE_GROUP}
   EOF
   ```

   Where:

   - `metadata.name`: Name of the node being connected.
   - `metadata.labels.role`: Label by which NodeGroup selects this StaticInstance.
   - `spec.address`: IP address of the virtual machine available over SSH.
   - `spec.credentialsRef.name`: Name of the SSHCredentials resource created earlier.

1. Check the StaticInstance status:

   ```shell
   d8 k get staticinstances
   d8 k describe staticinstance ${NODE_NAME}
   ```

1. Wait for the node to connect and check its status:

   ```shell
   d8 k get nodes -o wide
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                    STATUS   ROLES          AGE   VERSION    INTERNAL-IP      EXTERNAL-IP
   static-master-0         Ready    master         1h    v1.33.10   192.168.240.135  <none>
   vsphere-worker-caps     Ready    vsphere-caps   5m    v1.33.10   192.168.240.152  <none>
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. If connection fails, check the NodeGroup, StaticInstance, Machine status, and cluster events:

   ```shell
   d8 k get nodegroup ${NODE_GROUP}
   d8 k describe nodegroup ${NODE_GROUP}

   d8 k get staticinstances
   d8 k describe staticinstance ${NODE_NAME}

   d8 k -n d8-cloud-instance-manager get machines,machinesets,machinedeployments -o wide
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

## Adding manually created nodes through a bootstrap script

Before you begin, make sure that the following conditions are met:

- The [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module is enabled and configured.
- The `cloud-provider-vsphere` module components are in the `Running` state:

  ```shell
  d8 k -n d8-cloud-provider-vsphere get pods -o wide
  ```

- StorageClasses for vSphere have been created in the cluster:
  
  ```shell
  d8 k get sc
  ```

- A virtual machine that will be connected to the cluster has been created in vSphere.
- The virtual machine name in vSphere matches the hostname inside the operating system.
- The following parameters are set in the VM advanced parameters in vSphere:

  ```text
  disk.EnableUUID = TRUE
  guestinfo.metadata = <BASE64_ENCODED_METADATA>
  guestinfo.metadata.encoding = base64
  ```

  The `guestinfo.metadata` parameter must contain the metadata configuration encoded in Base64. Example `metadata.json` file:

  ```json
  {
     "instance-id": "cloud-static-worker-0",
     "local-hostname": "cloud-static-worker-0",
     "public-keys-data": "<SSH_PUBLIC_KEY>",
     "network": {
       "version": 2,
       "ethernets": {
         "id0": {
           "match": {
             "driver": "vmxnet3"
           },
           "set-name": "ens192",
           "dhcp4": true
         }
       }
     }
   }
  ```

  Where:

  - `instance-id`: Virtual machine identifier.
  - `local-hostname`: Node hostname inside the operating system.
  - `public-keys-data`: Public SSH key for accessing the virtual machine.
  - `network`: Network settings that will be applied inside the virtual machine.

  To get the value for the `guestinfo.metadata` parameter, run:

  ```shell
  METADATA_B64="$(base64 -w0 metadata.json)"
  echo "$METADATA_B64"
  ```

- The virtual machine is connected to the network specified in the [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) parameter of the `cloud-provider-vsphere` module configuration.
- The virtual machine has the required base packages installed for the supported OS. For RED OS, install `which` and the package manager in advance if they are missing.

1. Create a file with a NodeGroup resource and the CloudStatic node type. For example, `cloud-static-nodegroup.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: cloud-static
   spec:
     nodeType: CloudStatic
   ```

1. Apply the manifest:

   ```shell
   d8 k apply -f cloud-static-nodegroup.yaml
   ```

1. Make sure that the NodeGroup has been created and synchronized:

   ```shell
   d8 k get nodegroup cloud-static
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME           TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   cloud-static   CloudStatic   0       0       0                                                               1m    True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Get the bootstrap script for the created NodeGroup:

   ```shell
   NODE_GROUP=cloud-static

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > bootstrap.b64
   ```

1. Copy the bootstrap script to the virtual machine being connected:

   ```shell
   scp bootstrap.b64 <USER>@<NODE_IP>:/tmp/bootstrap.b64
   ```

1. Connect to the virtual machine over SSH:

   ```shell
   ssh <USER>@<NODE_IP>
   ```

1. On the virtual machine, set permissions and run the bootstrap script:

   ```shell
   base64 -d /tmp/bootstrap.b64 > /tmp/bootstrap.sh
   chmod +x /tmp/bootstrap.sh

   sudo bash /tmp/bootstrap.sh
   ```

   After the bootstrap script is started, it will install the required components, configure the container runtime and kubelet, and connect the node to the cluster.

1. On the master node, check that the new node has appeared:

   ```shell
   d8 k get nodes -o wide
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                       STATUS   ROLES          AGE   VERSION    INTERNAL-IP
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.135
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.152
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->
