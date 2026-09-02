---
title: "Cloud provider — Huawei Cloud"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Huawei Cloud."
---

Модуль `cloud-provider-huaweicloud` обеспечивает интеграцию Deckhouse Kubernetes Platform с [Huawei Cloud](https://www.huaweicloud.com/). Он предоставляет возможность модулю [`node-manager`](/modules/node-manager/) использовать ресурсы Huawei Cloud при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-huaweicloud`:

- Управление ресурсами Huawei Cloud через `cloud-controller-manager`:
  - актуализирует метаданные инстансов и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в облаке;
  - создаёт балансировщики нагрузки (ELB) для Service с типом LoadBalancer.
- Заказ дисков через CSI-драйвер EVS (`evs.csi.huaweicloud.com`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-huaweicloud.html#взаимодействия-модуля) `terraform-provider-huaweicloud`.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) можно было указывать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
