Deckhouse Platform интегрируется с [{{ site.data.admin.cloud-types.types[page.cloud_type].name }}]({{ site.data.admin.cloud-types.types[page.cloud_type].link }}) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами {{ site.data.admin.cloud-types.types[page.cloud_type].name }}:
  - актуализация метаданных серверов {{ site.data.admin.cloud-types.types[page.cloud_type].name }} и узлов кластера, удаление из кластера узлов, которых больше нет в {{ site.data.admin.cloud-types.types[page.cloud_type].name }};
  - создание балансировщиков нагрузки (Octavia) для Service с типом LoadBalancer.
- Заказ блочных дисков через CSI-драйвер Cinder (`cinder.csi.openstack.org`), чтобы из кластера можно было заказывать PersistentVolume. Manila (filesystem) не поддерживается.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-openstack.html#взаимодействия-модуля) `terraform-provider-openstack`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM) или Cluster API (CAPI).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/), режим сети зависит от параметра [`podNetworkMode`](/modules/cloud-provider-openstack/configuration.html#parameters-podnetworkmode).
