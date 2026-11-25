---
title: Интеграция с Microsoft Azure
permalink: ru/admin/integrations/public/azure/overview.html
lang: ru
---

Deckhouse Platform интегрируется с [Microsoft Azure](https://portal.azure.com/) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [AzureInstanceClass](/modules/cloud-provider-azure/cr.html#azureinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами Azure:
  - создание сетевых маршрутов для сети `PodNetwork` на стороне Azure;
  - создание балансировщиков нагрузки для Service с типом LoadBalancer;
  - актуализация метаданных узлов кластера, удаление из кластера узлов, которых больше нет в Azure.
- Заказ дисков через CSI-драйвер Azure Disk (`disk.csi.azure.com`) и создание StorageClass для типов дисков Azure, чтобы из кластера можно было заказывать PersistentVolume.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-azure.html#взаимодействия-модуля) `terraform-provider-azurerm`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
