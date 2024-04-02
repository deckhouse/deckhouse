* Для кластера создаётся отдельная [resource group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal).
* По умолчанию каждому инстансу динамически выделяется один внешний IP-адрес, который используется только для доступа в интернет. На каждый IP для SNAT доступно 64000 портов.
* Поддерживается [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([тарификация](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)). Позволяет использовать статические публичные IP для SNAT.
* Публичные IP адреса можно назначить на master-узлы и узлы, созданные Terraform.
* Если master не имеет публичного IP, то для установки и доступа в кластер, необходим дополнительный инстанс с публичным IP (aka bastion). В этом случае так же потребуется настроить peering между VNet кластера и VNet bastion.
* Между VNet кластера и другими VNet можно настроить peering.
