## Группы безопасности

Параметр `internalNetworkSecurity` для схем [Standard](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-standard-internalnetworksecurity) и [StandardWithNoRouter](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-standardwithnorouter-internalnetworksecurity) (по умолчанию `true`) включает создание группы безопасности (SecurityGroup) при создании кластера. Deckhouse Platform создаёт группу с именем префикса кластера (`prefix`) и назначает её узлам.

Будут созданы следующие правила входящего трафика:

- разрешение входящего трафика по протоколу TCP и порту `22` из CIDR, указанных в [`sshAllowList`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-sshallowlist) (по умолчанию `0.0.0.0/0`);
- разрешение входящего трафика по протоколу ICMP из `0.0.0.0/0`;
- разрешение входящего трафика по протоколу TCP и портам `30000`–`32767` для использования сервисов типа `NodePort`. Входящий трафик по протоколу UDP на порты `NodePort` по умолчанию не разрешается;
- разрешение любого входящего трафика от узлов, входящих в ту же группу безопасности.

В схемах Simple и SimpleWithInternalNetwork Deckhouse Platform не создаёт группы безопасности. Подготовьте и подключите их самостоятельно, как описано ниже.

### Подключение собственных групп

К узлам можно подключить собственные группы безопасности. Подготовьте их в облаке заранее, следуя [документации OpenStack](https://docs.openstack.org/nova/latest/user/security-groups.html), и укажите в `additionalSecurityGroups`:

- для master-узлов — в параметре [`masterNodeGroup.instanceClass.additionalSecurityGroups`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-masternodegroup-instanceclass-additionalsecuritygroups) ресурса [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration);
- для статических узлов — в параметре [`nodeGroups[].instanceClass.additionalSecurityGroups`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-nodegroups-instanceclass-additionalsecuritygroups) ресурса [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration);
- для эфемерных узлов — в параметре [`spec.additionalSecurityGroups`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-additionalsecuritygroups) ресурса [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass).
