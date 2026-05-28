---
title:  Hybrid cluster with Yandex Cloud
permalink: en/admin/integrations/hybrid/yandex-hybrid.html
search: hybrid with Yandex Cloud
description: Preparation for hybrid integration with Yandex Cloud in Deckhouse Kubernetes Platform.
---

The following describes the process of adding worker nodes from Yandex Cloud to an existing static Deckhouse Kubernetes Platform cluster.

Integration with Yandex Cloud uses the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module. It provides interaction between DKP and the Yandex Cloud API, retrieval of information about cloud infrastructure, creation of virtual machines, work with network parameters, and connection of nodes to an existing cluster.

This section describes three ways to add worker nodes:

- **Automatic node creation in Yandex Cloud**. DKP creates virtual machines through the Yandex Cloud API. VM parameters are defined by the [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) resource, and the required number of nodes and placement zones are defined by the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the `Cloud` type.
- **Connecting manually created nodes through CAPS**. A virtual machine is created by the user in advance, and DKP connects to it over SSH through Cluster API Provider Static. This uses the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the `Static` type, as well as the [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) and [StaticInstance](/modules/node-manager/cr.html#staticinstance) resources.
- **Connecting manually created nodes through a bootstrap script**. A virtual machine is created by the user in advance and connected to the cluster using the DKP bootstrap script. This scenario uses [NodeGroup](/modules/node-manager/cr.html#nodegroup) with the `Hybrid` type.

## Prerequisites

Before you begin, make sure that the following conditions are met:

- The cluster was created with the [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype) parameter.
- Network connectivity is configured between the network of static nodes and the Yandex Cloud VPC.
- Yandex Cloud nodes added to the cluster have access to the Kubernetes API, DNS, and the required addresses according to the [Network interaction](../../../../reference/network_interaction.html) and [Network policy configuration](../../configuration/network/policy/configuration.html) sections.
- The requirements from the [Connection and authorization in Yandex Cloud](../public/yandex/authorization.html) section are met:
  - A service account is prepared.
  - A folder where resources will be created is selected.
  - The required roles and access to the VPC being used are configured.
- When using Cilium with pod traffic tunneling, the [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) mode is selected according to the network connectivity between sites.

## Adding automatically created nodes

To run the preparation commands, you need the [Yandex Cloud CLI](https://yandex.cloud/ru/docs/cli/) (`yc`). You can use it on the administrator's workstation. The `yc` CLI is not required on the cluster master node: only the prepared manifests need to be applied in the cluster.

1. Prepare the cloud, folder, network, subnet, and zone identifiers where worker nodes will be created:

   ```shell
   export CLOUD_ID="<CLOUD_ID>"
   export FOLDER_ID="<FOLDER_ID>"
   export NETWORK_ID="<NETWORK_ID>"
   export SUBNET_ID="<SUBNET_ID>"
   export ZONE="ru-central1-a"
   ```

   You can get the values using the Yandex Cloud CLI:

   ```shell
   yc resource-manager cloud list
   yc resource-manager folder list
   yc vpc network list --folder-id "$FOLDER_ID"
   yc vpc subnet list --folder-id "$FOLDER_ID"
   ```

   Where:

   - `CLOUD_ID`: Yandex Cloud cloud ID.
   - `FOLDER_ID`: ID of the folder where resources will be created.
   - `NETWORK_ID`: VPC network ID.
   - `SUBNET_ID`: ID of the subnet where worker nodes will be created.
   - `ZONE`: Availability zone corresponding to the selected subnet.

   For details, see [Connection and authorization in Yandex Cloud](../public/yandex/authorization.html) section.

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

   The `editor` role is required for creating and managing cloud resources, and `vpc.admin` is required for working with VPC network resources.

1. Create a service account key and save it to a JSON file:

   ```shell
   yc iam key create \
     --service-account-id "$SA_ID" \
     --output dkp-hybrid-sa-key.json
   ```

   Prepare the `serviceAccountJSON` value in a single-line format:

   ```shell
   export SERVICE_ACCOUNT_JSON="$(jq -c . dkp-hybrid-sa-key.json)"
   ```

1. Prepare the public SSH key that will be added to the created worker nodes:

   ```shell
   export SSH_PUBLIC_KEY="$(cat ~/.ssh/id_rsa.pub)"
   ```

   If another key is used, specify its path instead of `~/.ssh/id_rsa.pub`.

   {% alert level="warning" %}
   The `sshPublicKey` parameter must contain the administrator's public SSH key, not the public key from the service account JSON file.
   {% endalert %}

1. Get the ID of the operating system image from which virtual machines will be created:

   ```shell
   export IMAGE_ID="$(yc compute image get-latest-from-family ubuntu-2404-lts \
     --folder-id standard-images \
     --format json | jq -r .id)"
   ```

   {% alert level="warning" %}
   The `imageID` parameter is the OS image ID in Yandex Cloud. Do not use the ID of an existing virtual machine or the service account key ID in this field.
   {% endalert %}

1. Specify the CIDR of the network where Yandex Cloud nodes will be placed:

   ```shell
   export NODE_NETWORK_CIDR="<NODE_NETWORK_CIDR>"
   ```

   `NODE_NETWORK_CIDR` is a CIDR that includes the internal IP addresses of Yandex Cloud nodes. For a single zone, it usually matches the CIDR of the selected subnet. For example, if worker nodes are created in the `10.128.0.0/24` subnet, specify `10.128.0.0/24`. You can get the subnet CIDR using the command:

   ```shell
   yc vpc subnet list --folder-id "$FOLDER_ID"
   ```

1. Create a file with the provider configuration. For example, `cloud-provider-cluster-configuration.yaml`:

   ```shell
   cat > cloud-provider-cluster-configuration.yaml <<EOF
   apiVersion: deckhouse.io/v1
   kind: YandexClusterConfiguration
   layout: WithoutNAT
   masterNodeGroup:
     replicas: 1
     instanceClass:
       cores: 4
       memory: 8192
       imageID: ${IMAGE_ID}
       diskSizeGB: 100
       platform: standard-v3
       externalIPAddresses:
         - "Auto"
   nodeNetworkCIDR: ${NODE_NETWORK_CIDR}
   existingNetworkID: empty
   provider:
     cloudID: ${CLOUD_ID}
     folderID: ${FOLDER_ID}
     serviceAccountJSON: '${SERVICE_ACCOUNT_JSON}'
   sshPublicKey: '${SSH_PUBLIC_KEY}'
   EOF
   ```

   Where:

   - `nodeNetworkCIDR`: CIDR of the network that includes the addresses of the subnets used for Yandex Cloud nodes.
   - `imageID`: OS image ID for the created virtual machines.
   - `cloudID`: Yandex Cloud cloud ID.
   - `folderID`: Yandex Cloud folder ID.
   - `serviceAccountJSON`: Service account JSON key in a single-line format.
   - `sshPublicKey`: Public SSH key for accessing the created nodes.

   {% alert level="info" %}
   In a hybrid scenario, when the control plane is already deployed as a static cluster, the `masterNodeGroup` section does not create master nodes in Yandex Cloud, but remains part of the provider configuration.
   {% endalert %}

1. Create a file with Yandex Cloud discovery data. For example, `cloud-provider-discovery-data.json`:

   ```shell
   cat > cloud-provider-discovery-data.json <<EOF
   {
     "apiVersion": "deckhouse.io/v1",
     "defaultLbTargetGroupNetworkId": "empty",
     "internalNetworkIDs": [
       "${NETWORK_ID}"
     ],
     "kind": "YandexCloudDiscoveryData",
     "monitoringAPIKey": "",
     "region": "ru-central1",
     "routeTableID": "empty",
     "shouldAssignPublicIPAddress": false,
     "zoneToSubnetIdMap": {
       "${ZONE}": "${SUBNET_ID}"
     },
     "zones": [
       "${ZONE}"
     ]
   }
   EOF
   ```

   Where:

   - `internalNetworkIDs`: List of Yandex Cloud network IDs that provide internal connectivity between nodes.
   - `zoneToSubnetIdMap`: Mapping between an availability zone and the subnet where nodes will be created.
   - `zones`: List of zones available for node creation.
   - `shouldAssignPublicIPAddress`: Controls assignment of public IP addresses to the created nodes.

   {% alert level="warning" %}
   If the `shouldAssignPublicIPAddress` parameter is set to `false`, the created nodes will not have a public IP address. In this case, the nodes must have access to the registry and external services through a NAT Gateway, NAT instance, proxy, or another egress mechanism. For zones where subnets are missing, the `empty` value can be used.
   {% endalert %}

1. Encode the `cloud-provider-cluster-configuration.yaml` and `cloud-provider-discovery-data.json` files in Base64:

   ```shell
   export CLUSTER_CONFIGURATION_B64="$(base64 -w0 cloud-provider-cluster-configuration.yaml)"
   export DISCOVERY_DATA_B64="$(base64 -w0 cloud-provider-discovery-data.json)"
   ```

1. Create a manifest with the `d8-provider-cluster-configuration` Secret and ModuleConfig for the `cloud-provider-yandex` module:

   ```shell
   cat > yandex-provider-secret-and-mc.yaml <<EOF
   apiVersion: v1
   kind: Secret
   metadata:
     labels:
       heritage: deckhouse
       name: d8-provider-cluster-configuration
     name: d8-provider-cluster-configuration
     namespace: kube-system
   type: Opaque
   data:
     cloud-provider-cluster-configuration.yaml: ${CLUSTER_CONFIGURATION_B64}
     cloud-provider-discovery-data.json: ${DISCOVERY_DATA_B64}
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-yandex
   spec:
     version: 1
     enabled: true
     settings:
       storageClass:
         default: network-ssd
   EOF
   ```

1. Copy the `yandex-provider-secret-and-mc.yaml` file to the cluster master node. Before applying it, delete the ValidatingAdmissionPolicyBinding object if it prevents creating objects with the `heritage: deckhouse` label:

   ```shell
   d8 k delete validatingadmissionpolicybindings.admissionregistration.k8s.io \
     heritage-label-objects.deckhouse.io \
     --ignore-not-found
   ```

   Apply the manifest:

   ```shell
   d8 k apply -f yandex-provider-secret-and-mc.yaml
   ```

1. Wait for the `cloud-provider-yandex` module to be enabled and for the YandexInstanceClass resource to appear:

   ```shell
   d8 k get moduleconfig cloud-provider-yandex
   d8 k get crd yandexinstanceclasses.deckhouse.io
   d8 k -n d8-cloud-provider-yandex get pods -o wide
   ```

1. Create a file with the YandexInstanceClass and NodeGroup resources. For example, `yandex-instanceclass-nodegroup.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
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
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroup
   metadata:
     name: yc-worker
   spec:
     nodeType: Cloud
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

   - YandexInstanceClass describes the parameters of the virtual machine that will be created in Yandex Cloud.
   - `mainSubnet` ID of the subnet from which the created worker nodes must have access to the cluster's static nodes.
   - NodeGroup describes the node group that DKP must maintain in the cluster.
   - `nodeType: Cloud` means that nodes will be created automatically through the cloud provider.
   - `cloudInstances.zones` must contain zones from the `zones` list in `cloud-provider-discovery-data.json`.

1. Apply the manifest:

   ```shell
   d8 k apply -f yandex-instanceclass-nodegroup.yaml
   ```

   After applying the manifest, DKP will start creating a virtual machine in Yandex Cloud through machine-controller-manager.

1. Check that the node has appeared in the cluster:

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

1. To diagnose the status and find possible issues, check the machine-controller-manager logs:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinedeployments.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager get machinesets.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager get machines.machine.sapcloud.io -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

## Adding manually created nodes through CAPS

Before you begin, make sure that the following conditions are met:

- The [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module is enabled and configured.
- The `cloud-provider-yandex` module components are in the `Running` state:

  ```shell
  d8 k -n d8-cloud-provider-yandex get pods -o wide
  ```

- A virtual machine that will be connected to the cluster has been created in Yandex Cloud.
- The virtual machine is connected to the Yandex Cloud network and subnet used for hybrid integration with the cluster.
- The internal IP address of the virtual machine is within the address range used for Yandex Cloud nodes.
- The virtual machine name in Yandex Cloud matches the hostname inside the operating system.
- The virtual machine has the required base packages installed for the supported OS. For RED OS, install `which` and the package manager in advance if they are missing.

1. On the master node, set the variables for the NodeGroup being created and the virtual machine being connected:

   ```shell
   export NODE_GROUP="yc-caps"
   export NODE_NAME="yandex-worker-hybrid-caps"
   export NODE_SSH_IP="<NODE_PUBLIC_OR_INTERNAL_IP>"
   export CAPS_USER="caps"
   ```

   Where:

   - `NODE_GROUP`: Name of the NodeGroup to which the node will be added.
   - `NODE_NAME`: Name of the node being connected.
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

   This scenario uses `nodeType: Static` because the node has already been created manually, and CAPS will only connect to it over SSH and configure it.

1. Make sure that the NodeGroup has been created and synchronized:

   ```shell
   d8 k get nodegroup ${NODE_GROUP}
   d8 k describe nodegroup ${NODE_GROUP}
   ```

   Example expected output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME      TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   yc-caps   Static   0       0       0                                                               1m    True
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
   NAME                         STATUS   ROLES     AGE   VERSION    INTERNAL-IP   EXTERNAL-IP
   static-master-0              Ready    master    1h    v1.33.10   10.128.0.15   <none>
   yandex-worker-hybrid-caps    Ready    yc-caps   5m    v1.33.10   10.128.0.29   <none>
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

- The [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module is enabled and configured.
- The `cloud-provider-yandex` module components are in the `Running` state:

  ```shell
  d8 k -n d8-cloud-provider-yandex get pods -o wide
  ```

- A virtual machine that will be connected to the cluster has been created in Yandex Cloud.
- The virtual machine is connected to the Yandex Cloud network and subnet used for hybrid integration with the cluster.
- The internal IP address of the virtual machine is within the address range used for Yandex Cloud nodes.
- The virtual machine name in Yandex Cloud matches the hostname inside the operating system.
- The virtual machine has the required base packages installed for the supported OS. For RED OS, install `which` and the package manager in advance if they are missing.

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

1. On the master node create a file with a NodeGroup resource. For example, `yandex-manual-nodegroup.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: yc-manual
   spec:
     nodeType: Hybrid
   ```

   {% alert level="info" %}
   For manually created Yandex Cloud nodes, use the `nodeType: Hybrid` value. In the NodeGroup status, such a group may be displayed as `CloudStatic`.
   {% endalert %}

1. Apply the manifest:

   ```shell
   d8 k apply -f yandex-manual-nodegroup.yaml
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

1. On the master node, get the bootstrap script for the created NodeGroup:

   ```shell
   NODE_GROUP=yc-manual

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > ${NODE_GROUP}-bootstrap.b64
   ```

1. On the master node, check that the file contains the Base64 data of the bootstrap script:

   ```shell
   head -c 80 ${NODE_GROUP}-bootstrap.b64
   echo
   base64 -d ${NODE_GROUP}-bootstrap.b64 | head -n 5
   ```

   The decoded content must start with a bash script:

   ```console
   #!/bin/bash
   ```

   {% alert level="info" %}
   To copy and run the bootstrap script, use the user specified in the VM metadata.
   {% endalert %}

1. Copy the bootstrap script to the VM being connected. If SSH access to the VM is available from the master node, run the following on the master node:

   ```shell
   scp ${NODE_GROUP}-bootstrap.b64 <USER>@<NODE_PUBLIC_OR_INTERNAL_IP>:/tmp/bootstrap.b64
   ```

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
