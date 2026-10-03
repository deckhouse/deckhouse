* Для кластера создаётся отдельная [resource group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal).
* По умолчанию каждому инстансу динамически выделяется один внешний IP-адрес, который используется только для доступа в интернет. На каждый IP-адрес для SNAT доступно 64000 портов.
* Поддерживается [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([тарификация](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)). Он позволяет использовать статические публичные IP-адреса для SNAT.
* Публичные IP-адреса можно назначить master-узлам и статическим узлам, описанным в параметре `nodeGroups` AzureClusterConfiguration.
* Если master-узел не имеет публичного IP-адреса, для установки и доступа в кластер необходим дополнительный инстанс с публичным IP-адресом (bastion-хост). В этом случае также потребуется настроить пиринг между VNet кластера и VNet bastion-хоста.
* Между VNet кластера и другими VNet можно настроить пиринг.
