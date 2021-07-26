---
title: "Cloud provider — AWS: схемы размещения"
---

## Схемы размещения
### WithoutNAT

**Рекомендованная схема размещения.**

Каждому узлу присваивается публичный IP (ElasticIP). NAT не используется совсем.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQDR2iRcFO3Ra3hmdrYCuoHPP6m3DCArtZjmbQGMJL00xmR-F94IMJKx2jKqeiwe-KvbykqtCEjsR9c/pub?w=812&h=655)
<!--- Исходник: https://docs.google.com/drawings/d/1JDmeSY12EoZ3zBfanEDY-QvSgLekzw6Tzjj2pgY8giM/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithoutNAT
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  region: eu-central-1
masterNodeGroup:
  # Количество master-узлов.
  # Если указано больше одного master-узла, то etcd-кластер соберётся автоматически.
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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
tags:
  team: torpedo
```

### Standard

**Важно!** В данной схеме размещения необходим bastion-хост для доступа к узлам.

Виртуальные машины будут выходить в интернет через NAT Gateway с общим и единственным source IP.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSkzOWvLzAwB4hmIk4CP1-mj2QIxCyJg2VJvijFfdttjnV0quLpw7x87KtTC5v2I9xF5gVKpTK-aqyz/pub?w=812&h=655)
<!--- Исходник: https://docs.google.com/drawings/d/1kln-DJGFldcr6gayVtFYn_3S50HFIO1PLTc1pC_b3L0/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: Standard
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  region: eu-central-1
masterNodeGroup:
  # Количество master-узлов.
  # Если указано больше одного master-узла, то etcd-кластер соберётся автоматически.
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
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
tags:
  team: torpedo
```

## AWSClusterConfiguration
Схема размещения (layout) описывается структурой `AWSClusterConfiguration`:
* `layout` — название схемы размещения.
  * Варианты — `WithoutNAT` или `Standard` (описание ниже).
* `provider` — параметры подключения к API AWS.
  * `providerAccessKeyId` — access key [ID](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys).
  * `providerSecretAccessKey` — access key [secret](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys).
  * `region` — имя AWS региона, в котором будут заказываться instances.
* `masterNodeGroup` — спецификация для описания NodeGroup мастера.
  * `replicas` — сколько мастер-узлов создать.
  * `instanceClass` — частичное содержимое полей [AWSInstanceClass](../../modules/030-cloud-provider-aws/#awsinstanceclass-custom-resource). Допустимые параметры:
    * `instanceType`
    * `ami`
    * `additionalSecurityGroups`
    * `diskType`
    * `diskSizeGb`
  * `zones` — ограниченный набор зон, в которых разрешено создавать master-узлы. Опциональный параметр.
  * `additionalTags` — дополнительные к основным (`AWSClusterConfiguration.tags`) теги, которые будут присвоены созданным инстансам.
* `nodeGroups` — массив дополнительных NodeGroup для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NodeGroup:
  * `name` — имя NodeGroup, будет использоваться для генерации имени узлов.
  * `replicas` — количество узлов.
  * `instanceClass` — частичное содержимое полей [AWSInstanceClass]({{"/modules/030-cloud-provider-aws/#awsinstanceclass-custom-resource" | true_relative_url }} ). Допустимые параметры:
    * `instanceType`
    * `ami`
    * `additionalSecurityGroups`
    * `diskType`
    * `diskSizeGb`
  * `zones` — ограниченный набор зон, в которых разрешено создавать узлы. Опциональный параметр.
  * `additionalTags` — дополнительные к основным (`AWSClusterConfiguration.tags`) теги, которые будут присвоены созданным инстансам.
  * `nodeTemplate` — настройки Node-объектов в Kubernetes, которые будут добавлены после регистрации узла.
    * `labels` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.labels`
      * Пример:

        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```

    * `annotations` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.annotations`
      * Пример:

        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```

    * `taints` — аналогично полю `.spec.taints` из объекта [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core). **Внимание!** Доступны только поля `effect`, `key`, `values`.
      * Пример:

        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```

* `vpcNetworkCIDR` — подсеть, которая будет указана в созданном VPC.
  * обязательный параметр если не указан параметр для развёртывания в уже созданном VPC `existingVPCID` (см. ниже).
* `existingVPCID` — ID существующего VPC, в котором будет развёрнута схема.
  * Обязательный параметр если не указан `vpcNetworkCIDR`.
  * **Важно!** Если в данной VPC уже есть Internet Gateway, деплой базовой инфраструктуры упадёт с ошибкой. На данный момент адоптнуть Internet Gateway нельзя.
* `nodeNetworkCIDR` — подсеть, в которой будут работать узлы кластера.
  * Диапазон должен быть частью или должен соответствовать диапазону адресов VPC.
  * Диапазон будет равномерно разбит на подсети по одной на Availability Zone в вашем регионе.
  * Необязательный, но рекомендованный параметр. По умолчанию — соответствует целому диапазону адресов VPC.
> Если при создании кластера создаётся новая VPC и не указан `vpcNetworkCIDR`, то VPC будет создана с диапазоном, указанным в `nodeNetworkCIDR`.
> Таким образом, вся VPC будет выделена под сети кластера и, соответственно, не будет возможности добавить другие ресурсы в эту VPC.
>
> Диапазон `nodeNetworkCIDR` распределяется по подсетям в зависимости от количества зон доступности в выбранном регионе. Например,
> если указана `nodeNetworkCIDR: "10.241.1.0/20"` и в регионе 3 зоны доступности, то подсети будут созданы с маской `/22`.
* `sshPublicKey` — публичный ключ для доступа на узлы.
* `tags` — теги, которые будут присвоены всем созданным ресурсам. Если поменять теги в рабочем кластере, то после конвержа
    необходимо пересоздать все машины, чтобы теги применились.
* `zones` — ограничение набора зон, в которых разрешено создавать узлы.
  * Опциональный параметр.
  * Формат — массив строк.

