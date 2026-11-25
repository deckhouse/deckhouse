---
title: Layouts in Amazon Web Services
permalink: en/admin/integrations/public/amazon/layout.html
lang: en
---

This section describes cluster [layouts](../../../../glossary.html#layout) in AWS infrastructure and their related parameters.
The selected layout affects how public IP addresses are assigned, NAT usage, and how nodes are accessed.

## WithoutNAT

This is the recommended layout.

Each node is assigned a public IP address (Elastic IP), and no NAT Gateway is used.
This layout allows direct access to nodes via public IP addresses and simplifies outgoing traffic routing.

![WithoutNAT layout in AWS](../../../../images/cloud-provider-aws/aws-withoutnat.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10681&t=Qb5yyWumzPiTBtfL-0 --->

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithoutNAT
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: <SSH_PUBLIC_KEY>
provider:
  providerAccessKeyId: '<AWS_ACCESS_KEY>'
  providerSecretAccessKey: '<AWS_SECRET_ACCESS_KEY>'
  region: eu-central-1
masterNodeGroup:
  # Number of master nodes.
  # If you specify more than one master node, an etcd cluster will be created automatically.
  replicas: 1
  instanceClass:
    # Instance type to use.
    instanceType: m5.xlarge
    # ID of the Amazon Machine Image (AMI).
    # AMI catalog can be found in the AWS Console: EC2 → AMI Catalog.
    ami: ami-0aad10862ade98f27
    # Disk size for the master node VM.
    diskSizeGb: 30
    # Disk type to use for the master node VM.
    diskType: gp3
nodeGroups:
  - name: mydb
    nodeTemplate:
      labels:
        node-role.kubernetes.io/mydb: ""
    replicas: 2
    instanceClass:
      instanceType: t2.medium
      ami: ami-0aad10862ade98f27
    additionalTags:
      backup: srv1
tags:
  team: torpedo
```

## WithNAT

{% alert level="warning" %}
In this layout, the NAT Gateway is always created in zone `a`.
If cluster nodes are placed in other zones, a failure in zone a may cause the cluster to become unavailable.
A [bastion host](../../../../glossary.html#bastion-host) is required to connect to the nodes.
{% endalert %}

In this [layout](../../../../glossary.html#layout), a NAT Gateway is used for accessing the internet, and nodes do not receive public IP addresses.
Access to nodes is only possible through a bastion host, which is placed in a separate subnet.

![WithNAT layout in AWS](../../../../images/cloud-provider-aws/aws-withnat.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-9864&t=Qb5yyWumzPiTBtfL-0 --->

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithNAT
provider:
  providerAccessKeyId: '<AWS_ACCESS_KEY>'
  providerSecretAccessKey: '<AWS_SECRET_ACCESS_KEY>'
  region: eu-central-1
withNAT:
  bastionInstance:
    zone: eu-central-1a
    instanceClass:
      instanceType: m5.large
      ami: ami-0aad10862ade98f27
      diskType: gp3
masterNodeGroup:
  # Number of master nodes.
  # If you specify more than one master node, an etcd cluster will be created automatically.
  replicas: 1
  instanceClass:
    # Instance type to use.
    instanceType: m5.xlarge
    # ID of the Amazon Machine Image (AMI).
    # AMI catalog can be found in the AWS Console: EC2 → AMI Catalog.
    ami: ami-0aad10862ade98f27
    # Disk size for the master node VM.
    diskSizeGb: 30
    # Disk type to use for the master node VM.
    diskType: gp3
nodeGroups:
  - name: mydb
    nodeTemplate:
      labels:
        node-role.kubernetes.io/mydb: ""
    replicas: 2
    instanceClass:
      instanceType: t2.medium
      ami: ami-0aad10862ade98f27
    additionalTags:
      backup: me
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: "<SSH_PUBLIC_KEY>"
tags:
  team: torpedo
```

[Integration configuration](configuration.html).
[Network configuration](network.html).
