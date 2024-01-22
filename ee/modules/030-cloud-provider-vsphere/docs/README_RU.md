---
title: "Cloud provider — VMware vSphere"
---

Взаимодействие с облачными ресурсами провайдера на базе VMware vSphere осуществляется с помощью модуля `cloud-provider-vsphere`, который предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы vSphere при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-vsphere`:

* Управляет ресурсами vSphere с помощью модуля `cloud-controller-vsphere`:

  * Создает сетевые маршруты для сети `PodNetwork` на стороне vSphere.

  * Актуализирует метаданные vSphere VirtualMachines и Kubernetes Nodes. Удаляет из Kubernetes узлы, которые отсутствуют в vSphere.

* Заказывает диски в vSphere на datastore через механизм First-Class Disk с помощью компонента `CSI storage`.

* Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [VsphereInstanceClass'ы](cr.html#vsphereinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
