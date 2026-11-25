---
title: Network configuration in Yandex Cloud
permalink: en/admin/integrations/public/yandex/network.html
lang: en
---

## Network parameters and security

This section describes the settings related to addressing, routing, external traffic, and network security
in a Deckhouse Platform cluster deployed in Yandex Cloud.

### Internal node addressing

The [`nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) parameter defines the IP address range that is split between [availability zones](../../../../glossary.html#availability-zone)
and applied to the internal node interfaces. The parameter belongs to the `nodes.parameters` section of the [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module:

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

When creating subnets automatically, Deckhouse Platform divides [`nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) into four equal parts for internal node subnets. Each default zone has a fixed part of the range:

| Zone | Part of the range |
| --- | --- |
| `ru-central1-a` | First |
| `ru-central1-b` | Second |
| `ru-central1-e` | Third |
| `ru-central1-d` | Fourth |

You can explicitly list one to four supported zones in [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones). When creating subnets automatically, Deckhouse Platform creates or keeps the subnets in all four default zones if [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) is unset or empty, or contains only default zones.

When installing a cluster or changing its configuration, you can select only `ru-central1-f`, without any default zones. For zone selection rules and a configuration example, see ["Selecting zones for the cluster"](/modules/cloud-provider-yandex/examples.html#selecting-zones-for-the-cluster).

Before changing zones, review the effects, complete the preparation, and select the instructions for your cluster configuration method in ["Changing the zone set in a running cluster"](/modules/cloud-provider-yandex/examples.html#changing-the-zone-set-in-a-running-cluster). Check quotas in every zone in the temporary and final node group zone lists, including zones already in use.

{% alert level="warning" %}
From December 16, 2026, Yandex Cloud restricts creating and modifying virtual machines in the `ru-central1-b` zone. Before this date, remove the zone from node group configurations and migrate the required data to other zones. Otherwise, Deckhouse Platform will be unable to create or recreate nodes in `ru-central1-b`. Step-by-step examples are provided in ["Decommissioning the ru-central1-b zone"](/modules/cloud-provider-yandex/examples.html#decommissioning-the-ru-central1-b-zone).
{% endalert %}

Nodes connect to the subnet in their zone. When subnets are created automatically, the range is divided into four parts in every [layout](../../../../glossary.html#layout). If [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) contains entries, Deckhouse Platform uses the specified subnets but does not manage them. In this case, leaving [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) unset or empty uses the zones from [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Keep at least one entry in this parameter; otherwise, Deckhouse Platform switches to automatic subnet creation.

{% alert level="info" %}
A subnet accepts only one route table, so two clusters with the [`cni-simple-bridge`](/modules/cni-simple-bridge/) module cannot use the same subnets.
To deploy such clusters in shared subnets, use the [`cni-cilium`](/modules/cni-cilium/) module.
{% endalert %}

### Assigning external IP addresses and outgoing traffic

The [`externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids) parameter belongs to the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module and is set per [node group](../../../../glossary.html#node-group):

- the key is the node group name;
- the value is a list of Yandex Cloud subnet IDs for external network interfaces of CloudPermanent nodes. Each node's subnet must match its zone.

Set this parameter if CloudPermanent nodes need additional external network interfaces.
If it is unset, [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) can assign public Yandex Cloud IP addresses.

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

The [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) parameter in the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module specifies a list of external IP addresses for each CloudPermanent node group. The address from each list entry is used by the node with the corresponding index.

If you need to connect directly to a master node using a public IP address, specify the [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) and [`externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids) parameters in the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module. Note that the interface connected to the network specified in `externalSubnetIDs` will be used as the node's default gateway.

### DNS and DHCP settings for internal networks

The [`dhcpOptions`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-dhcpoptions) parameter sets the DHCP server configuration
applied to all subnets created within the Deckhouse Platform cluster in Yandex Cloud. The parameter belongs to the `nodes.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.

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

If you have created subnets in Yandex Cloud and want to use them for cluster nodes, map their zones to subnet IDs in [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Deckhouse Platform uses these subnets but does not manage them. You can specify only one subnet ID per zone.

The following ModuleConfig fragment specifies five existing subnets, while [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) restricts the cluster-wide list to `ru-central1-a` and `ru-central1-f`. Keep the other configuration parameters:

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
          ru-central1-a: <RU_CENTRAL1_A_SUBNET_ID>
          ru-central1-b: <RU_CENTRAL1_B_SUBNET_ID>
          ru-central1-d: <RU_CENTRAL1_D_SUBNET_ID>
          ru-central1-e: <RU_CENTRAL1_E_SUBNET_ID>
          ru-central1-f: <RU_CENTRAL1_F_SUBNET_ID>
        zones:
          - ru-central1-a
          - ru-central1-f
```

Replace `<RU_CENTRAL1_A_SUBNET_ID>`, `<RU_CENTRAL1_B_SUBNET_ID>`, `<RU_CENTRAL1_D_SUBNET_ID>`, `<RU_CENTRAL1_E_SUBNET_ID>`, and `<RU_CENTRAL1_F_SUBNET_ID>` with subnet IDs from the corresponding zones.

The `ru-central1-b`, `ru-central1-d`, and `ru-central1-e` subnets remain in [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) and are kept in Yandex Cloud. In this configuration, their zones are outside the cluster-wide zone list for node placement.

To place new nodes in one of these subnets, include its zone in [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones), keeping at most four zones in the list. If the relevant node groups have explicitly configured zone lists, add the same zone to those lists.

If [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) is unset or empty, the cluster-wide list includes all five zones from [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).

For adding subnets and configuring node group zone lists, see ["Using existing subnets"](/modules/cloud-provider-yandex/examples.html#using-existing-subnets).

{% alert level="info" %}
Deckhouse Platform creates a route table but does not associate it with the subnets. Associate the table manually in the Yandex Cloud interface.
{% endalert %}

### Additional external networks

The [`additionalExternalNetworkIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-ccm-parameters-additionalexternalnetworkids) parameter defines a list of additional external networks whose addresses Deckhouse Platform treats as public. The parameter belongs to the `ccm.parameters` section of the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.

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

If this parameter is not set, Deckhouse Platform will only use the subnets explicitly specified in the module settings
(for example, via `externalSubnetIDs`) to determine whether an IP is public.

## Configuring the NAT instance

To place the NAT instance in an existing subnet, use the [`withNATInstance.internalSubnetID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetid) parameter.
The instance will be created in the zone corresponding to this subnet.

If you need to create a new subnet, specify the [`withNATInstance.internalSubnetCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetcidr) parameter.
The NAT instance will be placed in this subnet.

Set one of the two parameters, `withNATInstance.internalSubnetID` or `withNATInstance.internalSubnetCIDR`.

If [`withNATInstance.externalSubnetID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-externalsubnetid) is also specified,
the NAT instance will be connected to that subnet via a secondary network interface.

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

If the cluster uses Cilium with pod traffic tunneling over VXLAN, allow UDP traffic between nodes on the ports listed in ["Network interaction of Deckhouse Platform components"](../../../../reference/network_interaction.html).

To configure the rules of the security group:

1. Identify the cloud network used by the Deckhouse Platform cluster.

   The network name matches the [`prefix`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-prefix) parameter of the ModuleConfig `global`.
   You can retrieve it using the following command:

   ```bash
   d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
   ```

   If the value is empty, get it from the deprecated [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) parameter of [ClusterConfiguration](../../../../glossary.html#cluster-configuration) (the prefix has moved to the ModuleConfig `global`):

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

To connect to nodes located in private subnets (for example, when using the Standard or WithNATInstance [layout](../../../../glossary.html#layout)),
you can use a [bastion host](../../../../glossary.html#bastion-host), an intermediary VM with a public IP address that provides SSH access to the nodes.

The NAT instance is not automatically used by the `dhctl` [installer](../../../../glossary.html#installer) as a bastion host or jump host. To install the cluster, the master node must be accessible over SSH from the machine where `dhctl` is running: either directly or through a bastion host.

If the master node is not directly accessible, run the installation with the `--ssh-bastion-host`, `--ssh-bastion-user`, and, if needed, `--ssh-bastion-port` parameters.

To configure access, follow these steps:

1. Bootstrap the base infrastructure.
   Before creating a bastion host, perform the initial installation phase of Deckhouse Platform, which sets up the network infrastructure:

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

1. Start the main Deckhouse Platform bootstrap through the bastion host:

   ```shell
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
     --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> --config=/config.yml
   ```

   Replace `<SSH_PRIVATE_KEY_FILE>` with the name of the private key, for example `id_rsa` for an RSA key or `id_ed25519` for an ED25519 key.

   Where:

   - `--ssh-bastion-user`: User for connecting to the bastion host.
   - `--ssh-user`: User on the target cluster nodes.
   - `--ssh-agent-private-keys`: Path to the private SSH key.
   - `--config`: Path to the Deckhouse Platform configuration file.
