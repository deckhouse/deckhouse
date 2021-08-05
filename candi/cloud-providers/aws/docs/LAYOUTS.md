---
title: "Cloud provider — AWS: Layouts"
---

## WithoutNAT

**Recommended layout.**

Under this placement strategy, each node gets a public IP (ElasticIP). NAT is not used at all.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQDR2iRcFO3Ra3hmdrYCuoHPP6m3DCArtZjmbQGMJL00xmR-F94IMJKx2jKqeiwe-KvbykqtCEjsR9c/pub?w=812&h=655)
<!--- source : https://docs.google.com/drawings/d/1JDmeSY12EoZ3zBfanEDY-QvSgLekzw6Tzjj2pgY8giM/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithoutNAT
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  region: eu-central-1
masterNodeGroup:
  replicas: 1
  instanceClass:
    instanceType: m5.xlarge
    ami: ami-03818140b4ac9ae2b
nodeGroups:
  - name: mydb
    nodeTemplate:
      labels:
        node-role.kubernetes.io/mydb: ""
    replicas: 2
    instanceClass:
      instanceType: t2.medium
      ami: ami-03818140b4ac9ae2b
    additionalTags:
      backup: me
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
tags:
  team: rangers
```

## Standard

**Caution!** A bastion host is required to access nodes.

Virtual machines access the Internet using a NAT Gateway with a shared (and single) source IP.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSkzOWvLzAwB4hmIk4CP1-mj2QIxCyJg2VJvijFfdttjnV0quLpw7x87KtTC5v2I9xF5gVKpTK-aqyz/pub?w=812&h=655)
<!-- Source: https://docs.google.com/drawings/d/1kln-DJGFldcr6gayVtFYn_3S50HFIO1PLTc1pC_b3L0/edit -->

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: Standard
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  region: eu-central-1
masterNodeGroup:
  # Number of master nodes
  # If there is more than one master node, the etcd cluster will be set up automatically.
  replicas: 1
  instanceClass:
    instanceType: m5.xlarge
    ami: ami-03818140b4ac9ae2b
nodeGroups:
  - name: mydb
    nodeTemplate:
      labels:
        node-role.kubernetes.io/mydb: ""
    replicas: 2
    instanceClass:
      instanceType: t2.medium
      ami: ami-03818140b4ac9ae2b
    additionalTags:
      backup: me
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
tags:
  team: rangers
```
