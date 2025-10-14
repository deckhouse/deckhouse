---
title: "Cloud provider — DVP"
description: "Интеграция Deckhouse Platform Certified Security Edition с платформой виртуализации. Развертывание кластеров Deckhouse Platform Certified Security Edition поверх Deckhouse Platform Certified Security Edition."
---

Взаимодействие с облачными ресурсами провайдера [DVP](https://deckhouse.ru/products/virtualization-platform/) осуществляется с помощью модуля `cloud-provider-dvp`. Он позволяет [модулю управления узлами `node-manager`](/modules/node-manager/) задействовать ресурсы DVP при создании узлов для [заданной группы узлов](/modules/node-manager/cr.html#nodegroup).

Основные возможности модуля `cloud-provider-dvp`:

- управление ресурсами DVP через модуль `cloud-controller-manager`;
- заказ дисков с использованием компонента `CSI storage`;
- интеграция с [модулем `node-manager`](/modules/node-manager/) для поддержки [DVPInstanceClass](cr.html#dvpinstanceclass) при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
