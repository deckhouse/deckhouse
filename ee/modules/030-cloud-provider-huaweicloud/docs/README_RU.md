---
title: "Cloud provider — Huawei Cloud"
---

Взаимодействие с облачными ресурсами провайдеров на базе [Huawei Cloud](https://www.huaweicloud.com/intl/en-us/) осуществляется с помощью модуля `cloud-provider-huaweicloud`. Он позволяет [модулю управления узлами](../../modules/040-node-manager/)задействовать ресурсы Huawei Cloud при создании узлов для [заданной группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Основные возможности модуля `cloud-provider-huaweicloud`:

- Управление ресурсами Huawei Cloud через `cloud-controller-manager`;
- Заказ дисков с использованием компонента `CSI storage`;
- Интеграция с [модулем node-manager](../../modules/040-node-manager/) для поддержки [HuaweicloudInstanceClass](cr.html#huaweicloudinstanceclass) при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
