---
title: "Cloud provider — VMware vSphere"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform на базе VMware vSphere."
---

Взаимодействие с облачными ресурсами провайдера на базе VMware vSphere осуществляется с помощью модуля `cloud-provider-vsphere`. Он предоставляет возможность модулю [управления узлами](../../modules/node-manager/) использовать ресурсы vSphere при заказе узлов для описанной [группы узлов](../../modules/node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-vsphere`:
- Управляет ресурсами vSphere с помощью модуля `cloud-controller-vsphere`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне vSphere.
  * Актуализирует метаданные vSphere VirtualMachines и Kubernetes Nodes. Удаляет из Kubernetes узлы, которых уже нет в vSphere.
- Заказывает диски в vSphere на datastore через механизм First-Class Disk с помощью компонента `CSI storage`.
- Регистрируется в модуле [node-manager](../../modules/node-manager/), чтобы [VsphereInstanceClass'ы](cr.html#vsphereinstanceclass) можно было использовать при описании [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
