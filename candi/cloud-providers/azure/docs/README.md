title: "Cloud provider — Azure: Развертывание"

**Внимание!** Поддерживаются только [регионы](https://docs.microsoft.com/en-us/azure/availability-zones/az-region) в которых доступны `Availability Zones`.

## Поддерживаемые схемы размещения

Схема размещения описывается объектом AzureClusterConfiguration.

Его поля:
* `apiVersion` — deckhouse.io/v1alpha1
* `kind` — AzureClusterConfiguration
* `layout` — архитектура расположения ресурсов в облаке.
    * Варианты — `Standard` (описание ниже).
* `sshPublicKey` — публичный ключ для доступа на ноды под пользователем `azureuser`.
    * Обязательный параметр.
* `vNetCIDR` — адресное пространство виртуальной сети в формате [CIDR](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing).
    * Обязательный параметр.
* `subnetCIDR` — подсеть из адресного пространства `vNetCIDR`, в которой будут работать ноды кластера.
    * Обязательный параметр.
* `standard` — настройки для лейаута `Standard`.
    * `natGatewayPublicIpCount` — количество IP-адресов для [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([тарификация](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)).
    * По умолчанию `0` (`NAT Gateway` не используется).
    * Опциональный параметр.
* `tags` — список тегов в формате `key: value`, которые будут назначены всем ресурсам кластера.
* `masterNodeGroup` — спека для описания NG мастера.
    * `replicas` — сколько мастер-узлов создать.
    * `zones` — список зон, в которых допустимо создавать мастер-узлы.
        * Доступные зоны для выбранного типа инстанса можно посмотреть с помощью [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli):
            * `az vm list-skus -l westeurope -o table`
        * Значение по умолчанию `[1,2,3]`
    * `instanceClass` — частичное содержимое полей [AzureInstanceClass](/modules/030-cloud-provider-azure/docs#azureinstanceclass-custom-resource).  Параметры, обозначенные **жирным** шрифтом уникальны для `AzureClusterConfiguration`. Допустимые параметры:
        * `machineSize`
        * `diskSizeGb`
        * `urn`
        * **`enableExternalIP`** — параметр доступен только для layout `Standard`.
            * `false` —  значение по умолчанию. Узлы не имеют публичных адресов, доступ в интернет осуществляется через NAT.
            * `true` — для узлов создаются статические публичные адреса.
        * `additionalTags` — список дополнительных тегов в формате `key: value`, которые будут назначены инстансам.
* `nodeGroups` — массив дополнительных NG для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NG:
    * `name` — имя NG, будет использоваться для генерации имен нод.
    * `replicas` — количество нод.
    * `zones` — список зон, в которых допустимо создавать статичные-узлы.
        * Доступные зоны для выбранного типа инстанса можно посмотреть с помощью [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli):
            * `az vm list-skus -l westeurope -o table`
        * Значение по умолчанию `[1,2,3]`
    * `instanceClass` — частичное содержимое полей [AzureInstanceClass](/modules/030-cloud-provider-azure/docs#azureinstanceclass-custom-resource).  Параметры, обозначенные **жирным** шрифтом уникальны для `AzureClusterConfiguration`. Допустимые параметры:
        * `machineSize`
        * `diskSizeGb`
        * `urn`
        * **`enableExternalIP`** — параметр доступен только для layout `Standard`.
            * `false` —  значение по умолчанию. Узлы не имеют публичных адресов, доступ в интернет осуществляется через NAT.
            * `true` — для узлов создаются статические публичные адреса.
        * `additionalTags` — список дополнительных тегов в формате `key: value`, которые будут назначены инстансам.
    * `nodeTemplate` — настройки Node объектов в Kubernetes, которые будут добавлены после регистрации ноды.
      * `labels` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) `metadata.labels`
        * Пример:
          ```yaml
          labels:
            environment: production
            app: warp-drive-ai
          ```
      * `annotations` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) `metadata.annotations`
        * Пример:
          ```yaml
          annotations:
            ai.fleet.com/discombobulate: "true"
          ```
      * `taints` — аналогично полю `.spec.taints` из объекта [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#taint-v1-core). **Внимание!** Доступны только поля `effect`, `key`, `values`.
        * Пример:
          ```yaml
          taints:
          - effect: NoExecute
            key: ship-class
            value: frigate
          ```
* `provider` — параметры подключения к API Azure.
    * `subscriptionId` — Идентификатор подписки.
    * `clientId` — Идентификатор клиента.
    * `clientSecret` — Секрет клиента.
    * `tenantId` — Идентификатор тенанта.
    * `location` — имя региона, в котором будут созданы все ресурсы.
* `peeredVNets` — массив `VNet's`, с которыми будет объединена сеть кластера. Сервис-аккаунт должен иметь доступ ко всем перечисленным `VNet`. Если доступа нет, то пиринг необходимо [настраивать вручную](https://docs.microsoft.com/en-us/azure/virtual-network/virtual-network-peering-overview).
    * `resourceGroupName` — имя группы ресурсов, в которой находится VNet.
    * `vnetName` — имя VNet.

### Standard
* Для кластера создаётся отдельная [resorce group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal).
* По умолчанию каждому инстансу динамически выделается один внешний IP-адрес, который используется только для доступа в интернет. На каждый IP для SNAT доступно 64000 портов.
* Поддерживается [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([тарификация](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)). Позволяет использовать статические публичные IP для SNAT.
* Публичные IP адреса можно назначить на master-ноды и ноды созданные терраформом.
* Если master не имеет публичного IP, то для установки и доступа в кластер, необходим дополнительный инстанс с публичным IP (aka bastion). В этом случае так же потребуется настроить peering между VNet кластера и VNet bastion.
* Между VNet кластера и другими VNet можно настроить peering.

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa ..." # required
vNetCIDR: 10.50.0.0/16 # required
subnetCIDR: 10.50.0.0/24 # required
standard:
  natGatewayPublicIpCount: 1 # optional, by default 0
masterNodeGroup:
  replicas: 1
  zones: [1] # optional, by default [1]
  instanceClass:
    machineSize: Standard_F4 # required
    diskSizeGb: 32
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140 # required
    enableExternalIP: false # optional, by default true
provider:
  subscriptionId: "" # required
  clientId: "" # required
  clientSecret: "" # required
  tenantId: "" # required
  location: "westeurope" # required
peeredVNets: # optional, list of Azure VNets with which Kubernetes VNet will be peered
  - resourceGroupName: kube-bastion # required
    vnetName: kube-bastion-vnet # required
```


## Создание сервис-аккаунта

### [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)

```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/SUBSCRIPTION_ID" --name "NAME"
```
