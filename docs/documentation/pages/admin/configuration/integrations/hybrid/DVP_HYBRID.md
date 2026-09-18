---
title: Hybrid cluster with virtualization
permalink: en/admin/integrations/hybrid/dvp-hybrid.html
lang: en
search: hybrid with DVP
description: Preparing for hybrid integration with virtualization in Deckhouse Platform.
---

This section describes how to add virtualization nodes to an existing static DP cluster.

Integration with virtualization uses the [`cloud-provider-dvp`](/modules/cloud-provider-dvp/) module. It enables DP to interact with the virtualization cluster API, create virtual machines, connect the created VMs to an existing Kubernetes cluster, and manage the lifecycle of nodes through Cluster API mechanisms.

This section describes two ways to add nodes:

- **Automatically creating virtualization nodes**. DP creates virtual machines through the virtualization API. VM parameters are specified using the [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) resource, while the required number of nodes and placement zones are specified using the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) type.
- **Connecting manually created nodes using a bootstrap script**. A virtual machine is created in advance by the user and connected to the cluster using a DP bootstrap script. This scenario uses a [NodeGroup](/modules/node-manager/cr.html#nodegroup) with the [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html) type.

## Prerequisites

Before you begin, make sure that the following requirements are met:

- The cluster is created with the [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype) parameter.
- Network connectivity is configured between the network of the static DP cluster nodes and the network of virtual machines in virtualization. For details, see [Network requirements](./overview.html##general-network-requirements). Nodes created in virtualization have access to the Kubernetes API of the target DP cluster, DNS, and the required addresses according to the [Network interaction](../../../../reference/network_interaction.html) and [Configuring network policies](../../configuration/network/policy/configuration.html) sections. If Cilium is used with pod traffic tunneling, the [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) mode is selected according to the network connectivity between sites. The Kubernetes API of the virtualization cluster is accessible from the DP cluster.
- The requirements from the [Preparing the environment](/modules/cloud-provider-dvp/environment.html) section are met:
  - a [ServiceAccount](/modules/cloud-provider-dvp/environment.html#creating-a-user) has been created to access the virtualization API;
  - a kubeconfig has been generated to connect to the virtualization API;
  - a namespace has been prepared where virtual machines and disks will be created.
- A Linux OS image with `cloud-init` support is available in virtualization, for example `ubuntu-24-04-lts`.
- A suitable [VirtualMachineClass](/modules/virtualization/stable/cr.html#virtualmachineclass) is available in virtualization, for example `amd-epyc-gen-3`.
- A StorageClass for root disks of virtual machines is available in virtualization, for example `replicated`.
- If a virtual machine template is used, make sure that it contains only one disk.

{% alert level="warning" %}
[DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) parameters use resources from the virtualization cluster: VirtualMachineClass, ClusterVirtualImage, VirtualImage, VirtualDisk, and StorageClass from virtualization, not from the target DP cluster.
{% endalert %}

{% alert level="warning" %}
Make sure there are no StorageClass resources in the static DP cluster whose names match the names of StorageClass resources in the virtualization cluster.

When the `cloud-provider-dvp` module is enabled, the corresponding StorageClass resources are automatically synchronized from virtualization to the DP cluster. If a StorageClass with the same name already exists in the static cluster, a resource conflict may occur, causing the module installation or upgrade to fail.
{% endalert %}

## Adding automatically created nodes

1. On the administrator's machine where access to the virtualization cluster is configured, prepare a kubeconfig for the `cloud-provider-dvp` module to access the virtualization API.

   Follow the steps in the ["Preparing the environment"](/modules/cloud-provider-dvp/environment.html) section and encode the generated kubeconfig in Base64:

   ```shell
   export DVP_PROVIDER_KUBECONFIG="./kubeconfig"
   export DVP_KUBECONFIG_B64="$(base64 -w0 ${DVP_PROVIDER_KUBECONFIG})"
   ```

1. Specify the virtualization namespace where virtual machines and disks will be created:

   ```shell
   export DVP_NAMESPACE="<DVP_NAMESPACE>"
   ```

1. Specify the virtualization zone where nodes will be created.

   Currently, zoning in virtualization is under development, so use the `default` value for the `zones` parameters in ModuleConfig and NodeGroup:

   ```shell
   export DVP_ZONE="default"
   ```

   If necessary, you can check the topology labels of nodes in the virtualization cluster:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/region,topology.kubernetes.io/zone
   ```

   {% alert level="warning" %}
   The zone value in ModuleConfig and NodeGroup must match. Currently, only the `default` value is available in DVP.
   {% endalert %}

1. Create a file with the `cloud-provider-dvp` module configuration. For example, `cloud-provider-dvp-mc.yaml`:

   ```shell
   cat > cloud-provider-dvp-mc.yaml <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-dvp
   spec:
     enabled: true
     version: 2
     settings:
       nodes:
         parameters:
           layout: Standard
           sshPublicKey: "<SSH_PUBLIC_KEY>"
           zones:
             - ${DVP_ZONE}
       provider:
         parameters:
           namespace: ${DVP_NAMESPACE}
   EOF
   ```

   The manifest uses the environment variables set in the previous steps: `DVP_NAMESPACE` and `DVP_ZONE`. Replace `<SSH_PUBLIC_KEY>` with the public SSH key for access to the created nodes.

1. Apply the manifest:

   ```shell
   d8 k apply -f cloud-provider-dvp-mc.yaml
   ```

1. Wait until the `d8-cloud-provider-dvp` namespace appears:

   ```shell
   d8 k get ns d8-cloud-provider-dvp
   ```

   The namespace is created by the `cloud-provider-dvp` module, so the credentials Secret is applied as a separate step rather than together with the ModuleConfig. Until the Secret exists, the module components cannot reach the DVP API.

1. Create the credentials Secret:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: v1
   kind: Secret
   metadata:
     name: d8-credentials
     namespace: d8-cloud-provider-dvp
   type: cloud-provider.deckhouse.io/credentials
   stringData:
     authScheme: kubeconfig
     secret: ${DVP_KUBECONFIG_B64}
   EOF
   ```

   The Secret uses the `DVP_KUBECONFIG_B64` environment variable set in the first step.

1. Wait until the `cloud-provider-dvp` module is enabled:

   ```shell
   d8 k get module cloud-provider-dvp -o wide
   ```

   The module must switch to the `Ready` state, and the pods in the `d8-cloud-provider-dvp` namespace must be in the `Running` state.

1. Make sure that the `node-manager` module is in the `Ready` state:

   ```shell
   d8 k get module node-manager -o wide
   ```

   If the module is in the `Error` state, check that available virtualization zones are specified in ModuleConfig and NodeGroup.

1. Make sure that the [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) resource is available in the cluster:

   ```shell
   d8 k get crd dvpinstanceclasses.deckhouse.io
   ```

1. On the administrator's machine where access to the virtualization cluster is configured, check the available virtual machine classes, images, and StorageClasses:

   ```shell
   d8 k --kubeconfig ${DVP_PROVIDER_KUBECONFIG} get virtualmachineclasses
   d8 k --kubeconfig ${DVP_PROVIDER_KUBECONFIG} get clustervirtualimages
   d8 k --kubeconfig ${DVP_PROVIDER_KUBECONFIG} get storageclasses
   ```

   Use the obtained values when creating DVPInstanceClass.

1. Create a file with DVPInstanceClass and NodeGroup resources. For example, `dvp-instanceclass-nodegroup.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: DVPInstanceClass
   metadata:
     name: dvp-worker
   spec:
     virtualMachine:
       cpu:
         cores: 3
         coreFraction: 20%
       memory:
         size: 6Gi
       virtualMachineClassName: <VIRTUAL_MACHINE_CLASS_NAME>
       bootloader: EFI
     rootDisk:
       size: 15Gi
       storageClass: <STORAGE_CLASS_NAME>
       image:
         kind: ClusterVirtualImage
         name: <CLUSTER_VIRTUAL_IMAGE_NAME>
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: dvp-worker
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: DVPInstanceClass
         name: dvp-worker
       minPerZone: 1
       maxPerZone: 1
       zones:
         - default
   ```

   Where:

   - `virtualMachineClassName` — name of the VirtualMachineClass in virtualization, for example `amd-epyc-gen-3`;
   - `rootDisk.storageClass` — name of the StorageClass in virtualization, for example `replicated`;
   - `rootDisk.image.kind` — image source type. For a cluster image, use `ClusterVirtualImage`;
   - `rootDisk.image.name` — name of the OS image in virtualization, for example `ubuntu-24-04-lts`;
   - `cloudInstances.zones` — virtualization zone where the node will be created. The value must match the `zones` value in ModuleConfig.

1. Apply the manifest:

   ```shell
   d8 k apply -f dvp-instanceclass-nodegroup.yaml
   ```

   After the manifest is applied, DP will start creating a virtual machine in DVP and connect it to the cluster as a node.

1. Check the NodeGroup status:

   ```shell
   d8 k get nodegroup dvp-worker -o wide
   d8 k describe nodegroup dvp-worker
   ```

1. Check that a new node appears in the DP cluster:

   ```shell
   d8 k get nodes -o wide
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                              STATUS   ROLES                  AGE   VERSION    INTERNAL-IP
   dvp-hybrid-master-0               Ready    control-plane,master   1h    v1.33.10   10.12.0.69
   dvp-worker-c75a75c1-twqp4-bjpvl   Ready    dvp-worker             10m   v1.33.10   10.12.3.15
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

## Adding manually created nodes using a bootstrap script

Before you begin, make sure that the following requirements are met:

- The [`cloud-provider-dvp`](/modules/cloud-provider-dvp/) module is enabled:

  ```shell
  d8 k get module cloud-provider-dvp -o wide
  ```

- The `cloud-provider-dvp` module components are in the `Running` state:

  ```shell
  d8 k -n d8-cloud-provider-dvp get pods -o wide
  ```

- A virtual machine that will be connected to the cluster has been created in virtualization.
- The virtual machine is connected to the virtualization network used for hybrid integration with the cluster.
- The virtual machine IP address belongs to the range specified in [`internalNetworkCIDRs`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration-internalnetworkcidrs).
- The virtual machine name in virtualization matches the hostname inside the operating system.
- SSH access is available on the virtual machine for copying and running the bootstrap script.
- The SSH user can run commands using `sudo` without entering a password.
- One of the package managers (`apt`/`apt-get`, `yum`, or `rpm`) for a supported OS is installed on the virtual machine.

1. Create a NodeGroup with the `CloudStatic` node type. In this example and the following steps, the `cloud-static` name is used:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: cloud-static
   spec:
     nodeType: CloudStatic
   EOF
   ```

1. Make sure that NodeGroup has been created and synchronized:

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
     -o jsonpath='{.data.bootstrap\.sh}' > ${NODE_GROUP}-bootstrap.b64
   ```

1. On the master node, verify that the file contains valid Base64 data of the bootstrap script:

   ```shell
   base64 -d ${NODE_GROUP}-bootstrap.b64 > /dev/null
   ```

   Check the beginning of the decoded content:

   ```shell
   base64 -d ${NODE_GROUP}-bootstrap.b64 | head -n 5
   ```

   The decoded content must start with a bash script:

   ```console
   #!/bin/bash
   ...
   ```

1. Copy the bootstrap script to the virtual machine being connected:

   ```shell
   scp ${NODE_GROUP}-bootstrap.b64 <USER>@<NODE_IP>:/tmp/bootstrap.b64
   ```

1. Connect to the virtual machine over SSH:

   ```shell
   ssh <USER>@<NODE_IP>
   ```

1. On the virtual machine, decode the bootstrap script, set permissions, and run it:

   ```shell
   base64 -d /tmp/bootstrap.b64 > /tmp/bootstrap.sh
   chmod +x /tmp/bootstrap.sh

   sudo bash /tmp/bootstrap.sh
   ```

   After the bootstrap script starts, it will install the required components, configure the container runtime and kubelet, and connect the node to the cluster.

1. On the master node, check that a new node appears:

   ```shell
   d8 k get nodes -o wide
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                   STATUS   ROLES                  AGE   VERSION    INTERNAL-IP
   dvp-hybrid-master-0    Ready    control-plane,master   1h    v1.33.12   10.12.0.69
   cloud-static-worker-0  Ready    cloud-static           5m    v1.33.12   10.12.3.88
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. If connection fails, check the NodeGroup status, events, and bootstrap logs on the virtual machine being connected:

   ```shell
   d8 k get nodegroup cloud-static
   d8 k describe nodegroup cloud-static
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

   On the virtual machine being connected:

   ```shell
   sudo tail -n 120 /var/log/d8/bashible/bootstrap.log
   ```

   If the logs contain the `Failed to discover node_ip that matches internalNetworkCIDRs` error, check that the virtual machine IP address belongs to `internalNetworkCIDRs`.
