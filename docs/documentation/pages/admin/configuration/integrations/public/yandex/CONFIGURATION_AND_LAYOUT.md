---
title: Layouts and configuration in Yandex Cloud
permalink: en/admin/integrations/public/yandex/layout.html
---

## Layouts

This section describes the possible node placement layouts in Yandex Cloud infrastructure and the related configuration options. The layout is specified by the [`layout`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-layout) parameter in the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.
The selected layout affects networking behavior, availability of public IP addresses, outgoing traffic routing,
and how nodes are accessed.

### Standard

{% alert level="danger" %}
In this layout, nodes will not have public IP addresses and will access the internet via a Yandex Cloud NAT Gateway.
The NAT Gateway uses random public IP addresses from [dedicated address ranges](https://yandex.cloud/en/docs/overview/concepts/public-ips#virtual-private-cloud).
Because of this, you cannot whitelist the IP addresses of cloud resources
behind a specific NAT Gateway in third-party services.
{% endalert %}

{% alert level="warning" %}
Because nodes are created without public IP addresses in this layout, the master node must be accessible over SSH from the machine where `dhctl` is running: either directly over a private network or through a bastion host.
If the master node is not directly accessible, run the installation with the `--ssh-bastion-host`, `--ssh-bastion-user`, and, if needed, `--ssh-bastion-port` parameters.
{% endalert %}

![Standard layout in Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10422&t=Qb5yyWumzPiTBtfL-0 --->

Example layout configuration:

```yaml
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
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: <SSH_PUBLIC_KEY>
        nodeNetworkCIDR: 192.168.12.0/24
        existingNetworkID: <EXISTING_NETWORK_ID>
        zones:
          - ru-central1-a
          - ru-central1-b
          - ru-central1-d
        labels:
          billing: prod
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - <DNS_SERVER_1>
            - <DNS_SERVER_2>
```

### WithoutNAT

In this layout, no form of NAT is used, and each node is assigned a public IP address.

{% alert level="warning" %}
DP does not assign security groups to the nodes, so in this layout the nodes are accessible from the outside without restrictions. To restrict the traffic, use the rules of the default group as described in ["Configuring security groups in Yandex Cloud"](#configuring-security-groups-in-yandex-cloud).
{% endalert %}

![WithoutNAT layout in Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-withoutnat.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10557&t=Qb5yyWumzPiTBtfL-0 --->

Example layout configuration:

```yaml
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
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: WithoutNAT
        sshPublicKey: <SSH_PUBLIC_KEY>
        nodeNetworkCIDR: 192.168.12.0/24
        existingNetworkID: <EXISTING_NETWORK_ID>
        zones:
          - ru-central1-a
          - ru-central1-b
          - ru-central1-d
        externalIPAddresses:
          master:
            - Auto
            - Auto
            - Auto
          worker:
            - <ZONE_A_EXTERNAL_IP_WORKER_1>
            - Auto
        externalSubnetIDs:
          master:
            - <ZONE_A_SUBNET_ID>
            - <ZONE_B_SUBNET_ID>
            - <ZONE_D_SUBNET_ID>
          worker:
            - <ZONE_A_SUBNET_ID>
            - <ZONE_B_SUBNET_ID>
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - <DNS_SERVER_1>
            - <DNS_SERVER_2>
```

### WithNATInstance

In this layout, a NAT instance is created in a separate subnet,
and the route tables of the zone subnets are updated with a `0.0.0.0/0` route using the NAT instance as the next hop.
The subnet is isolated to prevent routing loops and must not overlap with any other networks used in the cluster.

To place the NAT instance in an existing subnet, use the [`withNATInstance.internalSubnetID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetid) parameter.
The instance will be created in the zone corresponding to this subnet.

If you need to create a new subnet, specify the [`withNATInstance.internalSubnetCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetcidr) parameter.
The NAT instance will be placed in this subnet.

Set one of the two parameters, `withNATInstance.internalSubnetID` or `withNATInstance.internalSubnetCIDR`.

If [`withNATInstance.externalSubnetID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-externalsubnetid) is also specified,
the NAT instance will be connected to that subnet via a secondary network interface.

![WithNATInstance layout in Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-withnatinstance.png)
<!--- Source: https://docs.google.com/drawings/d/1oVpZ_ldcuNxPnGCkx0dRtcAdL7BSEEvmsvbG8Aif1pE/edit --->

Example layout configuration:

```yaml
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
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: WithNATInstance
        sshPublicKey: <SSH_PUBLIC_KEY>
        nodeNetworkCIDR: 192.168.12.0/24
        existingNetworkID: <EXISTING_NETWORK_ID>
        zones:
          - ru-central1-a
        withNATInstance:
          natInstanceExternalAddress: <NAT_INSTANCE_EXTERNAL_ADDRESS>
          internalSubnetID: <INTERNAL_SUBNET_ID>
          externalSubnetID: <EXTERNAL_SUBNET_ID>
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - <DNS_SERVER_1>
            - <DNS_SERVER_2>
```

{% alert level="info" %}
In this example, master and worker nodes are created without public IP addresses and without additional external network interfaces. Outbound traffic from the cluster subnets goes through the NAT instance.

The NAT instance is not automatically used by the `dhctl` installer as a bastion host or jump host. To install the cluster, the master node must be accessible over SSH from the machine where `dhctl` is running: either directly or through a bastion host.

If the master node does not have a public IP address or is not directly accessible, run the installation with the `--ssh-bastion-host` and `--ssh-bastion-user` parameters.

If you need to connect directly to a master node using a public IP address, specify the [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) and [`externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids) parameters in the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module. Note that the interface connected to the network specified in `externalSubnetIDs` will be used as the node's default gateway.
{% endalert %}

## Configuration

The Yandex Cloud provider parameters are defined in four resources:

- The [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module defines the [layout](#layouts), the [network parameters](#network-parameters-and-security), and the cloud and folder IDs.
- The `d8-credentials` Secret in the `d8-cloud-provider-yandex` namespace stores the service account key. Its format is described in ["Credentials Secret"](authorization.html#credentials-secret).
- [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) describes the virtual machine parameters.
- [NodeGroup](/modules/node-manager/cr.html#nodegroup) describes the node groups and refers to an instance class. The master node group is described by the same resource with the [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) type.

To change the provider settings in a running cluster, edit the ModuleConfig:

```shell
d8 k edit mc cloud-provider-yandex
```

After changing the node parameters, run the following command:

```shell
dhctl converge
```

{% alert level="warning" %}
If the cluster was installed with the [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) schema, it must be migrated to the ModuleConfig-based configuration. Until the migration is complete, the [`D8CloudProviderYandexCloudMigrationPending`](../../../../reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending) alert fires and a Deckhouse Platform update can be blocked.

For instructions, see ["How to migrate a cloud provider to ModuleConfig-based configuration?"](../../../../faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat).

If the module settings had the `storageClass.default` parameter, set the default StorageClass with the global [`defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) parameter after the migration. The conversion does not carry this value over.
{% endalert %}

### Configuration example

Below is a minimal configuration of a cluster with the Standard layout, one master node, and one group of worker nodes:

```yaml
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
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 10.241.32.0/20
        sshPublicKey: <SSH_PUBLIC_KEY>
        labels:
          billing: prod
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: master
spec:
  cores: 4
  memory: 8192
  imageID: <IMAGE_ID>
  etcdDiskSizeGB: 10
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: master
    minPerZone: 1
    maxPerZone: 1
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
spec:
  cores: 4
  memory: 8192
  coreFraction: 50
  imageID: <IMAGE_ID>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: worker
    minPerZone: 1
    maxPerZone: 3
    zones:
      - ru-central1-a
```

Where:

- `CLOUD_ID` and `FOLDER_ID` are the Yandex Cloud cloud and folder identifiers, specified by the [`cloudID`](/modules/cloud-provider-yandex/configuration.html#parameters-provider-parameters-cloudid) and [`folderID`](/modules/cloud-provider-yandex/configuration.html#parameters-provider-parameters-folderid) parameters in the `provider.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module. For instructions on obtaining these identifiers, see ["Connecting and authorizing in Yandex Cloud"](authorization.html).
- `SSH_PUBLIC_KEY` is the public SSH key used to access nodes, specified by the [`sshPublicKey`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-sshpublickey) parameter in the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.
- `IMAGE_ID` is the operating system image ID used to create virtual machines, specified by the [`imageID`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-imageid) parameter of the YandexInstanceClass resource. If the parameter is not set, the image of the `master` node group instance class is used.

The YandexInstanceClass resource for the `master` node group must specify the [`etcdDiskSizeGB`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-etcddisksizegb) parameter, which defines the size of a separate disk for etcd. This parameter must not be specified for other node groups.

The rest of the parameters are described in the [module settings](/modules/cloud-provider-yandex/configuration.html) and in the [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) resource description.

The service account key is not stored in ModuleConfig. It is defined in a [separate Secret](authorization.html#credentials-secret).

## Network parameters and security

This section describes the settings related to addressing, routing, external traffic, and network security
in a DP cluster deployed in Yandex Cloud.

### Internal node addressing

The [`nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) parameter defines the IP address range that is split between availability zones
and applied to the internal node interfaces. The parameter belongs to the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        nodeNetworkCIDR: 192.168.12.0/24
```

DP divides this subnet into four equal parts and creates one internal subnet from each of them in every availability zone:

- ru-central1-a
- ru-central1-b
- ru-central1-e
- ru-central1-d

The nodes of the matching zone are connected to the created subnet. The division happens in every layout. When the [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) parameter is set, DP creates no subnets and uses the ones listed there.

{% alert level="info" %}
A subnet accepts only one route table, so two clusters with the [`cni-simple-bridge`](/modules/cni-simple-bridge/) module cannot use the same subnets.
To deploy such clusters in shared subnets, use the [`cni-cilium`](/modules/cni-cilium/) module.
{% endalert %}

### Assigning external IP addresses and outgoing traffic

The [`externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids) parameter belongs to the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module and is set per node group:

- the key is the node group name;
- the value is the list of Yandex Cloud subnet IDs that the external network interfaces of that group's nodes are attached to, in the order of the zones.

The parameter is required when you need to:

- assign a public IP address;
- define a default route for outgoing traffic from nodes;
- use the `Auto` value in the [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) parameter.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        externalSubnetIDs:
          worker:
            - <RU_CENTRAL1_A_SUBNET_ID>
            - <RU_CENTRAL1_B_SUBNET_ID>
            - <RU_CENTRAL1_D_SUBNET_ID>
```

The [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) parameter in the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module specifies a list of external IP addresses for each node group in the order of their availability zones.

### DNS and DHCP settings for internal networks

The [`dhcpOptions`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-dhcpoptions) parameter sets the DHCP server configuration
applied to all subnets created within the DP cluster in Yandex Cloud. The parameter belongs to the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.

Available fields:

- `domainName`: The search domain set in the network configuration.
- `domainNameServers`: An array of IP addresses of DNS servers used as recursive resolvers.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - 192.168.0.2
            - 192.168.0.3
```

{% alert level="info" %}
The DNS servers listed in `domainNameServers` must resolve both external and internal domain zones used by the cluster.
Otherwise, service disruptions may occur.
{% endalert %}

After updating `dhcpOptions`:

- force renew the DHCP lease on all VMs;
- restart the components running in the node network (`hostNetwork: true`), including `kube-dns`, to refresh the `resolv.conf` contents.

You can apply changes using the following command:

```shell
netplan apply
```

or another method used in your system, for example `systemd-networkd` or `dhclient`.

### Using pre-existing subnets

The [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) parameter maps
availability zones to pre-created subnets in Yandex Cloud.
Set it when DP has to use existing subnets instead of creating new ones. The parameter belongs to the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        existingZoneToSubnetIDMap:
          ru-central1-a: e2lu8r1tbbtryhdpa9ro
          ru-central1-b: e2lu8r1tbbtryhdpa9ro
          ru-central1-d: e2lu8r1tbbtryhdpa9ro
```

{% alert level="info" %}
DP creates a route table but does not associate it with the subnets. Associate the table manually in the Yandex Cloud interface.
{% endalert %}

### Additional external networks

The [`additionalExternalNetworkIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-ccm-parameters-additionalexternalnetworkids) parameter defines a list of additional external networks whose addresses DP treats as public. The parameter belongs to the `ccm.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.

This is useful when:

- There are external subnets not listed in `externalSubnetIDs` but that should be treated as external.
- You need precise control over how IP addresses are interpreted for load balancing, routing, and status export.
- You use custom network configurations (for example, external NAT gateways or manually reserved IPs).

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    ccm:
      parameters:
        additionalExternalNetworkIDs:
          - enp6t4sno
```

If this parameter is not set, DP will only use the subnets explicitly specified in the module settings
(for example, via `externalSubnetIDs`) to determine whether an IP is public.

## Configuring security groups in Yandex Cloud

For every [cloud network](https://yandex.cloud/en/docs/vpc/concepts/network#network), Yandex Cloud creates a default [security group](https://yandex.cloud/en/docs/vpc/concepts/security-groups), including for the cluster network. That group allows any inbound and outbound traffic and applies to the virtual machine network interfaces that are not assigned another group.

You can restrict the cluster traffic with the rules of that group. Assigning another security group to the nodes is not possible, as neither the module settings nor the YandexInstanceClass resource have parameters for that.

{% alert level="warning" %}
Do not assign security groups to the nodes manually in the Yandex Cloud console. The `dhctl converge` command brings the virtual machines to the state described in the cluster configuration and may remove such an assignment.
{% endalert %}

{% alert level="danger" %}
Do not delete the rules that allow any traffic until you have configured the other rules of the group. Otherwise, the group blocks the traffic between the nodes and the cluster components lose connection to each other.
{% endalert %}

Errors in the rules disrupt the cluster. Before configuring the rules in a production cluster, consult the [security group usage details](https://yandex.cloud/en/docs/vpc/concepts/security-groups#security-groups-notes) in Yandex Cloud.

If the cluster uses Cilium with pod traffic tunneling over VXLAN, allow UDP traffic between nodes on the ports listed in ["Network interaction of the platform components"](../../../../reference/network_interaction.html).

To configure the rules of the security group:

1. Identify the cloud network used by the DP cluster.

   The network name matches the [`prefix`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-prefix) parameter of the ModuleConfig `global`.
   You can retrieve it using the following command:

   ```bash
   d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
   ```

   If the value is empty, get it from the deprecated [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) parameter of ClusterConfiguration (the prefix has moved to the ModuleConfig `global`):

   ```bash
   d8 k get secrets -n kube-system d8-cluster-configuration -ojson | \
     jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix | cut -d: -f2
   ```

1. In the Yandex Cloud Console, go to the Virtual Private Cloud service and open the "Security Groups" section. You should see a security group marked `Default`.

   ![The default security group](../../../../images/cloud-provider-yandex/sg-en-default.png)

1. Create rules according to the [Yandex Cloud instructions](https://yandex.cloud/en/docs/managed-kubernetes/operations/connect/security-groups#rules-internal).

   ![Rules for the security group](../../../../images/cloud-provider-yandex/sg-en-rules.png)

1. Delete the rule that allows all **inbound** traffic (it is already deleted in the screenshot above) and save your changes.

## Configuring access via bastion host

To connect to nodes located in private subnets (for example, when using the Standard or WithNATInstance layout),
you can use a bastion host, an intermediary VM with a public IP address that provides SSH access to the nodes.

To configure access, follow these steps:

1. Bootstrap the base infrastructure.
   Before creating a bastion host, perform the initial installation phase of DP, which sets up the network infrastructure:

   ```shell
   dhctl bootstrap-phase base-infra --config config.yml
   ```

1. Create a bastion host in Yandex Cloud:

   ```shell
   yc compute instance create \
     --name bastion \
     --hostname bastion \
     --create-boot-disk image-family=ubuntu-2204-lts,image-folder-id=standard-images,size=20,type=network-hdd \
     --memory 2 \
     --cores 2 \
     --core-fraction 100 \
     --ssh-key ~/.ssh/<SSH_PUBLIC_KEY_FILE> \
     --zone ru-central1-a \
     --public-address 178.154.226.159
   ```

   Replace `<SSH_PUBLIC_KEY_FILE>` with the name of the public key, for example `id_rsa.pub` for an RSA key or `id_ed25519.pub` for an ED25519 key.

   Ensure the IP address specified in `--public-address` is accessible from your network and is correct.

1. Start the main DP bootstrap through the bastion host:

   ```shell
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
     --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> --config=/config.yml
   ```

   Replace `<SSH_PRIVATE_KEY_FILE>` with the name of the private key, for example `id_rsa` for an RSA key or `id_ed25519` for an ED25519 key.

   Where:

   - `--ssh-bastion-user`: User for connecting to the bastion host.
   - `--ssh-user`: User on the target cluster nodes.
   - `--ssh-agent-private-keys`: Path to the private SSH key.
   - `--config`: Path to the DP configuration file.
