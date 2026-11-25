---
title: Интеграция с Huawei Cloud
permalink: ru/admin/integrations/private/huaweicloud/overview.html
lang: ru
---

Deckhouse Platform интегрируется с [Huawei Cloud](https://www.huaweicloud.com/) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами Huawei Cloud:
  - актуализация метаданных инстансов и узлов кластера, удаление из кластера узлов, которых больше нет в облаке;
  - создание балансировщиков нагрузки (ELB) для Service с типом LoadBalancer.
- Заказ дисков через CSI-драйвер EVS (`evs.csi.huaweicloud.com`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-huaweicloud.html#взаимодействия-модуля) `terraform-provider-huaweicloud`.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
