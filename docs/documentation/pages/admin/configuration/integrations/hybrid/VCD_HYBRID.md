---
title: Hybrid cluster with VCD
permalink: en/admin/integrations/hybrid/vcd-hybrid.html
---

The following describes the process of adding worker nodes from VMware Cloud Director (VCD) to an existing static DKP cluster.

Integration with VCD uses the [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) module. It provides interaction between DKP and VMware Cloud Director, creation and deletion of virtual machines, retrieval of information about the VCD infrastructure, and integration with StorageClass and other provider capabilities.

This section describes three ways to add worker nodes:

- **Automatic node creation in VCD**. DKP creates virtual machines through the VCD API. VM parameters are defined by the [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) resource, and the required number of nodes is defined by the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) type.
- **Connecting manually created nodes through CAPS**. A virtual machine is created by the user in advance, and DKP connects to it over SSH through Cluster API Provider Static. This uses the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the `Static` type, as well as the [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) and [StaticInstance](/modules/node-manager/cr.html#staticinstance) resources.
- **Connecting manually created nodes through a bootstrap script**. A virtual machine is created by the user in advance and connected to the cluster using the DKP bootstrap script. This scenario uses the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html) type.

## Prerequisites for VCD

Before you begin, make sure that the following conditions are met:

- The cluster was created with the [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype) parameter.
- Network connectivity is configured between the network of static nodes and the network of virtual machines in VCD.
- VCD nodes added to the cluster have access to the Kubernetes API, DNS, and the required addresses according to the [Network interaction](../../../../reference/network_interaction.html) and [Network policy configuration](../../configuration/network/policy/configuration.html) sections.
- The requirements from the [Connection and authorization in VMware vCloud Director](../virtualization/vcd/connection-and-authorization.html) section are met:
  - a tenant with allocated resources is configured in VCD
  - a VCD account with a static password and administrator permissions is prepared
  - a working network with an enabled DHCP server is configured in VCD
  - the required VCD resources are prepared: VDC, vApp, templates, policies, and other parameters
- When using Cilium with pod traffic tunneling, the [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) mode is selected according to the network connectivity between sites.

## Adding automatically created nodes

