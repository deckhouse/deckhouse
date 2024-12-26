---
title: "Cloud provider — HuaweiCloud"
---

Взаимодействие с облачными ресурсами провайдеров на базе [HuaweiCloud](https://www.huaweicloud.com/intl/en-us/) осуществляется с помощью модуля `cloud-provider-huaweicloud`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы HuaweiCloud при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-huaweicloud`:

- Управляет ресурсами OpenStack с помощью модуля `cloud-controller-manager`:
- Заказывает диски с помощью компонента `CSI storage`.
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [HuaweicloudInstanceClass'ы](cr.html#huaweicloudinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
