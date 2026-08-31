---
title: "Cloud provider — Huawei Cloud: FAQ"
---

## Как настроить политики безопасности на узлах кластера?

Правила групп безопасности по умолчанию описаны в разделе [схем размещения](layouts.html).

Для CloudEphemeral-узлов дополнительные группы безопасности задаются в ресурсе [`HuaweiCloudInstanceClass`](cr.html#huaweicloudinstanceclass) параметром [`spec.securityGroups`](cr.html#huaweicloudinstanceclass-v1-spec-securitygroups). Они применяются вместе с группой безопасности, созданной модулем.
