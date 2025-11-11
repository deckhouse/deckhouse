---
title: Hybrid integrations
permalink: en/admin/integrations/hybrid/overview.html
---

Deckhouse Kubernetes Platform (DKP) can use cloud provider resources to expand the capacity of static clusters.
Currently, integration is supported with [OpenStack](../public/openstack/connection-and-authorization.html) and [vSphere](../virtualization/vsphere/authorization.html)-based clouds.

A hybrid cluster is a Kubernetes cluster that combines bare-metal nodes with nodes running on vSphere or OpenStack.
To create such a cluster, an L2 network must be available between all nodes.

{% alert level="info" %}
The Deckhouse Kubernetes Platform allows to set a prefix for the names of CloudEphemeral nodes added to a hybrid cluster with Static master nodes.
To do this, use the [`instancePrefix`](/modules/node-manager/configuration.html#parameters-instanceprefix) parameter of the `node-manager` module. The prefix specified in the parameter will be added to the name of all CloudEphemeral nodes added to the cluster. It is not possible to set a prefix for a specific NodeGroup.
{% endalert %}

## Hybrid cluster with vSphere

Follow these steps:

1. Remove `flannel` from the `kube-system` namespace:

   ```shell
   kubectl -n kube-system delete ds flannel-ds
   ```

1. Configure the integration and set the required parameters.

{% alert level="warning" %}
`Cloud-controller-manager` synchronizes state between vSphere and Kubernetes,
removing nodes from Kubernetes that are not present in vSphere.
In a hybrid cluster, this behavior is not always desirable.
Therefore, any Kubernetes node not launched with the `--cloud-provider=external` flag will be automatically ignored.
DKP automatically sets `static://` in the `.spec.providerID` field of such nodes, which `cloud-controller-manager` then ignores.
{% endalert %}

## Hybrid cluster with OpenStack

Follow these steps:

1. Remove `flannel` from the `kube-system` namespace:

   ```shell
   kubectl -n kube-system delete ds flannel-ds
   ```

1. Configure the integration and set the required parameters.
1. Create one or more [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) custom resources.
1. Create one or more [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources to manage the number and provisioning of cloud-based VMs.

{% alert level="warning" %}
`Cloud-controller-manager` synchronizes state between OpenStack and Kubernetes,
removing nodes from Kubernetes that are not present in OpenStack.
In a hybrid cluster, this behavior is not always desirable.
Therefore, any Kubernetes node not launched with the `--cloud-provider=external` flag will be automatically ignored.
DKP automatically sets `static://` in the `.spec.providerID` field of such nodes, which `cloud-controller-manager` then ignores.
{% endalert %}

### Storage integration

If you require PersistentVolumes on nodes connected to the cluster from OpenStack, you must create a StorageClass with the appropriate OpenStack volume type. You can get a list of available types using the following command:

```shell
openstack volume type list
```

Example for `ceph-ssd` volume type:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # Leave this as shown here.
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```

## Hybrid cluster with Yandex Cloud

To create a hybrid cluster combining static nodes and nodes in Yandex Cloud, follow these steps.

### Prerequisites

- A working cluster with the parameter `clusterType: Static`.
- The CNI controller switched to VXLAN mode. For details, refer to the [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) parameter.
- Configured network connectivity between the static cluster node network and VCD (either at L2 level, or at L3 level with port access according to the [required network policies for DKP operation](../../configuration/network/policy/)).

### Setup steps

1. Create a Service Account in the required Yandex Cloud folder:

   - Assign the `editor` role.
   - Provide access to the used VPC with the `vpc.admin` role.

1. Create the `d8-provider-cluster-configuration` secret with the required data. Example `cloud-provider-cluster-configuration.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: YandexClusterConfiguration
   layout: WithoutNAT
   masterNodeGroup:
     replicas: 1
     instanceClass:
       cores: 4
       memory: 8192
       imageID: fd80bm0rh4rkepi5ksdi
       diskSizeGB: 100
       platform: standard-v3
       externalIPAddresses:
       - "Auto"
   nodeNetworkCIDR: 10.160.0.0/16
   existingNetworkID: empty
   provider:
     cloudID: CLOUD_ID
     folderID: FOLDER_ID
     serviceAccountJSON: '{"id":"ajevk1dp8f9...--END PRIVATE KEY-----\n"}'
   sshPublicKey: <ssh-rsa SSHKEY>
   ```

   Parameter descriptions:
   - `nodeNetworkCIDR` — CIDR of the network covering all node subnets in Yandex Cloud.
   - `cloudID` — Your cloud ID.
   - `folderID` — Your folder ID.
   - `serviceAccountJSON` — The Service Account in JSON format.
   - `sshPublicKey` — Public SSH key for deployed machines.

     Values in `masterNodeGroup` are irrelevant since master nodes are not created.

1. Fill in `data.cloud-provider-discovery-data.json` in the same secret. Example:

   ```yaml
   {
     "apiVersion": "deckhouse.io/v1",
     "defaultLbTargetGroupNetworkId": "empty",
     "internalNetworkIDs": [
       "<NETWORK-ID>"
     ],
     "kind": "YandexCloudDiscoveryData",
     "monitoringAPIKey": "",
     "region": "ru-central1",
     "routeTableID": "empty",
     "shouldAssignPublicIPAddress": false,
     "zoneToSubnetIdMap": {
       "ru-central1-a": "<A-SUBNET-ID>",
       "ru-central1-b": "<B-SUBNET-ID>", 
       "ru-central1-d": "<D-SUBNET-ID>"
     },
     "zones": [
       "ru-central1-a",
       "ru-central1-b",
      "ru-central1-d"
     ]
   }
   ```

    Parameter descriptions:
    - `internalNetworkIDs` — List of network IDs in Yandex Cloud providing internal node connectivity.
    - `zoneToSubnetIdMap` — Zone-to-subnet mapping (one subnet per zone).
    - `shouldAssignPublicIPAddress: true` — Assigns public IPs to created nodes if required. For zones without subnets, can be set to `empty`.

1. Encode the above files (YandexClusterConfiguration and YandexCloudDiscoveryData) in Base64, then insert them into the `cloud-provider-cluster-configuration.yaml` and `cloud-provider-discovery-data.json` fields in the secret.

   ```yaml
   apiVersion: v1
   data:
     cloud-provider-cluster-configuration.yaml: <YANDEXCLUSTERCONFIGURATION_BASE64_ENCODED>
     cloud-provider-discovery-data.json: <YANDEXCLOUDDISCOVERYDATA-BASE64-ENCODED>
   kind: Secret
   metadata:
     labels:
       heritage: deckhouse
       name: d8-provider-cluster-configuration
     name: d8-provider-cluster-configuration
     namespace: kube-system
   type: Opaque
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
   ```

1. Remove the `ValidatingAdmissionPolicyBinding` object to avoid conflicts:

   ```shell
   d8 k delete validatingadmissionpolicybindings.admissionregistration.k8s.io heritage-label-objects.deckhouse.io
   ```

1. Apply the manifests in the cluster.

1. Wait for the `cloud-provider-yandex` module activation and CRD creation:

   ```shell
   d8 k get mc cloud-provider-yandex
   d8 k get crd yandexinstanceclasses
   ```

1. Create and apply NodeGroup and YandexInstanceClass manifests:

   ```yaml
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Cloud
     cloudInstances:
       classReference:
         kind: YandexInstanceClass
         name: worker
       minPerZone: 1
       maxPerZone: 3
       zones:
         - ru-central1-d
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: YandexInstanceClass
   metadata:
     name: worker
   spec:
     cores: 4
     memory: 8192
     diskSizeGB: 50
     diskType: network-ssd
     mainSubnet: <YOUR-SUBNET-ID>
   ```

   The `mainSubnet` parameter must contain the subnet ID from Yandex Cloud that is used for interconnection with your infrastructure (L2 connectivity with static node groups).

   After applying the manifests, the provisioning of virtual machines in Yandex Cloud managed by the `node-manager` module will begin.

1. Check the `machine-controller-manager` logs for troubleshooting:

   ```shell
   d8 k -n d8-cloud-provider-yandex get machine
   d8 k -n d8-cloud-provider-yandex get machineset
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager
   ```

## Hybrid cluster with VCD

This section describes the process of creating a hybrid cluster that combines static (bare-metal) nodes and cloud nodes in VMware vCloud Director (VCD) using Deckhouse Kubernetes Platform (DKP).

Before you begin, ensure the following conditions are met:

- **Infrastructure**:
  - A bare-metal DKP cluster is installed.
  - A tenant is configured in VCD [with allocated resources](../virtualization/vcd/connection-and-authorization.html).
  - Configured network connectivity between the static cluster node network and VCD (either at L2 level, or at L3 level with port access according to the [required network policies for DKP operation](../../configuration/network/policy/)).
  - A working network is configured in VCD with DHCP enabled.
  - A user with a static password and VCD administrator privileges has been created.

- **Software settings**:
  - The CNI controller is switched to VXLAN mode. More details — [`tunnelMode` configuration](/modules/cni-cilium/configuration.html#parameters-tunnelmode).
  - A [list of required VCD resources](../virtualization/vcd/connection-and-authorization.html) is prepared (VDC, VAPP, templates, policies, etc.).

### Setup

1. Create a configuration file `cloud-provider-vcd-token.yml` with the following content:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VCDClusterConfiguration
   layout: Standard
   mainNetwork: <NETWORK_NAME>
   internalNetworkCIDR: <NETWORK_CIDR>
   organization: <ORGANIZATION>
   virtualApplicationName: <VAPP_NAME>
   virtualDataCenter: <VDC_NAME>
   provider:
     server: <API_URL>
     apiToken: <PASSWORD>
     username: <USER_NAME>
     insecure: false
   masterNodeGroup:
     instanceClass:
       etcdDiskSizeGb: 10
       mainNetworkIPAddresses:
       - 192.168.199.2
       rootDiskSizeGb: 50
       sizingPolicy: <SIZING_POLICY>
       storageProfile: <STORAGE_PROFILE>
       template: <VAPP_TEMPLATE>
     replicas: 1
   sshPublicKey: <SSH_PUBLIC_KEY>
   ```

   Where:
   - `mainNetwork` — the name of the network where cloud nodes will be deployed in your VCD cluster.
   - `internalNetworkCIDR` — the CIDR of the specified network.
   - `organization` — the name of your VCD organization.
   - `virtualApplicationName` — the name of the vApp where nodes will be created (e.g., `dkp-vcd-app`).
   - `virtualDataCenter` — the name of the virtual data center.
   - `template` — the VM template used to create nodes.
   - `sizingPolicy` and `storageProfile` — corresponding policies configured in VCD.
   - `provider.server` — the API URL of your VCD instance.
   - `provider.apiToken` — the access token (password) of a user with administrator privileges in VCD.
   - `provider.username` — the name of the static user that will be used to interact with VCD.
   - `mainNetworkIPAddresses` — a list of IP addresses from the specified network that will be assigned to master nodes.
   - `storageProfile` — the name of the storage profile defining where the VM disks will be placed.

1. Encode the `cloud-provider-vcd-token.yml` file in Base64:

   ```shell
   base64 -i $PWD/cloud-provider-vcd-token.yml
   ```

1. Create a secret with the following content:

   ```yaml
   apiVersion: v1
   data:
     cloud-provider-cluster-configuration.yaml: <BASE64_STRING_OBTAINED_IN_THE_PREVIOUS_STEP>
     cloud-provider-discovery-data.json: eyJhcGlWZXJzaW9uIjoiZGVja2hvdXNlLmlvL3YxIiwia2luZCI6IlZDRENsb3VkUHJvdmlkZXJEaXNjb3ZlcnlEYXRhIiwiem9uZXMiOlsiZGVmYXVsdCJdfQo=
   kind: Secret
     metadata:
       labels:
         heritage: deckhouse
         name: d8-provider-cluster-configuration
       name: d8-provider-cluster-configuration
       namespace: kube-system
   type: Opaque
   ```

1. Enable the `cloud-provider-vcd` module:

   ```shell
   d8 system module enable cloud-provider-vcd
   ```

1. Edit the `d8-cni-configuration` secret so that the `mode` parameter is determined from `mc cni-cilium` (change `.data.cilium` to `.data.necilium` if necessary).

1. Verify that all pods in the `d8-cloud-provider-vcd` namespace are in the `Running` state:

   ```shell
   d8 k get pods -n d8-cloud-provider-vcd
   ```

1. Reboot the master node and wait for initialization to complete.

1. Create instance classes in VCD:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VCDInstanceClass
   metadata:
     name: worker
   spec:
     rootDiskSizeGb: 50
     sizingPolicy: <SIZING_POLICY>
     storageProfile: <STORAGE_PROFILE>
     template: <VAPP_TEMPLATE>
   ```  

1. Create a [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     cloudInstances:
       classReference:
         kind: VCDInstanceClass
         name: worker
       maxPerZone: 2
       minPerZone: 1
     nodeTemplate:
       labels:
         node-role/worker: ""
     nodeType: CloudEphemeral
   ```

1. Verify that the required number of nodes has appeared in the cluster:

   ```shell
   d8 k get nodes -o wide
   ```
