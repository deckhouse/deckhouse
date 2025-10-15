---
title: Подключение и авторизация
permalink: ru/admin/integrations/public/amazon/authorization.html
description: "Настройка подключения и авторизации AWS для Deckhouse Kubernetes Platform. Роли IAM, настройка учетных данных и требования к интеграции AWS для облачного развертывания."
lang: ru
---

## Требования

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

Перед началом работы необходимо подготовить облачное окружение и обеспечить доступ к AWS-ресурсам для компонентов Deckhouse Kubernetes Platform (DKP), взаимодействующих с API AWS.

На всех виртуальных машинах, которые будут использоваться в составе кластера, должен быть установлен пакет `cloud-init`. После запуска необходимо убедиться, что активны следующие службы:

- `cloud-config.service`;
- `cloud-final.service`;
- `cloud-init.service`.

Эти службы необходимы для корректной инициализации экземпляров EC2 и взаимодействия с инфраструктурными модулями DKP, такими как [`cloud-provider-aws`](/modules/cloud-provider-aws/) и `machine-controller-manager`.

## Доступ к AWS API

Для доступа к API AWS требуется IAM-пользователь с соответствующим набором прав. Необходимо создать JSON-спецификацию IAM-политики с разрешениями на работу с EC2, ELB, IAM, KMS и другими сервисами AWS.

Пример JSON-файла с конфигурацией необходимых прав:

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
                "ec2:DescribeAccountAttributes",
                "ec2:DescribeAddresses",
                "ec2:DescribeAvailabilityZones",
                "ec2:DescribeImages",
                "ec2:DescribeInstanceAttribute",
                "ec2:DescribeInstanceCreditSpecifications",
                "ec2:DescribeInstances",
                "ec2:DescribeInstanceTypes",
                "ec2:DescribeInstanceTopology",
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
                "ec2:CreateNetworkInterface",
                "ec2:DescribeNetworkInterfaceAttribute",
                "ec2:ModifyNetworkInterfaceAttribute",
                "ec2:DeleteNetworkInterface",
                "ec2:DescribeNetworkInterfaces",                
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
```

### Настройка IAM через веб-интерфейс AWS

1. Создайте политики:

   - Перейдите в раздел «Identity and Access Management (IAM)» → «Policies».
   - Нажмите «Create policy».
   - Перейдите на вкладку «JSON» и вставьте подготовленную JSON-спецификацию.
   - Нажмите «Next: Tags», затем «Next: Review».
   - Укажите имя политики, например D8CloudProviderAWS.
   - Нажмите «Create policy».

1. Создайте пользователя:

   - Перейдите в «IAM» → «Users».
   - Нажмите «Add users».
   - В поле «User name» укажите, например, deckhouse.
   - В разделе «Select AWS credential type» выберите опцию «Access key» → «Programmatic access».
   - Нажмите «Next: Permissions».

1. Привяжите политики к пользователю:

   - Выберите «Attach existing policies directly».
   - Введите имя ранее созданной политики в поле фильтрации и установите флажок рядом с нужной политикой.
   - Нажмите «Next: Tags», затем «Next: Review».
   - Нажмите «Create user».

{% alert level="info" %}
Cохраните полученные `Access key ID` и `Secret access key`. Эти данные потребуются для настройки DKP.
{% endalert %}

Проверьте, есть ли у вашей учетной записи (и, соответственно, у созданного пользователя) доступ к нужным регионам. Для этого выберите необходимый регион в выпадающем списке в правом верхнем углу. Если произойдет переключение в выбранный регион, доступ к региону есть. Если доступа к региону нет, вы получите следующее сообщение (может отличаться):

![Разрешить использование региона](../../../../images/cloud-provider-aws/region_enable.png)

В этом случае нажмите «Continue», чтобы разрешить использование региона.

### Настройка IAM через AWS CLI

Также IAM можно настроить через интерфейс командной строки. Для этого выполните следующие шаги:

1. Сохраните JSON-спецификацию в файл `policy.json`:

   ```shell
   cat > policy.json << EOF
   <Policy JSON spec>
   EOF
   ```

1. Создайте новую Policy с именем `D8CloudProviderAWS` и примечанием `ARN`, используя JSON-спецификацию из файла `policy.json`:

   ```shell
   aws iam create-policy --policy-name D8CloudProviderAWS --policy-document file://policy.json
   ```

   Пример вывода:

   ```yaml
   {
       "Policy": {
           "PolicyName": "D8CloudProviderAWS",
           "PolicyId": "AAA",
           "Arn": "arn:aws:iam::123:policy/D8CloudProviderAWS",
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

1. Создайте нового пользователя:

   ```shell
   aws iam create-user --user-name deckhouse
   ```

   Пример вывода:

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

1. Разрешите доступ к API и сохраните `AccessKeyId` и `SecretAccessKey`:

   ```shell
   aws iam create-access-key --user-name deckhouse
   ```

   Пример вывода:

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

1. Привяжите политики к пользователю. Объедините `User` и `Policy`:

   ```shell
   aws iam attach-user-policy --user-name username --policy-arn arn:aws:iam::123:policy/D8CloudProviderAWS
   ```

### Настройка IAM через Terraform

Пример конфигурации ресурсов для Terraform:

```hcl
resource "aws_iam_user" "user" {
  name = "deckhouse"
}

resource "aws_iam_access_key" "user" {
  user = aws_iam_user.user.name
}

resource "aws_iam_policy" "policy" {
  name        = "D8CloudProviderAWS"
  path        = "/"
  description = "Deckhouse policy"

  policy = <<EOF
<JSON-спецификация Policy>
EOF
}

resource "aws_iam_user_policy_attachment" "policy-attachment" {
  user       = aws_iam_user.user.name
  policy_arn = aws_iam_policy.policy.arn
}
```

### Использование своей IAM-роли для узлов (iamNodeRole)

Параметр `iamNodeRole` в ресурсе AWSClusterConfiguration позволяет переопределить IAM-роль по умолчанию, которую DKP назначает всем EC2-инстансам узлов кластера.

По умолчанию DKP создаёт и назначает IAM-роль с именем `<prefix>-node`, где `<prefix>` — это значение глобального параметра `cloud.prefix`. Эта роль включает минимально необходимый набор прав для работы узлов.

Если же вам необходимо:

- использовать предварительно созданную IAM-роль, управляемую вручную;
- предоставить узлам расширенные полномочия, например, доступ к специфичным сервисам AWS;

— вы можете указать имя IAM-роли в параметре `iamNodeRole`.

{% alert level="info" %}
Указанная роль должна включать все права, требуемые DKP, иначе узлы не смогут полноценно функционировать. В частности, она обязательно должна включать права роли `<prefix>-node>`, даже если они назначаются вручную.
{% endalert %}

Подробнее о IAM-ролях для AWS EC2 можно прочитать [в документации AWS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html).
