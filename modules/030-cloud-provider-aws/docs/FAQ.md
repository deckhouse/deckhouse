---
title: "Cloud provider — AWS: FAQ"
---

## How do I create a peering connection between VPCs?

Let's, for example, create a peering connection between two VPCs, vpc-a and vpc-b.

> **Caution!** IPv4 CIDR must be unique for each VPC.

To configure, follow these steps:

1. Switch to the region where vpc-a is running.
1. Click `VPC` -> `VPC Peering Connections` -> `Create Peering Connection` and then configure a peering connection:
   * Name: `vpc-a-vpc-b`.
   * Fill in `Local` and `Another VPC` fields.
1. Switch to the region where vpc-b is running.
1. Click `VPC` -> `VPC Peering Connections`.
1. Select the newly created perring connection and click `Action "Accept Request"`.
1. Add routes to vpc-b's CIDR over a peering connection to the vpc-a's routing tables.
1. Add routes to vpc-a's CIDR over a peering connection to the vpc-b's routing tables.

## How do I create a cluster in a new VPC with access over an existing bastion host?

1. Bootstrap the base-infrastructure of the cluster:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

2. Set up a peering connection using the instructions [above](#how-do-i-create-a-peering-connection-between-vpcs).

3. Continue installing the cluster, enter `y` when asked about the Terraform cache:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

## How do I create a cluster in a new VPC and set up bastion host to access the nodes?

1. Bootstrap the base-infrastructure of the cluster:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

2. Manually set up the bastion host in the subnet <prefix>-public-0.

3. Continue installing the cluster, enter `y` when asked about the Terraform cache:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

## Configuring a bastion host

There are two possible cases:
* A bastion host already exists in an external VPC; in this case, you need to:
  1. Create a basic infrastructure of the cluster: `dhctl bootstrap-phase base-infra`;
  1. Set up peering connection between an external and a newly created VPC;
  1. Continue the installation by specifying the bastion host: `dhctl bootstrap --ssh-bastion...`
* A bastion host needs to be deployed to a newly created VPC; in this case, you need to:
  1. Create a basic infrastructure of the cluster: `dhctl bootstrap-phase base-infra`;
  1. Manually run a bastion in the <prefix>-public-0 subnet;
  1. Continue the installation by specifying the bastion host: `dhctl bootstrap --ssh-bastion...`

## Adding CloudStatic nodes to a cluster

To add a pre-created VM as a node to a cluster, follow these steps:

1. Attach a security group `<prefix>-node` to the virtual machine.
1. Attach the IAM role `<prefix>-node` to the virtual machine.
1. Add the following tags to the virtual machine (so that `cloud-controller-manager` can find virtual machines in the cloud):

   ```text
   "kubernetes.io/cluster/<cluster_uuid>" = "shared"
   "kubernetes.io/cluster/<prefix>" = "shared"
   ```

   * You can find out the `cluster_uuid` using the command:

     ```shell
     kubectl -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
     ```

   * You can find out `prefix` using the command:

     ```shell
     kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
       | base64 -d | grep prefix
     ```

## How to increase the size of a volume?

Set the new size in the corresponding PersistentVolumeClaim resource, in the `spec.resources.requests.storage` parameter.

The operation is fully automatic and takes up to one minute. No further action is required.

The progress of the process can be observed in events using the command `kubectl describe pvc`.

> After modifying a volume, you must wait at least six hours and ensure that the volume is in the `in-use` or `available` state before you can modify the same volume. This is sometimes referred to as a cooldown period. You can find details in the [official documentation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/modify-volume-requirements.html).

## How to configure access to Amazon ECR repository on cluster Nodes

1. Need to set permissions to read images in [Repository policies](https://docs.aws.amazon.com/AmazonECR/latest/userguide/repository-policies.html).

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "RepositoryRead",
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::xxx:role/xxx-node"
      },
      "Action": [
        "ecr:BatchCheckLayerAvailability",
        "ecr:BatchGetImage",
        "ecr:DescribeImages",
        "ecr:DescribeRepositories",
        "ecr:GetAuthorizationToken",
        "ecr:GetDownloadUrlForLayer",
        "ecr:ListImages",
        "ecr:ListTagsForResource"
      ]
    }
  ]
}
```

This policy should be applied in `Amazon ECR > Private registry > Repositories > {{ name }} > Permissions`

1. Add to [additionalRolePolicies](cluster_configuration.html#awsclusterconfiguration-additionalrolepolicies) `ecr:GetAuthorizationToken`

1. Add [NodeGroupConfiguration](../040-node-manager/cr.html#nodegroupconfiguration) for authorization to `Amazon ECR` by replacing the values for `ECR_REGION` and `ECR_ID`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-ecr-registry.sh
spec:
  bundles:
    - 'ubuntu-lts'
  nodeGroups:
    - '*'
  weight: 31
  content: |
    # Copyright 2024 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    bb-apt-install awscli
    ECR_REGION=eu-central-1
    ECR_ID=xxx
    ECR_PASSWORD=$(aws ecr get-login-password --region $ECR_REGION)
    ECR_DOMAIN=$ECR_ID.dkr.ecr.$ECR_REGION.amazonaws.com
    ECR_AUTH=`echo "AWS:$ECR_PASSWORD" | base64 -w0`

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/ecr.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${ECR_DOMAIN}"]
              endpoint = ["https://${ECR_DOMAIN}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${ECR_DOMAIN}".auth]
              auth = "${ECR_AUTH}"
    EOF
```
