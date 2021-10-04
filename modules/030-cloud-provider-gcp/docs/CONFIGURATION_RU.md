---
title: "Сloud provider — GCP: настройки"
---

Модуль настраивается автоматически исходя из выбранной схемы размещения определяемой в параметрах структуры [GCPClusterConfiguration](cluster_configuration.html). В большинстве случаев нет необходимости ручной конфигурации модуля.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера GCP — это custom resource [`GCPInstanceClass`](cr.html#gcpinstanceclass), в котором указываются конкретные параметры самих машин.

Модуль автоматически создаёт StorageClasses, покрывающие все варианты дисков в GCP:

| Тип | Репликация | Имя StorageClass |
|---|---|---|
| standard | none | pd-standard-not-replicated |
| standard | regional | pd-standard-replicated |
| balanced | none | pd-balanced-not-replicated |
| balanced | regional | pd-balanced-replicated |
| ssd | none | pd-ssd-not-replicated |
| ssd | regional | pd-ssd-replicated |

А также позволяет отфильтровать ненужные StorageClass, указанием их в параметре `exclude`.

## Параметры

<!-- SCHEMA -->
