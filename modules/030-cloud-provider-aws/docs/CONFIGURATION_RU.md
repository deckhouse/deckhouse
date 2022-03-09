---
title: "Cloud provider — AWS: настройки"
---

## Параметры

Модуль настраивается автоматически исходя из выбранной схемы размещения (custom resource `AWSClusterConfiguration`). В большинстве случаев нет необходимости ручной конфигурации модуля.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера AWS — это custom resource [`AWSInstanceClass`](cr.html#awsinstanceclass), в котором указываются конкретные параметры самих машин.

## Storage

Модуль автоматически создаёт StorageClasses, которые есть в AWS: `gp3`, `gp2`, `sc1` и `st1`. Позволяет сконфигурировать диски с необходимым IOPS. А также отфильтровать ненужные StorageClass, указанием их в параметре `exclude`.

<!-- SCHEMA -->
