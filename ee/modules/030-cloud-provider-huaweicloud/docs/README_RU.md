---
title: "Cloud provider — Huawei Cloud"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Huawei Cloud."
---

Модуль `cloud-provider-huaweicloud` обеспечивает интеграцию Deckhouse Kubernetes Platform с [Huawei Cloud](https://www.huaweicloud.com/). Он позволяет модулю [`node-manager`](/modules/node-manager/) использовать ресурсы Huawei Cloud при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-huaweicloud`:

- Управление ресурсами Huawei Cloud через `cloud-controller-manager`:
  - актуализирует метаданные инстансов и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в облаке;
  - создаёт балансировщики нагрузки (ELB) для Service с типом LoadBalancer.
- Заказ дисков через CSI-драйвер EVS (`evs.csi.huaweicloud.com`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [HuaweiCloudInstanceClass](cr.html#huaweicloudinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы HuaweiCloudInstanceClass можно было указывать при описании NodeGroup.
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/) в режиме `VXLAN`.
