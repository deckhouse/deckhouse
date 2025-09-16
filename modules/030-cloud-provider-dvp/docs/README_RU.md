---
title: "Cloud provider — DVP"
description: "Интеграция Deckhouse Kubernetes Platform с платформой виртуализации Deckhouse Virtualization Platform. Развертывание кластеров DKP поверх DVP."
---

Взаимодействие с облачными ресурсами провайдера [DVP](https://deckhouse.ru/products/virtualization-platform/) осуществляется с помощью модуля `cloud-provider-dvp`. Он позволяет [модулю управления узлами `node-manager`](../../modules/040-node-manager/) задействовать ресурсы DVP при создании узлов для [заданной группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Основные возможности модуля `cloud-provider-dvp`:

- управление ресурсами DVP через модуль `cloud-controller-manager`;
- заказ дисков с использованием компонента `CSI storage`;
- интеграция с [модулем `node-manager`](../../modules/040-node-manager/) для поддержки [DVPInstanceClass](cr.html#dvpinstanceclass) при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
