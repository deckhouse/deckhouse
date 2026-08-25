---
title: "Cloud provider — GCP"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Google Cloud Platform."
---

Модуль `cloud-provider-gcp` обеспечивает интеграцию Deckhouse Kubernetes Platform с [Google Cloud Platform](https://cloud.google.com/). Он позволяет модулю [node-manager](/modules/node-manager/) использовать ресурсы GCP при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-gcp`:

- Управление ресурсами GCP через `cloud-controller-manager`:
  - создаёт сетевые маршруты для сети `PodNetwork` на стороне GCP;
  - создаёт балансировщики нагрузки для Service с типом `LoadBalancer`;
  - актуализирует метаданные узлов кластера и удаляет из Kubernetes узлы, которых больше нет в GCP.
- Заказ дисков через CSI-драйвер Persistent Disk (`pd.csi.storage.gke.io`) и создание StorageClass для типов дисков GCP, чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM). Параметры виртуальных машин задаются в ресурсе [GCPInstanceClass](cr.html#gcpinstanceclass).
- Регистрация в модуле [node-manager](/modules/node-manager/), чтобы `GCPInstanceClass` можно было указывать при описании `NodeGroup`.
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
