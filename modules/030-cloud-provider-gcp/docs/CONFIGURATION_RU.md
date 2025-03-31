---
title: "Cloud provider — GCP: настройки"
---

Модуль настраивается автоматически, исходя из выбранной схемы размещения, определяемой в параметрах структуры [GCPClusterConfiguration](cluster_configuration.html). В большинстве случаев нет необходимости ручной конфигурации модуля.

{% include module-alerts.liquid %}

{% include module-conversion.liquid %}

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../../modules/node-manager/cr.html#nodegroup) модуля `node-manager`, в котором также указывается название используемого для этой группы узлов инстанс-класса (параметр `cloudInstances.classReference` NodeGroup). Инстанс-класс для cloud-провайдера GCP — это custom resource [`GCPInstanceClass`](cr.html#gcpinstanceclass), в котором указываются конкретные параметры самих машин.

Модуль автоматически создает StorageClass'ы, покрывающие все варианты дисков в GCP:

| Тип | Репликация | Имя StorageClass |
|---|---|---|
| standard | none | pd-standard-not-replicated |
| standard | regional | pd-standard-replicated |
| balanced | none | pd-balanced-not-replicated |
| balanced | regional | pd-balanced-replicated |
| ssd | none | pd-ssd-not-replicated |
| ssd | regional | pd-ssd-replicated |

Также он позволяет отфильтровать ненужные StorageClass'ы, для этого нужно указать их в параметре `exclude`.

{% include module-settings.liquid %}
