---
title: "Cloud provider — VMware vSphere"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform на базе VMware vSphere."
---

Модуль `cloud-provider-vsphere` обеспечивает интеграцию Deckhouse Kubernetes Platform с [VMware vSphere](https://www.vmware.com/products/vsphere.html). Он позволяет модулю [`node-manager`](/modules/node-manager/) использовать ресурсы vSphere при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-vsphere`:

- Управление ресурсами vSphere через `cloud-controller-manager`:
  - создаёт сетевые маршруты для сети `PodNetwork` на стороне vSphere;
  - актуализирует метаданные виртуальных машин и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в vSphere.
- Заказ дисков через CSI на datastore. По умолчанию используются CNS-тома с изменением размера на лету. Режим First-Class Disk (FCD) доступен как legacy и настраивается параметром `compatibilityFlag`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM). Параметры виртуальных машин задаются в ресурсе [VsphereInstanceClass](cr.html#vsphereinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы VsphereInstanceClass можно было указывать при описании NodeGroup.
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/) в режиме `DirectWithNodeRoutes`.
