---
title: "Сloud provider — OpenStack: настройки"
---

Модуль автоматически включается для всех облачных кластеров развёрнутых в OpenStack.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера OpenStack — это custom resource [`OpenStackInstanceClass`](cr.html#openstackinstanceclass), в котором указываются конкретные параметры самих машин.

## Параметры

<!-- SCHEMA -->
