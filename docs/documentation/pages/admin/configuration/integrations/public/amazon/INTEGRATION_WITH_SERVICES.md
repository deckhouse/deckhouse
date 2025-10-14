---
title: Integration with Amazon Web Services
permalink: en/admin/integrations/public/amazon/services.html
---

## Working with Amazon Elastic Container Registry (ECR)

To allow cluster nodes to access private Amazon ECR repositories, follow these steps:

1. Define read permissions for container images in the repository policies.
   It's important that the `Principal` field includes the IAM Role attached to the Deckhouse Kubernetes Platform (DKP) nodes.

   Example policy:

    ```yaml
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

1. Set this policy via the AWS Console.
   Go to **Amazon ECR** → **Private registry** → **Repositories** → select the desired repository → **Permissions**.
1. Add `ecr:GetAuthorizationToken` to [`additionalRolePolicies`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-additionalrolepolicies) in your AWSClusterConfiguration
   so that nodes can automatically retrieve an authorization token to pull images:

   ```yaml
   additionalRolePolicies:
     - ecr:GetAuthorizationToken
     - ecr:BatchGetImage
     - ecr:DescribeRepositories
   ```

The [additionalRolePolicies](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-additionalrolepolicies) parameter lets you extend the set of IAM actions granted to EC2 instances managed by DKP.
This is particularly useful if access to the following is required:

- Amazon ECR
- Amazon S3
- Other AWS services that require specific permissions

If this parameter is not set, IAM roles will only include the basic actions:

```yaml
ec2:DescribeTags
ec2:DescribeInstances
```
