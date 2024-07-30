* A separate [resource group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal) is created for the cluster.
* By default, one external IP address is dynamically allocated to each instance (it is used for Internet access only). Each IP has 64000 ports available for SNAT.
* The [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([pricing](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)) is supported. With it, you can use static public IP addresses for SNAT.
* Public IP addresses can be assigned to master nodes and nodes created by Terraform.
* If the master does not have a public IP, then an additional instance with a public IP (aka bastion host) is required for installation tasks and access to the cluster. In this case, you will also need to configure peering between the cluster's VNet and bastion's VNet.
* Peering can also be configured between the cluster VNet and other VNets.
