---
title: "Cloud provider — Azure"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Microsoft Azure."
---

Модуль `cloud-provider-azure` обеспечивает интеграцию Deckhouse Kubernetes Platform с [Microsoft Azure](https://portal.azure.com/). Он предоставляет возможность модулю [`node-manager`](/modules/node-manager/) использовать ресурсы Azure при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-azure`:

- Управление ресурсами Azure через `cloud-controller-manager`:
  - создаёт сетевые маршруты для сети `PodNetwork` на стороне Azure;
  - создаёт балансировщики нагрузки для Service с типом LoadBalancer;
  - актуализирует метаданные узлов кластера и удаляет из Kubernetes узлы, которых больше нет в Azure.
- Заказ дисков через CSI-драйвер Azure Disk (`disk.csi.azure.com`) и создание StorageClass для типов дисков Azure, чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM). Параметры виртуальных машин задаются в ресурсе [AzureInstanceClass](/modules/cloud-provider-azure/cr.html#azureinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы [AzureInstanceClass](/modules/cloud-provider-azure/cr.html#azureinstanceclass) можно было указывать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).

{% alert level="warning" %}
Для корректной работы утилит вроде `ntpdate` и `chrony` убедитесь, что у балансировщика нагрузки есть правила для UDP-трафика. Если исходящий UDP блокируется, добавьте правило к существующему балансировщику или создайте Service с типом LoadBalancer и UDP-портом.
{% endalert %}
