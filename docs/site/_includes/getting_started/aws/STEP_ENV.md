{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

You have to create an IAM account with the {{ page.platform_name[page.lang] }} cloud provider so that Deckhouse Kubernetes Platform can manage cloud resources. The detailed instructions for creating an IAM account with AWS are available in the [documentation](/documentation/v1/modules/030-cloud-provider-aws/environment.html). Below, we will provide a brief overview of the necessary actions (run them on the **personal computer**):

Create the `JSON specification` using the following command.

{% offtopic title="Command to create policy.json" %}
```bash
cat > policy.json << EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "autoscaling:DescribeAutoScalingGroups",
                "autoscaling:DescribeLaunchConfigurations",
                "autoscaling:DescribeTags",
                "ec2:AllocateAddress",
                "ec2:AssociateAddress",
                "ec2:AssociateRouteTable",
                "ec2:AttachInternetGateway",
                "ec2:AttachVolume",
                "ec2:AuthorizeSecurityGroupEgress",
                "ec2:AuthorizeSecurityGroupIngress",
                "ec2:CreateInternetGateway",
                "ec2:CreateKeyPair",
                "ec2:CreateNATGateway",
                "ec2:CreateRoute",
                "ec2:CreateRouteTable",
                "ec2:CreateSecurityGroup",
                "ec2:CreateSubnet",
                "ec2:CreateTags",
                "ec2:CreateVolume",
                "ec2:CreateVpc",
                "ec2:DeleteInternetGateway",
                "ec2:DeleteKeyPair",
                "ec2:DeleteNATGateway",
                "ec2:DeleteRoute",
                "ec2:DeleteRouteTable",
                "ec2:DeleteSecurityGroup",
                "ec2:DeleteSubnet",
                "ec2:DeleteTags",
                "ec2:DeleteVolume",
                "ec2:DeleteVpc",
                "ec2:DescribeAccountAttributes",
                "ec2:DescribeAddresses",
                "ec2:DescribeAvailabilityZones",
                "ec2:DescribeImages",
                "ec2:DescribeInstanceAttribute",
                "ec2:DescribeInstanceCreditSpecifications",
                "ec2:DescribeInstances",
                "ec2:DescribeInstanceTypes",
                "ec2:DescribeInternetGateways",
                "ec2:DescribeKeyPairs",
                "ec2:DescribeNatGateways",
                "ec2:DescribeNetworkInterfaces",
                "ec2:DescribeRegions",
                "ec2:DescribeRouteTables",
                "ec2:DescribeSecurityGroups",
                "ec2:DescribeSecurityGroupRules",
                "ec2:DescribeSubnets",
                "ec2:DescribeTags",
                "ec2:DescribeVolumesModifications",
                "ec2:DescribeVolumes",
                "ec2:DescribeVpcAttribute",
                "ec2:DescribeVpcClassicLink",
                "ec2:DescribeVpcClassicLinkDnsSupport",
                "ec2:DescribeVpcs",
                "ec2:DetachInternetGateway",
                "ec2:DetachVolume",
                "ec2:DisassociateAddress",
                "ec2:DisassociateRouteTable",
                "ec2:ImportKeyPair",
                "ec2:ModifyInstanceAttribute",
                "ec2:ModifySubnetAttribute",
                "ec2:ModifyVolume",
                "ec2:ModifyVpcAttribute",
                "ec2:ReleaseAddress",
                "ec2:RevokeSecurityGroupEgress",
                "ec2:RevokeSecurityGroupIngress",
                "ec2:RunInstances",
                "ec2:TerminateInstances",
                "ec2:DescribeVpcPeeringConnections",
                "ec2:CreateVpcPeeringConnection",
                "ec2:DeleteVpcPeeringConnection",
                "ec2:AcceptVpcPeeringConnection",
                "elasticloadbalancing:AddTags",
                "elasticloadbalancing:ApplySecurityGroupsToLoadBalancer",
                "elasticloadbalancing:AttachLoadBalancerToSubnets",
                "elasticloadbalancing:ConfigureHealthCheck",
                "elasticloadbalancing:CreateListener",
                "elasticloadbalancing:CreateLoadBalancer",
                "elasticloadbalancing:CreateLoadBalancerListeners",
                "elasticloadbalancing:CreateLoadBalancerPolicy",
                "elasticloadbalancing:CreateTargetGroup",
                "elasticloadbalancing:DeleteListener",
                "elasticloadbalancing:DeleteLoadBalancer",
                "elasticloadbalancing:DeleteLoadBalancerListeners",
                "elasticloadbalancing:DeleteTargetGroup",
                "elasticloadbalancing:DeregisterInstancesFromLoadBalancer",
                "elasticloadbalancing:DeregisterTargets",
                "elasticloadbalancing:DescribeListeners",
                "elasticloadbalancing:DescribeLoadBalancerAttributes",
                "elasticloadbalancing:DescribeLoadBalancerPolicies",
                "elasticloadbalancing:DescribeLoadBalancers",
                "elasticloadbalancing:DescribeTargetGroups",
                "elasticloadbalancing:DescribeTargetHealth",
                "elasticloadbalancing:DetachLoadBalancerFromSubnets",
                "elasticloadbalancing:ModifyListener",
                "elasticloadbalancing:ModifyLoadBalancerAttributes",
                "elasticloadbalancing:ModifyTargetGroup",
                "elasticloadbalancing:RegisterInstancesWithLoadBalancer",
                "elasticloadbalancing:RegisterTargets",
                "elasticloadbalancing:SetLoadBalancerPoliciesForBackendServer",
                "elasticloadbalancing:SetLoadBalancerPoliciesOfListener",
                "iam:AddRoleToInstanceProfile",
                "iam:CreateInstanceProfile",
                "iam:CreateRole",
                "iam:CreateServiceLinkedRole",
                "iam:DeleteInstanceProfile",
                "iam:DeleteRole",
                "iam:DeleteRolePolicy",
                "iam:GetInstanceProfile",
                "iam:GetRole",
                "iam:GetRolePolicy",
                "iam:ListInstanceProfilesForRole",
                "iam:ListRolePolicies",
                "iam:ListAttachedRolePolicies",
                "iam:PassRole",
                "iam:PutRolePolicy",
                "iam:RemoveRoleFromInstanceProfile",
                "iam:TagRole",
                "kms:DescribeKey",
                "sts:GetCallerIdentity"
            ],
            "Resource": "*"
        }
    ]
}
EOF
```
{% endofftopic %}

Create a new Policy based on the specification created above with `D8CloudProviderAWS` as a policy name:
{% snippetcut %}
```shell
aws iam create-policy --policy-name D8Policy --policy-document file://policy.json
```
{% endsnippetcut %}

> You will see the following:
> ```yaml
  {
      "Policy": {
          "PolicyName": "D8Policy",
          "PolicyId": "AAA",
          "Arn": "arn:aws:iam::123:policy/D8Policy",
          "Path": "/",
          "DefaultVersionId": "v1",
          "AttachmentCount": 0,
          "PermissionsBoundaryUsageCount": 0,
          "IsAttachable": true,
          "CreateDate": "2020-08-27T02:52:06+00:00",
          "UpdateDate": "2020-08-27T02:52:06+00:00"
      }
  }
  ```

Create a new user:
{% snippetcut %}
```shell
aws iam create-user --user-name deckhouse
```
{% endsnippetcut %}

> You will see the following:
> ```yaml
  {
      "User": {
          "Path": "/",
          "UserName": "deckhouse",
          "UserId": "AAAXXX",
          "Arn": "arn:aws:iam::123:user/deckhouse",
          "CreateDate": "2020-08-27T03:05:42+00:00"
      }
  }
  ```

You need to allow access to the API and remember your `AccessKeyId` + `SecretAccessKey` values:
{% snippetcut %}
```shell
aws iam create-access-key --user-name deckhouse
```
{% endsnippetcut %}

> You will see the following:
> ```yaml
  {
      "AccessKey": {
          "UserName": "deckhouse",
          "AccessKeyId": "XXXYYY",
          "Status": "Active",
          "SecretAccessKey": "ZZZzzz",
          "CreateDate": "2020-08-27T03:06:22+00:00"
      }
  }
  ```

Attach the specified `Policy` to the specified `User`:
{% snippetcut %}
```shell
aws iam attach-user-policy --user-name username --policy-arn arn:aws:iam::123:policy/D8Policy
```
{% endsnippetcut %}
