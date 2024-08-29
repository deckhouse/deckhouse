---
title: "Cloud provider — AWS: FAQ"
---


## Как поднять пиринговое соединение  между VPC?

Для примера рассмотрим настройку пирингового соединения между двумя условными VPC — vpc-a и vpc-b.

> **Важно!** IPv4 CIDR у обоих VPC должен различаться.

Для настройки выполните следующие шаги:

1. Перейдите в регион, где работает vpc-a.
1. Нажмите `VPC` -> `VPC Peering Connections` -> `Create Peering Connection` и настройте пиринговое соединение:
   * Name: `vpc-a-vpc-b`.
   * Заполните `Local` и `Another VPC`.
1. Перейдите в регион, где работает vpc-b.
1. Нажмите `VPC` -> `VPC Peering Connections`.
1. Выделите созданное соединение и выберите `Action "Accept Request"`.
1. Для vpc-a добавьте во все таблицы маршрутизации маршруты до CIDR vpc-b через пиринговое соединение.
1. Для vpc-b добавьте во все таблицы маршрутизации маршруты до CIDR vpc-a через пиринговое соединение.

## Как создать кластер в новом VPC с доступом через имеющийся bastion-хост?

1. Выполнить bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

2. Поднять пиринговое соединение по инструкции [выше](#как-поднять-пиринговое-соединение--между-vpc).

3. Продолжить установку кластера. На вопрос про кэш Terraform ответить `y`:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

## Как создать кластер в новом VPC и развернуть bastion-хост для доступа к узлам?

1. Выполнить bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

2. Запустить вручную bastion-хост в subnet <prefix>-public-0.

3. Продолжить установку кластера. На вопрос про кэш Terraform ответить `y`:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

## Особенности настройки bastion

Поддерживаются сценарии:
* bastion-хост уже создан во внешней VPC:
  1. Создайте базовую инфраструктуру кластера — `dhctl bootstrap-phase base-infra`.
  1. Настройте пиринговое соединение между внешней и свежесозданной VPC.
  1. Продолжите установку с указанием bastion-хоста — `dhctl bootstrap --ssh-bastion...`.
* bastion-хост требуется поставить в свежесозданной VPC:
  1. Создайте базовую инфраструктуру кластера — `dhctl bootstrap-phase base-infra`.
  1. Запустите вручную bastion-хост в subnet <prefix>-public-0.
  1. Продолжите установку с указанием bastion-хоста — `dhctl bootstrap --ssh-bastion...`.

## Добавление CloudStatic-узлов в кластер

Для добавления виртуальной машины в качестве узла в кластер выполните следующие шаги:

1. Прикрепите группу безопасности `<prefix>-node` к виртуальной машине.
1. Прикрепите IAM-роль `<prefix>-node` к виртуальной машине.
1. Укажите следующие теги у виртуальной машины (чтобы `cloud-controller-manager` смог найти виртуальные машины в облаке):

   ```text
   "kubernetes.io/cluster/<cluster_uuid>" = "shared"
   "kubernetes.io/cluster/<prefix>" = "shared"
   ```

   * Узнать `cluster_uuid` можно с помощью команды:

     ```shell
     kubectl -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
     ```

   * Узнать `prefix` можно с помощью команды:

     ```shell
     kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
       | base64 -d | grep prefix
     ```

## Как увеличить размер volume в кластере?

Задайте новый размер в соответствующем ресурсе PersistentVolumeClaim в параметре `spec.resources.requests.storage`.

Операция проходит полностью автоматически и занимает до одной минуты. Никаких дополнительных действий не требуется.

За ходом процесса можно наблюдать в events через команду `kubectl describe pvc`.

> После изменения volume нужно подождать не менее шести часов и убедиться, что volume находится в состоянии `in-use` или `available`, прежде чем станет возможно изменить его еще раз. Подробности можно найти [в официальной документации](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/modify-volume-requirements.html).

## Как настроить доступ до Amazon ECR repository на нодах кластера

1. Нужно в [Repository policies](https://docs.aws.amazon.com/AmazonECR/latest/userguide/repository-policies.html) задать права для четения образов

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

Эту политику нужно применять в `Amazon ECR > Private registry > Repositories > {{ name }} > Permissions`

1. Добавить в [additionalRolePolicies](cluster_configuration.html#awsclusterconfiguration-additionalrolepolicies) `ecr:GetAuthorizationToken`

1. Добавляем ngc для авторизации в registry заменив значения для ECR_REGION и ECR_ID

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
