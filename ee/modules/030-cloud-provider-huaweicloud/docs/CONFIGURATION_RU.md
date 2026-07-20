---
title: "Cloud provider — Huawei Cloud: настройки"
force_searchable: true
---

Модуль автоматически включается для всех облачных кластеров, развернутых в Huawei Cloud.

## Список необходимых сервисов Huawei Cloud

Список сервисов, необходимых для работы Deckhouse Kubernetes Platform:

| Сервис                           | Версия API |
|:---------------------------------|:----------:|
| Identity                         | [v3](https://support.huaweicloud.com/intl/en-us/api-iam/iam_30_0001.html) |
| Compute                          | [v2.1](https://support.huaweicloud.com/intl/en-us/api-ecs/ecs_04_0001.html) |
| Network (VPC)                    | [v1/v2](https://support.huaweicloud.com/intl/en-us/api-vpc/vpc-api-pdf.pdf) |
| Network (OpenStack Neutron API)  | [v2.0](https://support.huaweicloud.com/intl/en-us/api-vpc/vpc-api-pdf.pdf) |
| Block Storage                    | [v3](https://support.huaweicloud.com/intl/en-us/api-evs/evs_04_2065.html) |
| Load Balancing (ELB) *           | [v3](https://support.huaweicloud.com/intl/en-us/api-elb/CreateLoadBalancer.html) |

\* Требуется, если в кластере необходимо заказывать балансировщики нагрузки.

{% include module-alerts.liquid %}

Модуль не имеет настроек.
