---
title: "Cloud provider — AWS"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Amazon AWS."
---

Модуль `cloud-provider-aws` обеспечивает интеграцию Deckhouse Kubernetes Platform с [Amazon AWS](https://aws.amazon.com/). Он предоставляет возможность модулю [`node-manager`](/modules/node-manager/) использовать ресурсы AWS при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-aws`:

- Управление ресурсами AWS через `cloud-controller-manager`:
  - создаёт сетевые маршруты для сети `PodNetwork` на стороне AWS;
  - создаёт балансировщики нагрузки для Service с типом LoadBalancer;
  - актуализирует метаданные узлов кластера и удаляет из Kubernetes узлы, которых больше нет в AWS.
- Заказ дисков через CSI-драйвер EBS (`ebs.csi.aws.com`) и создание StorageClass для типов томов AWS, чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM). Параметры виртуальных машин задаются в ресурсе [AWSInstanceClass](/modules/cloud-provider-aws/cr.html#awsinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы [AWSInstanceClass](/modules/cloud-provider-aws/cr.html#awsinstanceclass) можно было указывать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
