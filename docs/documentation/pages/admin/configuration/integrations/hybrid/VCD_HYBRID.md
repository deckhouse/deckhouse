---
title: Hybrid cluster with VCD
permalink: en/admin/integrations/hybrid/vcd-hybrid.html
search: hybrid with VCD
description: Preparation for hybrid integration with VMware Cloud Director in Deckhouse Kubernetes Platform.
---

The following describes the process of adding worker nodes from VMware Cloud Director (VCD) to an existing static Deckhouse Kubernetes Platform (DKP) cluster.

Integration with VCD uses the [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) module. It provides interaction between DKP and VMware Cloud Director, creation and deletion of virtual machines, retrieval of information about the VCD infrastructure, and integration with StorageClass and other provider capabilities.

This section describes two ways to add worker nodes:

- **Automatic node creation in VCD**. DKP creates virtual machines through the VCD API. VM parameters are defined by the [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) resource, and the required number of nodes is defined by the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) type.
- **Connecting manually created nodes through a bootstrap script**. A virtual machine is created by the user in advance and connected to the cluster using the DKP bootstrap script. This scenario uses the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html) type.

## Prerequisites for VCD

Before you begin, make sure that the following conditions are met:

- The cluster was created with the [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype) parameter.
- [Network connectivity](./overview.html#general-network-requirements) is configured between the network of static nodes and the network of virtual machines in VCD.
- VCD nodes added to the cluster have access to the Kubernetes API, DNS, and the required addresses according to the [Network interaction](../../../../reference/network_interaction.html) and [Network policy configuration](../../configuration/network/policy/configuration.html) sections.
- The requirements from the [Connection and authorization in VMware vCloud Director](../virtualization/vcd/connection-and-authorization.html) section are met:
  - A tenant with allocated resources is configured in VCD.
  - A VCD account with a static password and administrator permissions is prepared.
  - A working network with an enabled DHCP server is configured in VCD.
  - The required VCD resources are prepared: VDC, vApp, templates, policies, and other parameters.
- When using Cilium with pod traffic tunneling, the [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) mode is selected according to the network connectivity between sites.

## Adding automatically created nodes

1. Create a file with a ModuleConfig resource. For example, `cloud-provider-vcd-mc.yaml`:

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

   - `mainNetwork`: Name of the network where cloud nodes will be placed in VCD.
   - `organization`: Organization name in VCD.
   - `virtualDataCenter`: Virtual Data Center name in VCD.
   - `virtualApplicationName`: Name of the vApp where nodes will be created, for example `dkp-vcd-app`.
   - `sshPublicKey`: Public SSH key for accessing the nodes.
   - `provider.server`: VCD API URL.
   - `provider.username`: VCD username.
   - `provider.password`: VCD user password.
   - `provider.insecure`: Set to `true` if VCD uses a self-signed TLS certificate.

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

1. Create a file with the [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) and [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources. For example, `vcd-instanceclass-nodegroup.yaml`:

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

## Adding manually created nodes through a bootstrap script

Before you begin, make sure that the following conditions are met:

- The [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) module is enabled and configured:

  ```shell
  d8 k get moduleconfig cloud-provider-vcd
  d8 k get module cloud-provider-vcd -o wide
  ```

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
- One of the package managers (`apt`/`apt-get`, `yum`, or `rpm`) for a supported OS is installed on the virtual machine.

1. Create a file with a NodeGroup resource and the CloudStatic node type. For example, `cloud-static-nodegroup.yaml`:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: cloud-static
   spec:
     nodeType: CloudStatic
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
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.138
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.151
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->
