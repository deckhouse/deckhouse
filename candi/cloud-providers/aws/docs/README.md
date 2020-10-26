title: "Cloud provider — AWS: Развертывание"

## Поддерживаемые схемы размещения

Схема размещения описывается объектом `AWSClusterConfiguration`. Его поля:

* `layout` — архитектура расположения ресурсов в облаке.
  * Варианты — `Standard` или `WithoutNAT` (описание ниже).
* `standard` — настройки для лейаута `Standard`.
  * `associatePublicIPToMasters` — выдать ли мастерам публичные IP. По умолчанию — `true`.
* `provider` — параметры подключения к API AWS.
  * `providerAccessKeyId` — access key [ID](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys).
  * `providerSecretAccessKey` — access key [secret](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys).
  * `region` — имя AWS региона, в котором будут заказываться instances.
* `masterNodeGroup` — спеки для описания NG мастера.
  * `replicas` — сколько мастер-узлов создать.
  * `instanceClass` — частичное содержимое полей [AWSInstanceClass](/modules/030-cloud-provider-aws/#awsinstanceclass-custom-resource). Допустимые параметры:
    * `instanceType`
    * `ami`
    * `additionalSecurityGroups`
    * `diskType`
    * `diskSizeGb`
  * `zones` — ограниченный набор зон, в которых разрешено создавать мастер-ноды. Опциональный параметр.
  * `additionalTags` — дополнительные к основным (`AWSClusterConfiguration.tags`) теги, которые будут присвоены созданным инстансам.
* `nodeGroups` — массив дополнительных NG для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NG:
  * `name` — имя NG, будет использоваться для генерации имени нод.
  * `replicas` — количество нод.
  * `instanceClass` — частичное содержимое полей [AWSInstanceClass](/modules/030-cloud-provider-aws/#awsinstanceclass-custom-resource). Допустимые параметры:
    * `instanceType`
    * `ami`
    * `additionalSecurityGroups`
    * `diskType`
    * `diskSizeGb`
  * `zones` — ограниченный набор зон, в которых разрешено создавать ноды. Опциональный параметр.
  * `additionalTags` — дополнительные к основным (`AWSClusterConfiguration.tags`) теги, которые будут присвоены созданным инстансам.
* `vpcNetworkCIDR` — подсеть, которая будет указана в созданном VPC.
  * обязательный параметр если не указан параметр для развёртывания в уже созданном VPC `existingVPCID` (см. ниже).
* `existingVPCID` — ID существующего VPC, в котором будет развёрнута схема.
  * Обязательный параметр если не указан `vpcNetworkCIDR`.
* `nodeNetworkCIDR` — подсеть, в которой будут работать ноды кластера.
  * Диапазон должен быть частью или должен соответствовать диапазону адресов VPC.
  * Диапазон будет равномерно разбит на подсети по одной на Availability Zone в вашем регионе.
  * Необязательный, но рекомендованный параметр. По умолчанию — соответствует целому диапазону адресов VPC.
* `sshPublicKey` — публичный ключ для доступа на ноды.
* `tags` — теги, которые будут присвоены всем созданным ресурсам.

### Standard

В данной схеме размещения виртуальные машины будут выходить в интернет через NAT Gateway. Исключение — master, который опционально может получить публичный IP.

> Если используется master без белого IP и bastion находится в другой VPC, то в процессе установки после создания base-infrastructure
> необходимо будет вручную настроить peering между VPC с bastion'ом и VPC кластера. Чтобы избежать этой проблемы,
> рекомендуется использовать существующую VPC bastion'a, а не создавать новую.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSkzOWvLzAwB4hmIk4CP1-mj2QIxCyJg2VJvijFfdttjnV0quLpw7x87KtTC5v2I9xF5gVKpTK-aqyz/pub?w=812&h=655)
<!--- Исходник: https://docs.google.com/drawings/d/1kln-DJGFldcr6gayVtFYn_3S50HFIO1PLTc1pC_b3L0/edit --->

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AWSClusterConfiguration
layout: Standard
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  region: eu-central-1
standard:
  associatePublicIPToMasters: true # Выделить ли мастерам белые IP. Если не выделять, то потребуется вручную поднимать бастион.
masterNodeGroup:
  replicas: 1 # Если будет больше одного мастера, то etcd-кластер соберётся автоматически.
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
nodeNetworkCIDR: "10.222.0.0/16"
sshPublicKey: ...
tags:
  team: torpedo
```

### WithoutNAT

В данной схеме каждой ноде присваивается публичный IP. NAT не используется совсем.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQDR2iRcFO3Ra3hmdrYCuoHPP6m3DCArtZjmbQGMJL00xmR-F94IMJKx2jKqeiwe-KvbykqtCEjsR9c/pub?w=812&h=655)
<!--- Исходник: https://docs.google.com/drawings/d/1JDmeSY12EoZ3zBfanEDY-QvSgLekzw6Tzjj2pgY8giM/edit --->

```yaml
apiVersion: deckhouse.io/v1alpha1
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
nodeNetworkCIDR: "10.222.0.0/16"
sshPublicKey: ...
tags:
  team: torpedo
```

## Рекомендуемая настройка IAM

Для работы cloud-provider и machine-controller-manager требуется доступ в API AWS из-под IAM-пользователя, который обладает достаточным набором прав.

### JSON-спецификация Policy

Инструкции, как применить этот JSON ниже.

```json
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
```

### Настройка IAM через веб-интерфейс

* Создать `Customer Managed Policy`
* Выбрать вкладку `JSON` и вставить спецификацию выше.
* `Review Policy`
* Задать имя, например `D8CloudProviderAWS`
* `Create Policy`
* Создать IAM User
* Задать имя, например `d8-candi`
* Выбрать `Programmatic access`
* Выбрать вкладку `Attach existing policies directly` и нажать `Create Policy`
* Найти `D8CloudProviderAWS` и поставить галку
* Далее по интуиции

### Настройка IAM через cli

Сохранить [спецификацию выше](#json-спецификация-policy) в файл:
```
cat > policy.json << EOF
<JSON-спецификация Policy>
EOF

```
Создать на основе спецификации новую Policy с именем `D8CloudProviderAWS` и примечаем ARN:
```
aws iam create-policy --policy-name D8MyPolicy --policy-document file://policy.json

{
    "Policy": {
        "PolicyName": "D8MyPolicy",
        "PolicyId": "AAA",
        "Arn": "arn:aws:iam::123:policy/D8MyPolicy",
        "Path": "/",
        "DefaultVersionId": "v1",
        "AttachmentCount": 0,
        "PermissionsBoundaryUsageCount": 0,
        "IsAttachable": true,
        "CreateDate": "2020-08-27T02:52:06+00:00",
        "UpdateDate": "2020-08-27T02:52:06+00:00"
    }
CREATE-ACCESS-KEY()                                        CREATE-ACCESS-KEY()
}
```

Создать User:
```
aws iam create-user --user-name d8-candi

{
    "User": {
        "Path": "/",
        "UserName": "d8-candi",
        "UserId": "AAAXXX",
        "Arn": "arn:aws:iam::123:user/d8-candi",
        "CreateDate": "2020-08-27T03:05:42+00:00"
    }
}
```

Разрешаем доступ к API и сохраняем пару `AccessKeyId` + `SecretAccessKey`:
```
aws iam create-access-key --user-name d8-candi

{
    "AccessKey": {
        "UserName": "d8-candi",
        "AccessKeyId": "XXXYYY",
        "Status": "Active",
        "SecretAccessKey": "ZZZzzz",
        "CreateDate": "2020-08-27T03:06:22+00:00"
    }
}
```

Объединяем User и Policy:

```
aws iam attach-user-policy --user-name d8-candi --policy-arn arn:aws:iam::123:policy/D8MyPolicy
```

### Настройка IAM через terraform

```
resource "aws_iam_user" "user" {
  name = "d8-candi"
}

resource "aws_iam_access_key" "user" {
  user = aws_iam_user.user.name
}

resource "aws_iam_policy" "policy" {
  name        = "D8MyPolicy"
  path        = "/"
  description = "Deckhouse candi policy"

  policy = <<EOF
<JSON-спецификация Policy>
EOF
}

resource "aws_iam_user_policy_attachment" "policy-attachment" {
  user       = aws_iam_user.user.name
  policy_arn = aws_iam_policy.policy.arn
}
```
