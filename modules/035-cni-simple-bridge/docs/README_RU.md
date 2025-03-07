---
title: "Модуль cni-simple-bridge"
---

Модуль не имеет настроек.

Во время бутстрапа нового кластера, если явно не описан другой CNI, то `simple-bridge` будет использован по умолчанию для следующих облачных провайдеров:
- [AWS](../../modules/cloud-provider-aws/).
- [Azure](../../modules/cloud-provider-azure/).
- [GCP](../../modules/cloud-provider-gcp/).
- [Yandex](../../modules/cloud-provider-yandex/).
