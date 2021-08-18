---
title: "Cloud provider — AWS: FAQ"
---

## How do I create a peering connection between VPCs?

Let's, for example, create a peering connection between two VPCs, vpc-a and vpc-b.

**Caution!**
IPv4 CIDR must be unique for each VPC.

* Switch to the region where vpc-a is running.
* VPC -> VPC Peering Connections -> Create Peering Connection, configure a peering connection:

  * Name: vpc-a-vpc-b
  * Fill in Local and Another VPC fields.

* Switch to the region where vpc-b is running.
* VPC -> VPC Peering Connections.
* Select the newly created perring connection and click Action "Accept Request".
* Add routes to vpc-b's CIDR over a peering connection to the vpc-a's routing tables.
* Add routes to vpc-a's CIDR over a peering connection to the vpc-b's routing tables.


## How do I create a cluster in a new VPC with access over an existing bastion host?

* Bootstrap the base-infrastructure of the cluster:

  ```shell
  dhctl bootstrap-phase base-infra --config config
  ```

* Set up a peering connection using the instructions [above](#how-do-i-create-a-peering-connection-between-vpcs).
* Continue installing the cluster, enter "y" when asked about the terraform cache:

  ```shell
  dhctl bootstrap --config config --ssh-...
  ```

## How do I create a cluster in a new VPC and set up bastion host to access the nodes?

* Bootstrap the base-infrastructure of the cluster:

  ```shell
  dhctl bootstrap-phase base-infra --config config
  ```

* Manually set up the bastion host in the subnet <prefix>-public-0.
* Continue installing the cluster, enter "y" when asked about the terraform cache:

  ```shell
  dhctl bootstrap --config config --ssh-...
  ```

## Configuring a bastion host

There are two possible cases:
* a bastion host already exists in an external VPC; in this case, you need to:
  * Create a basic infrastructure: `dhctl bootstrap-phase base-infra`;
  * Set up peering connection between an external and a newly created VPC;
  * Continue the installation by specifying the bastion: `dhctl bootstrap --ssh-bastion...`
* a bastion host needs to be deployed to a newly created VPC; in this case, you need to:
  * Create a basic infrastructure: `dhctl bootstrap-phase base-infra`;
  * Manually run a bastion in the <prefix>-public-0 subnet;
  * Continue the installation by specifying the bastion: `dhctl bootstrap --ssh-bastion...`
