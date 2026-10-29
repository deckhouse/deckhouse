---
title:  Hybrid cluster with Yandex Cloud
permalink: en/admin/integrations/hybrid/yandex-hybrid.html
search: hybrid with Yandex Cloud
description: Preparation for hybrid integration with Yandex Cloud in Deckhouse Platform.
---

The following describes the process of adding nodes from Yandex Cloud to an existing static Deckhouse Platform cluster.

Integration with Yandex Cloud uses the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module. It provides interaction between DP and the Yandex Cloud API, retrieval of information about cloud infrastructure, creation of virtual machines, work with network parameters, and connection of nodes to an existing cluster.

This section describes two ways to add nodes:

- **Automatic node creation in Yandex Cloud**. DP creates virtual machines through the Yandex Cloud API. VM parameters are defined by the [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) resource, and the required number of nodes and placement zones are defined by the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the `CloudEphemeral` type.
- **Connecting manually created nodes through a [bootstrap script](../../../glossary.html#bootstrap-script)**. A virtual machine is created by the user in advance and connected to the cluster using the DP bootstrap script. This scenario uses [NodeGroup](/modules/node-manager/cr.html#nodegroup) with the `CloudStatic` type.

## Prerequisites

Before you begin, make sure that the following conditions are met:

- The cluster was created with the [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype) parameter, and its nodes meet the [minimum system requirements](/products/kubernetes-platform/guides/hardware-requirements.html).
- ["Network connectivity"](./overview.html#general-network-requirements) is configured between the network of [static nodes](../../../glossary.html#static-node) and the Yandex Cloud VPC. Yandex Cloud nodes added to the cluster have access to the Kubernetes API, DNS, and the required addresses according to the ["Network interaction"](../../../../reference/network_interaction.html) and ["Network policy configuration"](../../configuration/network/policy/configuration.html) sections. When using Cilium with pod traffic tunneling, the [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) mode is selected according to the network connectivity between sites.
- The requirements from the ["Connection and authorization in Yandex Cloud"](../public/yandex/authorization.html) section are met:
  - A service account is prepared.
  - A folder where resources will be created is selected.
  - The required roles and access to the VPC being used are configured.

## Adding automatically created nodes

To run the preparation commands, you need the [Yandex Cloud CLI](https://yandex.cloud/en/docs/cli/) (`yc`). You can use it on the administrator's workstation. The `yc` CLI is not required on the cluster [master node](../../../glossary.html#master-node): only the prepared manifests need to be applied in the cluster.

1. Get the identifiers of the cloud and folder where nodes will be created:

   ```shell
   yc resource-manager cloud list
   yc resource-manager folder list
   ```

1. Specify the obtained identifiers in variables:

   ```shell
   export CLOUD_ID="<CLOUD_ID>"
   export FOLDER_ID="<FOLDER_ID>"
   ```

   Where:

   - `CLOUD_ID` — Yandex Cloud cloud ID;
   - `FOLDER_ID` — ID of the folder where resources will be created.

1. Get the identifiers of the network, subnet, and zone where nodes will be created:

   ```shell
   yc vpc network list --folder-id "$FOLDER_ID"
   yc vpc subnet list --folder-id "$FOLDER_ID"
   yc compute zone list
   ```

1. Specify the obtained values in variables:

   ```shell
   export NETWORK_ID="<NETWORK_ID>"
   export SUBNET_ID="<SUBNET_ID>"
   export ZONE="<ZONE>"
   export ROUTE_TABLE_ID="<ROUTE_TABLE_ID>"
   ```

   Where:

   - `NETWORK_ID` — VPC network ID;
   - `SUBNET_ID` — ID of the subnet where nodes will be created;
   - `ZONE` — [availability zone](../../../glossary.html#availability-zone) that corresponds to the selected subnet, for example `ru-central1-a`;
   - `ROUTE_TABLE_ID` — ID of the route table attached to the subnet. It is shown in the `ROUTE TABLE ID` column of the `yc vpc subnet list` output.

   For details, see ["Connecting and authorizing in Yandex Cloud"](../public/yandex/authorization.html).

1. Create a service account in the required Yandex Cloud folder and assign permissions to it:

   ```shell
   yc iam service-account create \
     --name dkp-hybrid \
     --folder-id "$FOLDER_ID"

   export SA_ID="$(yc iam service-account get \
     --name dkp-hybrid \
     --folder-id "$FOLDER_ID" \
     --format json | jq -r .id)"

   yc resource-manager folder add-access-binding "$FOLDER_ID" \
     --role editor \
     --subject "serviceAccount:${SA_ID}"

   yc resource-manager folder add-access-binding "$FOLDER_ID" \
     --role vpc.admin \
     --subject "serviceAccount:${SA_ID}"
   ```

   The `editor` role is required to create and manage cloud resources, and the `vpc.admin` role grants access to VPC network resources.

1. Create a service account key and save it to a JSON file:

   ```shell
   yc iam key create \
     --service-account-id "$SA_ID" \
     --output dkp-hybrid-sa-key.json
   ```

   Save the service account JSON key to the `SERVICE_ACCOUNT_JSON` environment variable in single-line format:

   ```shell
   export SERVICE_ACCOUNT_JSON="$(jq -c . dkp-hybrid-sa-key.json)"
   ```

1. Save the administrator's public SSH key to the `SSH_PUBLIC_KEY` environment variable:

   ```shell
   export SSH_PUBLIC_KEY="$(cat ~/.ssh/id_rsa.pub)"
   ```

   If you use another key, specify its path instead of `~/.ssh/id_rsa.pub`.

   {% alert level="warning" %}
   The `SSH_PUBLIC_KEY` variable must contain the administrator's public SSH key that will be used to access the nodes being created. Do not use the public key from the service account JSON file.
   {% endalert %}

1. Get the ID of the operating system image that will be used to create virtual machines and save it to the `IMAGE_ID` environment variable:

   ```shell
   export IMAGE_ID="$(yc compute image get-latest-from-family ubuntu-2404-lts \
     --folder-id standard-images \
     --format json | jq -r .id)"
   ```

   {% alert level="warning" %}
   The `IMAGE_ID` variable must contain the OS image ID in Yandex Cloud. Do not use an existing virtual machine ID or a service account key ID.
   {% endalert %}

1. Specify the CIDR of the network where Yandex Cloud nodes will be placed:

   ```shell
   export NODE_NETWORK_CIDR="<NODE_NETWORK_CIDR>"
   ```

   `NODE_NETWORK_CIDR` is the CIDR that includes the internal IP addresses of Yandex Cloud nodes. For a single zone, it usually matches the CIDR of the selected subnet. For example, if nodes are created in the `10.128.0.0/24` subnet, specify `10.128.0.0/24`. You can get the subnet CIDR with the following command:

   ```shell
   yc vpc subnet list --folder-id "$FOLDER_ID"
   ```

1. Create the [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) manifest for the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module. For example, `yandex-module-config.yaml`:

   ```shell
   cat > yandex-module-config.yaml <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-yandex
   spec:
     version: 2
     enabled: true
     settings:
       provider:
         parameters:
           cloudID: ${CLOUD_ID}
           folderID: ${FOLDER_ID}
       nodes:
         parameters:
           layout: WithoutNAT
           nodeNetworkCIDR: ${NODE_NETWORK_CIDR}
           sshPublicKey: '${SSH_PUBLIC_KEY}'
           existingNetworkID: ${NETWORK_ID}
           existingRouteTableID: ${ROUTE_TABLE_ID}
           existingZoneToSubnetIDMap:
             ${ZONE}: ${SUBNET_ID}
           zones:
             - ${ZONE}
   EOF
   ```

   The manifest automatically uses the values of the environment variables set in the previous steps: `CLOUD_ID`, `FOLDER_ID`, `NODE_NETWORK_CIDR`, `SSH_PUBLIC_KEY`, `NETWORK_ID`, `ZONE`, and `SUBNET_ID`.

   The [`existingNetworkID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingnetworkid), [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap), and [`existingRouteTableID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingroutetableid) parameters describe the Yandex Cloud resources that already exist, because in a hybrid cluster DP does not create the network, the subnets, or the route table. The route table is required because the cloud controller manager creates the pod subnet routes in it and does not start with an empty value.

   The layout is set by the [`layout`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-layout) parameter. The available layouts are described in ["Configuration and layouts"](../public/yandex/layout.html#layouts).

1. Copy the `yandex-module-config.yaml` file to the cluster master node. Apply the manifest:

   ```shell
   d8 k apply -f yandex-module-config.yaml
   ```

   While DP is enabling the module, the validating webhooks of DP and the provider may be unavailable, and the command fails with `failed calling webhook`. This state is temporary, repeat the command in half a minute. The same applies to the next step with the Secret.

1. Wait until the module creates the `d8-cloud-provider-yandex` namespace:

   ```shell
   d8 k get namespace d8-cloud-provider-yandex
   ```

1. Create the Secret with the service account key:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: v1
   kind: Secret
   metadata:
     name: d8-credentials
     namespace: d8-cloud-provider-yandex
   type: cloud-provider.deckhouse.io/credentials
   stringData:
     authScheme: serviceAccount
     secret: '${SERVICE_ACCOUNT_JSON}'
   EOF
   ```

   The Secret format is described in ["Credentials Secret"](../public/yandex/authorization.html#credentials-secret).

1. Wait until the `cloud-provider-yandex` module is enabled and the YandexInstanceClass resource appears:

   ```shell
   d8 k get module cloud-provider-yandex -o wide
   d8 k get crd yandexinstanceclasses.deckhouse.io
   ```

1. Create a file with [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) and [NodeGroup](/modules/node-manager/cr.html#nodegroup) manifests. For example, `yandex-instanceclass-nodegroup.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: YandexInstanceClass
   metadata:
     name: yc-worker
   spec:
     cores: 4
     memory: 8192
     diskSizeGB: 50
     diskType: network-ssd
     mainSubnet: <SUBNET_ID>
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: yc-worker
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: YandexInstanceClass
         name: yc-worker
       minPerZone: 1
       maxPerZone: 1
       zones:
         - ru-central1-a
   ```

   Where:

   - YandexInstanceClass describes the parameters of the virtual machine that will be created in Yandex Cloud;
   - `mainSubnet` — ID of the subnet from which the created nodes must have access to the [static cluster](../../../glossary.html#static-cluster) nodes;
   - NodeGroup describes the [node group](../../../glossary.html#node-group) that DP must maintain in the cluster;
   - `nodeType: CloudEphemeral` means that nodes will be created automatically through the [cloud provider](../../../glossary.html#cloud-provider);
   - `cloudInstances.zones` must contain zones from the `zones` list in the module settings.

   In the `WithoutNAT` [layout](../../../glossary.html#layout), the created nodes get a public IP address. To keep the nodes of the group on internal addresses only, set the [`assignPublicIPAddress: false`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-assignpublicipaddress) parameter in the YandexInstanceClass and provide the nodes with another way out to the internet, for example through a subnet NAT gateway.

1. Apply the manifest:

   ```shell
   d8 k apply -f yandex-instanceclass-nodegroup.yaml
   ```

   After the manifest is applied, DP will start creating a virtual machine in Yandex Cloud through machine-controller-manager.

1. Check that the node appears in the cluster:

   ```shell
   d8 k get nodes -o wide
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                                 STATUS   ROLES                  AGE   VERSION    INTERNAL-IP
   static-master-0                      Ready    control-plane,master   1h    v1.33.10   10.128.0.15
   yc-worker-f3564dca-7fc59-s2w5d       Ready    yc-worker              10m   v1.33.10   10.128.0.21
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. To diagnose the state and troubleshoot possible issues, check the machine-controller-manager logs:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinedeployments.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager get machinesets.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager get machines.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

## Adding manually created nodes through a bootstrap script

Before you begin, make sure that the following conditions are met:

- The [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module is enabled:

  ```shell
  d8 k get module cloud-provider-yandex -o wide
  ```

- The `cloud-provider-yandex` module components are in the `Running` state:

  ```shell
  d8 k -n d8-cloud-provider-yandex get pods -o wide
  ```

- A virtual machine that will be connected to the cluster has been created in Yandex Cloud.
- The virtual machine is connected to the Yandex Cloud network and subnet used for hybrid integration with the cluster.
- The virtual machine has a network interface in the Yandex Cloud VPC network and subnet used for hybrid integration with the cluster. The IP address of this interface must belong to the CIDR specified in `nodeNetworkCIDR` and be reachable from the static cluster nodes.
- The virtual machine name in Yandex Cloud matches the hostname inside the operating system.
- One of the package managers (`apt`/`apt-get`, `yum`, or `rpm`) for a supported OS is installed on the virtual machine.

1. Check the virtual machine metadata in Yandex Cloud.

   The VM metadata must have `cloud-init` configured with the user that will be used for SSH connection.

   Example metadata:

   ```yaml
   #cloud-config
   datasource:
     Ec2:
       strict_id: false
   ssh_pwauth: no
   users:
     - name: <USER>
       sudo: ALL=(ALL) NOPASSWD:ALL
       shell: /bin/bash
       ssh_authorized_keys:
         - <SSH_PUBLIC_KEY>
   ```

   Where:

   - `<USER>`: Username for SSH access to the virtual machine.
   - `<SSH_PUBLIC_KEY>`: Administrator's public SSH key.

1. On the [master node](../../../glossary.html#master-node), create a file with the NodeGroup manifest and specify the [node group](../../../glossary.html#node-group) name. In this example and the following steps, the `yc-manual` name is used. For example, `yandex-manual-nodegroup.yaml`:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: yc-manual
   spec:
     nodeType: CloudStatic
   EOF
   ```

1. Make sure that the NodeGroup has been created and synchronized:

   ```shell
   d8 k get nodegroup yc-manual
   d8 k describe nodegroup yc-manual
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME        TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   yc-manual   CloudStatic   0       0       0                                                               1m    True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. On the master node, get the [bootstrap script](../../../glossary.html#bootstrap-script) for the created NodeGroup:

   ```shell
   NODE_GROUP=yc-manual

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > ${NODE_GROUP}-bootstrap.b64
   ```

1. Copy the bootstrap script to the VM being connected. If SSH access to the VM is available from the master node, run the following on the master node:

   ```shell
   scp ${NODE_GROUP}-bootstrap.b64 <USER>@<NODE_PUBLIC_OR_INTERNAL_IP>:/tmp/bootstrap.b64
   ```

   {% alert level="info" %}
   To copy and run the bootstrap script, use the user specified in the VM metadata.
   {% endalert %}

   If SSH access to the VM is available only from the administrator's workstation, first copy the file from the master node to the workstation, and then from the workstation to the VM:

   ```shell
   scp <MASTER_USER>@<MASTER_IP>:/root/${NODE_GROUP}-bootstrap.b64 ./bootstrap.b64
   scp ./bootstrap.b64 <USER>@<NODE_PUBLIC_OR_INTERNAL_IP>:/tmp/bootstrap.b64
   ```

   Where:

   - `<MASTER_USER>`: User for SSH access to the master node.
   - `<MASTER_IP>`: IP address of the master node.
   - `<USER>`: User on the VM being connected.
   - `<NODE_PUBLIC_OR_INTERNAL_IP>`: Public or internal IP address of the VM being connected.

1. On the VM being connected, decode the bootstrap script, set permissions, and run it:

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
   NAME                    STATUS   ROLES       AGE   VERSION    INTERNAL-IP   EXTERNAL-IP
   static-master-0         Ready    master      1h    v1.33.10   10.128.0.15   <none>
   yandex-worker-hybrid    Ready    yc-manual   5m    v1.33.10   10.128.0.17   <PUBLIC_IP>
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. If connection fails, check the NodeGroup status, events, and component logs:

   ```shell
   d8 k get nodegroup yc-manual
   d8 k describe nodegroup yc-manual
   d8 k describe node yandex-worker-hybrid
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```
