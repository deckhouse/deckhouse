---
title: "Cloud provider — OpenStack"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью OpenStack."
---

Взаимодействие с облачными ресурсами провайдеров на базе [OpenStack](https://www.openstack.org/) осуществляется с помощью модуля `cloud-provider-openstack`. Он предоставляет возможность модулю [управления узлами](/node-manager/) использовать ресурсы OpenStack при заказе узлов для описанной [группы узлов](/node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-openstack`:

- Управляет ресурсами OpenStack с помощью модуля `cloud-controller-manager`:
  - Актуализирует метаданные OpenStack Servers и Kubernetes Nodes. Удаляет из Kubernetes узлы, которых уже нет в OpenStack.
- Заказывает диски в Cinder (block) OpenStack с помощью компонента `CSI storage`. Manilla (filesystem) пока не поддерживается. CSI-драйвер Cinder поддерживает повторную аутентификацию в OpenStack с обновлением сервисного каталога, что повышает устойчивость операций с томами в подах, работающих длительное время без перезапуска.
- Регистрируется в модуле [node-manager](/node-manager/), чтобы [OpenStackInstanceClass'ы](cr.html#openstackinstanceclass) можно было использовать при описании [NodeGroup](/node-manager/cr.html#nodegroup).

{% alert level="warning" %}
Модуль находится в процессе миграции управления узлами типа CloudEphemeral с MCM на Cluster API (CAPI). Существующие NodeGroup остаются на MCM; новые по умолчанию используют CAPI. Миграция существующих групп выполняется через пересоздание NodeGroup.

Инструкция: [Как мигрировать группы узлов на Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).
{% endalert %}
