---
title: "Cloud provider — VMware vSphere"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform на базе VMware vSphere."
---

Взаимодействие с облачными ресурсами провайдера на базе VMware vSphere осуществляется с помощью модуля `cloud-provider-vsphere`. Он предоставляет возможность модулю [управления узлами](/node-manager/) использовать ресурсы vSphere при заказе узлов для описанной [группы узлов](/node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-vsphere`:

- Управляет ресурсами vSphere с помощью модуля `cloud-controller-manager`:
  - Создает сетевые маршруты для сети PodNetwork на стороне vSphere.
  - Актуализирует метаданные vSphere VirtualMachines и Kubernetes Nodes. Удаляет из Kubernetes узлы, которых уже нет в vSphere.
- Заказывает диски в vSphere на datastore через механизм First-Class Disk с помощью компонента `CSI storage`.
- Регистрируется в модуле [`node-manager`](/node-manager/), чтобы [VsphereInstanceClass'ы](cr.html#vsphereinstanceclass) можно было использовать при описании [NodeGroup](/node-manager/cr.html#nodegroup).

{% alert level="warning" %}
Модуль находится в процессе миграции управления CloudEphemeral-узлами с Machine Controller Manager (MCM) на Cluster API (CAPI). Существующие NodeGroup продолжают использовать MCM, а новые по умолчанию создаются с использованием CAPI. Порядок миграции существующих групп — в разделе [«Как мигрировать группы узлов на Cluster API (CAPI)»](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).
{% endalert %}

{% alert level="info" %}
**Паритет vCenter-тегов для CAPI-узлов.** Под CAPI на каждую VM ставится тег `deckhouse-cluster-name/<clusterUUID>` (как и в MCM). Тег `deckhouse-node-role/<nodeGroup>-<zone>`, который MCM ставил дополнительно, в CAPI-варианте пока не воспроизводится — для группировки узлов по NodeGroup используйте Kubernetes-лейбл `node.deckhouse.io/group`. Полный паритет тегов — в отдельном follow-up.

**Поля размещения `VsphereInstanceClass` под CAPI.** `spec.resourcePool` учитывается: если оно задано, модуль создаёт отдельный `VSphereDeploymentZone` под каждый NodeGroup для каждой зоны, в которой NodeGroup работает, с `placementConstraint.resourcePool` из InstanceClass, и указывает `MachineDeployment` NodeGroup-а на этот DZ. `spec.datastore` **не** учитывается — CAPV читает datastore из `VSphereFailureDomain.spec.topology.datastore`, а этот объект один на зону и иммутабелен через webhook. Все NodeGroup-ы в одной зоне поэтому используют один зонный datastore; чтобы разложить NodeGroup-ы по разным datastore, используйте разные зоны.
{% endalert %}
