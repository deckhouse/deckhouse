---
title: Интеграция с VMware Cloud Director
permalink: ru/admin/integrations/virtualization/vcd/overview.html
lang: ru
---

Deckhouse Platform интегрируется с [VMware Cloud Director](https://www.vmware.com/products/cloud-director.html) (VCD) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами VCD:
  - актуализация метаданных виртуальных машин и узлов кластера, удаление из кластера узлов, которых больше нет в VCD;
  - создание балансировщиков нагрузки для Service с типом LoadBalancer. Для этого используется VMware NSX Advanced Load Balancer (Avi), поддержка доступна при использовании NSX-T.
- Заказ дисков через CSI-драйвер Named Disk (`named-disk.csi.cloud-director.vmware.com`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vcd.html#взаимодействия-модуля) `terraform-provider-vcd`. Базовая инфраструктура заказывается в зависимости от [схемы размещения](./layout.html).
- Заказ CloudEphemeral-узлов через Cluster API (CAPI).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).

{% alert level="info" %}
Для работы с API VCD версии ниже 37.2 платформа использует режим совместимости (legacy-компоненты).
{% endalert %}
