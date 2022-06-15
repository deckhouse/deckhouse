![resources](https://docs.google.com/drawings/d/e/2PACX-1vR1oHqbXPJPYxUXwpkRGM6VPpZaNc8WoGH-N0Zqb9GexSc-NQDvsGiXe_Hc-Z1fMQWBRawuoy8FGENt/pub?w=989&amp;h=721)
<!--- Исходник: https://docs.google.com/drawings/d/1VTAoz6-65q7m99KA933e1phWImirxvb9-OLH9DRtWPE/edit --->

* Для кластера создаётся отдельная VPC с [Cloud NAT](https://cloud.google.com/nat/docs/overview).
* Узлы в кластере не имеют публичных IP адресов.
* Публичные IP адреса можно назначить на master и статические узлы.
  * При этом будет использоваться One-to-one NAT для отображения публичного IP-адреса в IP-адрес узла (следует помнить, что CloudNAT в этом случае использоваться не будет).
* Если master не имеет публичного IP, то для установки и доступа в кластер, необходим дополнительный инстанс с публичным IP (aka bastion).
* Между VPC кластера и другими VPC можно настроить peering.
