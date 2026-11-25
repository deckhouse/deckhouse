---
title: Layouts in Yandex Cloud
permalink: en/admin/integrations/public/yandex/layout.html
lang: en
---

## Layouts

This section describes the possible node placement [layouts](../../../../glossary.html#layout) in Yandex Cloud infrastructure and the related configuration options. The layout is specified by the [`layout`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-layout) parameter in the ModuleConfig resource of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module.
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
Because nodes are created without public IP addresses in this layout, the [master node](../../../../glossary.html#master-node) must be accessible over SSH from the machine where `dhctl` is running: either directly over a private network or through a [bastion host](../../../../glossary.html#bastion-host).
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
Deckhouse Platform does not assign security groups to the nodes, so in this layout the nodes are accessible from the outside without restrictions. To restrict the traffic, use the rules of the default group as described in ["Configuring security groups in Yandex Cloud"](network.html#configuring-security-groups-in-yandex-cloud).
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
In this example, master and [worker nodes](../../../../glossary.html#worker-node) are created without public IP addresses and without additional external network interfaces. Outbound traffic from the cluster subnets goes through the NAT instance.
{% endalert %}

[Integration configuration](configuration.html).
[Network configuration](network.html).
