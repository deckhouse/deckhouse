---
title: "Cloud provider — AWS: Layouts"
---

## Layouts
### WithoutNAT

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

### Standard

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

## AWSClusterConfiguration
A particular placement strategy is defined using the `AWSClusterConfiguration` struct. It has the following fields:
* `layout` — the name of the layout.
  * Possible values: `WithoutNAT` or `Standard` (see the description below).
* `provider` — parameters for connecting to the AWS API.
  * `providerAccessKeyId` — access key [ID](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys).
  * `providerSecretAccessKey` — access key [secret](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys).
  * `region` — the name of the AWS region where instances will be provisioned;
* `masterNodeGroup` — parameters of the master's NodeGroup;
  * `replicas` — the number of master nodes to create;
  * `instanceClass` — partial contents of the [AWSInstanceClass](../../modules/030-cloud-provider-aws/#awsinstanceclass-custom-resource) CR. Possible values:
    * `instanceType`
    * `ami`
    * `additionalSecurityGroups`
    * `diskType`
    * `diskSizeGb`
  * `zones` — a limited set of zones in which master nodes can be createed. An optional parameter;
  * `additionalTags` — tags to attach to instances being created in addition to the main ones (`AWSClusterConfiguration.tags`);
* `nodeGroups` — an array of additional NodeGroups for creating static nodes (e.g., for dedicated front nodes or gateways). Each NodeGroup has the following parameters:
  * `name` — the name of the NodeGroup; it is used to generate the node name;
  * `replicas` — the number of nodes;
  * `instanceClass` — partial contents of the [AWSInstanceClass]({{"/modules/030-cloud-provider-aws/#awsinstanceclass-custom-resource" | true_relative_url }} ) CR. Possible values:
    * `instanceType`
    * `ami`
    * `additionalSecurityGroups`
    * `diskType`
    * `diskSizeGb`
  * `zones` — a limited set of zones in which nodes can be createed. An optional parameter;
  * `additionalTags` — tags to attach to instances being created in addition to the main ones (`AWSClusterConfiguration.tags`);
  * `nodeTemplate` — parameters of Node objects in Kubernetes to add after registering the node;
    * `labels` — the same as the `metadata.labels` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
      * An example:

        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```

    * `annotations` — the same as the `metadata.annotations` [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
      * An example:

        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```

    * `taints` — the same as the `.spec.taints` field of the [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core) object. **Caution!** Only the `effect`, `key`, `values` are available.
      * An example:

        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```

* `vpcNetworkCIDR` — a subnet to use in the VPC being created;
  * a mandatory parameter if the `existingVPCID` parameter is omitted (see below);
* `existingVPCID` — ID of the existing VPC to use for deploying;
  * A mandatory parameter if the `vpcNetworkCIDR` is omitted;
  * **Caution!** If there is an Internet Gateway in the target VPC, the deployment of the basic infrastructure will fail with an error. Currently, an Internet Gateway cannot be adopted;
* `nodeNetworkCIDR` — a subnet to use for cluster nodes;
  * The IP range must overlap or match the VPC address range;
  * The IP range will be evenly split into subnets, one per Availability Zone in your region;
  * An optional but recommended parameter. By default, it corresponds to the whole range of VPC addresses;
> If a new VPC is created along with a new cluster and no `vpcNetworkCIDR` is provided, then the range from  `nodeNetworkCIDR` is used for the VPC.
> Thus, the entire VPC is allocated for the cluster networks, and you will not be able to add other resources to this VPC.
>
> The `nodeNetworkCIDR` range is distributed between subnets depending on the number of availability zones in the selected region. For example:
> if `nodeNetworkCIDR: "10.241.1.0/20"` and there are three availability zones in the region, subnets will be created with the `/22` mask.
* `sshPublicKey` — a public key for accessing nodes;
* `tags` — tags to attach to newly created resources. You have to re-create all the machines to add new tags if tags were modified in the running cluster;
* `zones` — a limited set of zones in which nodes can be created.
  * An optional parameter;
  * Format — an array of strings;