1. Create a file, for example `cloud-provider-vcd-mc.yaml`, with a ModuleConfig resource:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-vcd
   spec:
     version: 1
     enabled: true
     settings:
       mainNetwork: <NETWORK_NAME>
       organization: <ORGANIZATION>
       virtualDataCenter: <VDC_NAME>
       virtualApplicationName: <VAPP_NAME>
       sshPublicKey: <SSH_PUBLIC_KEY>
       provider:
         server: <API_URL>
         username: <USER_NAME>
         password: <PASSWORD>
         insecure: false
   ```

   Where:

   - `mainNetwork` — the name of the network where cloud nodes will be placed in VCD
   - `organization` — the Organization name in VCD
   - `virtualDataCenter` — the Virtual Data Center name in VCD
   - `virtualApplicationName` — the name of the vApp where nodes will be created, for example `dkp-vcd-app`
   - `sshPublicKey` — the public SSH key for accessing the nodes
   - `provider.server` — the VCD API URL
   - `provider.username` — the VCD username
   - `provider.password` — the VCD user password
   - `provider.insecure` — set to `true` if VCD uses a self-signed TLS certificate

   If a token is used for authentication, specify `apiToken` instead of `username` and `password`:

   ```yaml
   provider:
     server: <API_URL>
     apiToken: <API_TOKEN>
     username: ""
     password: ""
     insecure: false
   ```

1. Apply the ModuleConfig:

   ```shell
   d8 k apply -f cloud-provider-vcd-mc.yaml
   d8 k get mc cloud-provider-vcd
   ```

1. Make sure that all pods in the `d8-cloud-provider-vcd` namespace are in the `Running` state:

   ```shell
   d8 k get pods -n d8-cloud-provider-vcd
   ```

1. Make sure that StorageClasses for VCD have been created in the cluster:

   ```shell
   d8 k get sc
   ```

1. Create a file, for example `vcd-instanceclass-nodegroup.yaml`, with the [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) and [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources:

   ```yaml
   ---
   apiVersion: deckhouse.io/v1
   kind: VCDInstanceClass
   metadata:
     name: worker
   spec:
     rootDiskSizeGb: 50
     sizingPolicy: <SIZING_POLICY>
     storageProfile: <STORAGE_PROFILE>
     template: <VAPP_TEMPLATE>
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: VCDInstanceClass
         name: worker
       maxPerZone: 2
       minPerZone: 1
     nodeTemplate:
       labels:
         node-role/worker: ""
   ```

1. Apply the manifest:

   ```shell
   d8 k apply -f vcd-instanceclass-nodegroup.yaml
   ```

   After the manifest is applied, DKP will start creating virtual machines in VCD managed by the `node-manager` module.

1. Make sure that the required number of nodes has appeared in the cluster:

   ```shell
   d8 k get nodes -o wide
   ```

1. If VM creation fails, check the Machine and MachineSet objects and the machine-controller-manager logs:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

## Adding manually created nodes through CAPS

Before you begin, make sure that the following conditions are met:

- The [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) module is enabled and configured.
- The `cloud-provider-vcd` module components are in the `Running` state:

  ```shell
  d8 k -n d8-cloud-provider-vcd get pods -o wide
  ```

- StorageClasses for VCD have been created in the cluster:
  
  ```shell
  d8 k get sc
  ```

- A virtual machine that will be connected to the cluster has been created in VCD.
- The virtual machine name in VCD matches the hostname inside the operating system.
- The following value is set in the VM advanced parameters in VCD:

  ```text
  disk.EnableUUID = 1
  ```

- The virtual machine is connected to the VCD network used as the main network for the cluster's cloud nodes. Usually, this is the network specified in the [`mainNetwork`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-mainnetwork) parameter of the `cloud-provider-vcd` configuration or in the VCDInstanceClass being used.
- The virtual machine has administrative SSH access for initial configuration of the user that CAPS will use to connect to the node, or such a user has already been created in advance.
- The SSH user can run commands through `sudo` without entering a password.
- The virtual machine has the required base packages installed for the supported OS. For RED OS, install `which` and the package manager in advance if they are missing.

1. On the master node, set the variables for the NodeGroup being created and the virtual machine being connected:

   ```shell
   export NODE_GROUP="vcd-caps"
   export NODE_NAME="vcd-worker-caps"
   export NODE_SSH_IP="<NODE_IP>"
   export CAPS_USER="caps"
   ```

   Where:

   - `NODE_GROUP` — the name of the NodeGroup to which the node will be added;
   - `NODE_NAME` — the name of the node being connected. It must match the hostname inside the operating system and the VM name in VCD;
   - `NODE_SSH_IP` — the IP address of the virtual machine available over SSH;
   - `CAPS_USER` — the user that CAPS will use to connect to the virtual machine.

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

   ```console
   NAME       TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   vcd-caps   Static   0       0       0                                                               1m    True
   ```

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

   - `metadata.name` — the name of the node being connected
   - `metadata.labels.role` — the label by which NodeGroup selects this StaticInstance
   - `spec.address` — the IP address of the virtual machine available over SSH
   - `spec.credentialsRef.name` — the name of the SSHCredentials resource created earlier

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

   ```console
   NAME             STATUS   ROLES      AGE   VERSION    INTERNAL-IP      EXTERNAL-IP
   static-master-0  Ready    master     1h    v1.33.10   192.168.240.138  <none>
   vcd-worker-caps  Ready    vcd-caps   5m    v1.33.10   192.168.240.151  <none>
   ```

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

- The [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) module is enabled and configured.
- The `cloud-provider-vcd` module components are in the `Running` state:

  ```shell
  d8 k -n d8-cloud-provider-vcd get pods -o wide
  ```

- StorageClasses for VCD have been created in the cluster:
  
  ```shell
  d8 k get sc
  ```

- A virtual machine that will be connected to the cluster has been created in VCD.
- The virtual machine name in VCD matches the hostname inside the operating system.
- The following value is set in the VM advanced parameters in VCD:

  ```text
  disk.EnableUUID = 1
  ```

- The virtual machine is connected to the VCD network used as the main network for the cluster's cloud nodes. Usually, this is the network specified in the [`mainNetwork`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-mainnetwork) parameter of the `cloud-provider-vcd` configuration or in the VCDInstanceClass being used.
- The virtual machine has the required base packages installed for the supported OS. For RED OS, install `which` and the package manager in advance if they are missing.

1. Create a file, for example `cloud-static-nodegroup.yaml`, with a NodeGroup resource and the CloudStatic node type:

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

   ```console
   NAME           TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   cloud-static   CloudStatic   0       0       0                                                               1m    True
   ```

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

   ```console
   NAME                       STATUS   ROLES          AGE   VERSION    INTERNAL-IP
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.138
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.151
   ```
