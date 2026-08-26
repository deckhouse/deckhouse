---
title: "Cloud provider — VMware vSphere"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform на базе VMware vSphere."
---

Модуль `cloud-provider-vsphere` обеспечивает интеграцию Deckhouse Kubernetes Platform с [VMware vSphere](https://www.vmware.com/products/vsphere.html). Он предоставляет возможность модулю [`node-manager`](/modules/node-manager/) использовать ресурсы vSphere при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-vsphere`:

- Управление ресурсами vSphere через `cloud-controller-manager`:
  - создаёт сетевые маршруты для сети `PodNetwork` на стороне vSphere;
  - актуализирует метаданные виртуальных машин и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в vSphere.
- Заказ дисков через CSI на datastore. По умолчанию используются CNS-тома с изменением размера на лету. Режим First-Class Disk (FCD) доступен как legacy и настраивается параметром [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag).
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vsphere.html#взаимодействия-модуля) `terraform-provider-vsphere`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM). Параметры виртуальных машин задаются в ресурсе [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) можно было указывать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).

{% alert level="warning" %}
Модуль находится в процессе миграции управления CloudEphemeral-узлами с Machine Controller Manager (MCM) на Cluster API (CAPI). Существующие NodeGroup продолжают использовать MCM, а новые по умолчанию создаются с использованием CAPI. Порядок миграции существующих групп — в разделе [«Как мигрировать группы узлов на Cluster API (CAPI)»](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).
{% endalert %}

{% alert level="info" %}
**Паритет vCenter-тегов для CAPI-узлов.** Под CAPI на каждую VM ставится тег `deckhouse-cluster-name/<clusterUUID>` (как и в MCM). Тег `deckhouse-node-role/<nodeGroup>-<zone>`, который MCM ставил дополнительно, в CAPI-варианте пока не воспроизводится — для группировки узлов по NodeGroup используйте Kubernetes-лейбл `node.deckhouse.io/group`. Полный паритет тегов — в отдельном follow-up.

**Поля размещения `VsphereInstanceClass` под CAPI.** `spec.datastore` и `spec.resourcePool` игнорируются для NodeGroup-ов под управлением CAPI — CAPV перезаписывает их на каждом reconcile из привязанных `VSphereDeploymentZone` / `VSphereFailureDomain`. Возможность override через отдельные DeploymentZone на InstanceClass — в отдельном follow-up.
{% endalert %}
