### Preparing environment
You have to create an IAM account with the AWS cloud provider so that Deckhouse can manage cloud resources. The detailed instructions for creating an IAM account with AWS are available in the provider's [documentation](https://docs.aws.amazon.com/eks/latest/userguide/create-service-account-iam-policy-and-role.html). Below, we will provide a brief overview of the necessary actions:

- Create the `JSON specification` using the following command.
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
                "ec2:DescribeInternetGateways",
                "ec2:DescribeKeyPairs",
                "ec2:DescribeNatGateways",
                "ec2:DescribeNetworkInterfaces",
                "ec2:DescribeRegions",
                "ec2:DescribeRouteTables",
                "ec2:DescribeSecurityGroups",
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

- Create a new Policy based on the specification created above with `D8CloudProviderAWS` as a policy name and the ARN identifier:
  ```shell
aws iam create-policy --policy-name D8Policy --policy-document file://policy.json
```

   You will see the following:
   ```yaml
{
    "Policy": {
        "PolicyName": "D8Policy",
        "PolicyId": "AAA",
        "Arn": "arn:aws:iam::123:policy/D8Policy",
        "Path": "/",D8Policy
        "DefaultVersionId": "v1",
        "AttachmentCount": 0,
        "PermissionsBoundaryUsageCount": 0,
        "IsAttachable": true,
        "CreateDate": "2020-08-27T02:52:06+00:00",
        "UpdateDate": "2020-08-27T02:52:06+00:00"
    }
}
```
- Create a new user:
   ```shell
aws iam create-user --user-name deckhouse
```

  You will see the following:
   ```yaml
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
- You need to allow access to the API and remember your `AccessKeyId` + `SecretAccessKey` values:
  ```shell
aws iam create-access-key --user-name deckhouse
```

  You will see the following:
  ```yaml
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
- Attach the specified `Policy` to the specified `User`:
  ```shell
aws iam attach-user-policy --user-name username --policy-arn arn:aws:iam::123:policy/D8Policy
```

### Preparing the configuration
- Select your layout — the way how resources are located in the cloud *(there are several pre-defined layouts for each provider in Deckhouse)*. For the AWS example, we will use the **WithoutNAT** layout. In this layout, the virtual machines will access the Internet through a NAT Gateway with a shared and single source IP. All nodes created with Deckhouse can optionally get a public IP (ElasticIP). The other available options are described in the [Cloud providers](https://early.deckhouse.io/ru/documentation/v1/kubernetes.html) section of the documentation.
- Define the three primary sections with parameters of the prospective cluster in the `config.yml` file:

{% offtopic title="config.yml for CE" %}
```yaml
# general cluster parameters (ClusterConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: ClusterConfiguration
# type of the infrastructure: bare-metal (Static) or Cloud (Cloud)
clusterType: Cloud
# cloud provider-related settings
cloud:
  # type of the cloud provider
  provider: AWS
  # prefix to differentiate cluster objects (can be used, e.g., in routing)
  prefix: "aws-demo"
# address space of the cluster's pods
podSubnetCIDR: 10.111.0.0/16
# address space of the cluster's services
serviceSubnetCIDR: 10.222.0.0/16
# Kubernetes version to install
kubernetesVersion: "1.19"
# cluster domain (used for local routing)
clusterDomain: "cluster.local"
---
# section for bootstrapping the Deckhouse cluster (InitConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: InitConfiguration
# Deckhouse parameters
deckhouse:
  # address of the registry where the installer image is located; in this case, the default value for Deckhouse CE is set
  imagesRepo: registry.deckhouse.io/deckhouse/ce
  # a special string with parameters to access Docker registry
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=
  # the release channel in use
  releaseChannel: Beta
  configOverrides:
    global:
      # the cluster name (it is used, e.g., in Prometheus alerts' labels)
      clusterName: somecluster
      # the cluster's project name (it is used for the same purpose as the cluster name)
      project: someproject
      modules:
        # template that will be used for system apps domains within the cluster
        # e.g., Grafana for %s.somedomain.com will be available as grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# section containing the parameters of the cloud provider
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: AWSClusterConfiguration
# pre-defined layout from Deckhouse
layout: WithoutNAT
# AWS access parameters
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  # cluster region
  region: eu-central-1
# parameters of the master node group
masterNodeGroup:
  # number of replicas
  # if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
  replicas: 1
  # Parameters of the VM image
  instanceClass:
    # Type of the instance
    instanceType: c5.large
    # Amazon Machine Image id
    ami: ami-0fee04b212b7499e2
# address space of the AWS cloud
vpcNetworkCIDR: "10.241.0.0/16"
# address space of the cluster's nodes
nodeNetworkCIDR: "10.241.32.0/20"
# public SSH key for accessing cloud nodes
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
```
{% endofftopic %}
{% offtopic title="config.yml for EE" %}
```yaml
# general cluster parameters (ClusterConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: ClusterConfiguration
# type of the infrastructure: bare-metal (Static) or Cloud (Cloud)
clusterType: Cloud
# cloud provider-related settings
cloud:
  # type of the cloud provider
  provider: AWS
  # prefix to differentiate cluster objects (can be used, e.g., in routing)
  prefix: "aws-demo"
# address space of the cluster's pods
podSubnetCIDR: 10.111.0.0/16
# address space of the cluster's services
serviceSubnetCIDR: 10.222.0.0/16
# Kubernetes version to install
kubernetesVersion: "1.19"
# cluster domain (used for local routing)
clusterDomain: "cluster.local"
---
# section for bootstrapping the Deckhouse cluster (InitConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: InitConfiguration
# Deckhouse parameters
deckhouse:
  # address of the registry where the installer image is located; in this case, the default value for Deckhouse EE is set
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # a special string with your token to access Docker registry (generated automatically for your demo token)
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # the release channel in use
  releaseChannel: Beta
  configOverrides:
    global:
      # the cluster name (it is used, e.g., in Prometheus alerts' labels)
      clusterName: somecluster
      # the cluster's project name (it is used for the same purpose as the cluster name)
      project: someproject
      modules:
        # template that will be used for system apps domains within the cluster
        # e.g., Grafana for %s.somedomain.com will be available as grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# section containing the parameters of the cloud provider
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: AWSClusterConfiguration
# pre-defined layout from Deckhouse
layout: WithoutNAT
# AWS access parameters
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  # cluster region
  region: eu-central-1
# parameters of the master node group
masterNodeGroup:
  # number of replicas
  # if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
  replicas: 1
  # parameters of the VM image
  instanceClass:
    # type of the instance
    instanceType: c5.large
    # Amazon Machine Image id
    ami: ami-0fee04b212b7499e2
# address space of the AWS cloud
vpcNetworkCIDR: "10.241.0.0/16"
# address space of the cluster's nodes
nodeNetworkCIDR: "10.241.32.0/20"
# public SSH key for accessing cloud nodes
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
```
{% endofftopic %}

Notes:
- The complete list of supported cloud providers and their specific settings is available in the [Cloud providers](/en/documentation/v1/kubernetes.html) section of the documentation.
- To learn more about the Deckhouse release channels, please see the [relevant documentation](/en/documentation/v1/deckhouse-release-channels.html).
