### Подготовка окружения
Чтобы Deckhouse смог управлять ресурсами в облаке AWS, необходимо создать IAM-аккаунт. Подробная инструкция по этому действию доступна в [документации провайдера](https://docs.aws.amazon.com/eks/latest/userguide/create-service-account-iam-policy-and-role.html), а здесь мы представим краткую последовательность необходимых действий, выполняемых в консоли.

- При помощи следующей команды сохраните JSON-спецификацию:
{% offtopic title="Команда создания policy.json" %}
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

- Создайте на основе ранее созданной спецификации новую Policy с именем `D8CloudProviderAWS` и примечанием ARN:
  ```shell
aws iam create-policy --policy-name D8Policy --policy-document file://policy.json
```

   В ответ отобразится следующий текст:
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
- Создайте нового пользователя:
   ```shell
aws iam create-user --user-name deckhouse
```

  В ответ отобразится следующий текст:
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
- Разрешите доступ к API и сохраните пару `AccessKeyId` + `SecretAccessKey`:
  ```shell
aws iam create-access-key --user-name deckhouse
```
  В ответ отобразится следующий текст:
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
- Объедините `User` и `Policy`:
  ```shell
aws iam attach-user-policy --user-name username --policy-arn arn:aws:iam::123:policy/D8Policy
```

### Подготовка конфигурации
- Выберите layout — архитектуру размещения объектов в облаке *(для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse)*. Для примера с AWS мы возьмем вариант **WithoutNAT**. В данной схеме размещения виртуальные машины будут выходить в интернет через NAT Gateway с общим и единственным source IP. Все узлы, созданные с помощью Deckhouse, опционально могут получить публичный IP (ElasticIP). Другие доступные варианты описаны в секции документации [Cloud providers](https://early.deckhouse.io/ru/documentation/v1/kubernetes.html).
- Задайте минимальные 3 секции параметров для будущего кластера в файле `config.yml`:

{% offtopic title="config.yml для CE" %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare-metal (Static) или облако (Cloud)
clusterType: Cloud
# параметры облачного провайдера
cloud:
  # используемый облачный провайдер
  provider: AWS
  # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
  prefix: "aws-demo"
# адресное пространство pod’ов кластера
podSubnetCIDR: 10.111.0.0/16
# адресное пространство для service’ов кластера
serviceSubnetCIDR: 10.222.0.0/16
# устанавливаемая версия Kubernetes
kubernetesVersion: "1.19"
# домен кластера
clusterDomain: "cluster.local"
---
# секция первичной инициализации кластера Deckhouse (InitConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# секция с параметрами Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для CE-сборки Deckhouse
  imagesRepo: registry.deckhouse.io/deckhouse/ce
  # строка с параметрами подключения к Docker registry
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=
  # используемый канал обновлений
  releaseChannel: Beta
  configOverrides:
    global:
      # имя кластера; используется, например, в лейблах алертов Prometheus
      clusterName: somecluster
      # имя проекта для кластера; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# секция, описывающая параметры облачного провайдера
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: AWSClusterConfiguration
# layout — архитектура расположения ресурсов в облаке
layout: WithoutNAT
# параметры доступа к облаку AWS
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  # регион привязки кластера
  region: eu-central-1
masterNodeGroup:
  # количество реплик мастера
  # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
  replicas: 1
  # параметры инстанса
  instanceClass:
    # тип используемого инстанса
    instanceType: c5.large
    # id образа виртуальной машины
    ami: ami-0fee04b212b7499e2
# адресное пространство облака внутри AWS
vpcNetworkCIDR: "10.241.0.0/16"
# адресное пространство узлов кластера
nodeNetworkCIDR: "10.241.32.0/20"
# публичная часть SSH-ключа для доступа к узлам облака
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
```
{% endofftopic %}
{% offtopic title="config.yml для EE" %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare-metal (Static) или облако (Cloud)
clusterType: Cloud
# параметры облачного провайдера
cloud:
  # используемый облачный провайдер
  provider: AWS
  # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
  prefix: "aws-demo"
# адресное пространство pod’ов кластера
podSubnetCIDR: 10.111.0.0/16
# адресное пространство для service’ов кластера
serviceSubnetCIDR: 10.222.0.0/16
# устанавливаемая версия Kubernetes
kubernetesVersion: "1.19"
# домен кластера
clusterDomain: "cluster.local"
---
# секция первичной инициализации кластера Deckhouse (InitConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# секция с параметрами Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для EE-сборки Deckhouse
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего демонстрационного токена)
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # используемый канал обновлений
  releaseChannel: Beta
  configOverrides:
    global:
      # имя кластера; используется, например, в лейблах алертов Prometheus
      clusterName: somecluster
      # имя проекта для кластера; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# секция, описывающая параметры облачного провайдера
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: AWSClusterConfiguration
# layout — архитектура расположения ресурсов в облаке
layout: WithoutNAT
# параметры доступа к облаку AWS
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  # регион привязки кластера
  region: eu-central-1
masterNodeGroup:
  # количество реплик мастера
  # если будет больше одного мастер-узла, то control-plane на всех master-узлах будет развернут автоматическии
  replicas: 1
  # параметры инстанса
  instanceClass:
    # тип используемого инстанса
    instanceType: c5.large
    # id образа виртуальной машины
    ami: ami-0fee04b212b7499e2
# адресное пространство облака внутри AWS
vpcNetworkCIDR: "10.241.0.0/16"
# адресное пространство узлов кластера
nodeNetworkCIDR: "10.241.32.0/20"
# публичная часть SSH-ключа для доступа к узлам облака
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
```
{% endofftopic %}

Примечания:
- Полный список поддерживаемых облачных провайдеров и настроек для них доступен в секции документации [Cloud providers](/ru/documentation/v1/kubernetes.html).
- Подробнее о каналах обновления Deckhouse (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
