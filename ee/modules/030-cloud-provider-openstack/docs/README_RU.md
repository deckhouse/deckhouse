---
title: "Cloud provider — OpenStack"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью OpenStack."
---

Взаимодействие с облачными ресурсами провайдеров на базе [OpenStack](https://www.openstack.org/) осуществляется с помощью модуля `cloud-provider-openstack`. Он предоставляет возможность модулю [управления узлами](../../modules/node-manager/) использовать ресурсы OpenStack при заказе узлов для описанной [группы узлов](../../modules/node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-openstack`:
- Управляет ресурсами OpenStack с помощью модуля `cloud-controller-manager`:
  * Актуализирует метаданные OpenStack Servers и Kubernetes Nodes. Удаляет из Kubernetes узлы, которых уже нет в OpenStack.
- Заказывает диски в Cinder (block) OpenStack с помощью компонента `CSI storage`. Manilla (filesystem) пока не поддерживается.
- Регистрируется в модуле [node-manager](../../modules/node-manager/), чтобы [OpenStackInstanceClass'ы](cr.html#openstackinstanceclass) можно было использовать при описании [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
