---
title: "Cloud provider — Huawei Cloud"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Huawei Cloud."
---

Взаимодействие с облачными ресурсами провайдеров на базе [Huawei Cloud](https://www.huaweicloud.com/intl/en-us/) осуществляется с помощью модуля `cloud-provider-huaweicloud`. Он позволяет [модулю управления узлами](/modules/node-manager/) задействовать ресурсы Huawei Cloud при создании узлов для [заданной группы узлов](/modules/node-manager/cr.html#nodegroup).

Основные возможности модуля `cloud-provider-huaweicloud`:

- управление ресурсами Huawei Cloud через `cloud-controller-manager`;
- заказ дисков с использованием компонента `CSI storage`;
- интеграция с [модулем node-manager](/modules/node-manager/) для поддержки [HuaweiCloudInstanceClass](cr.html#huaweicloudinstanceclass) при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
