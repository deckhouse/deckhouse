---
title: Интеграция со службами Huawei Cloud
permalink: ru/admin/integrations/private/huaweicloud/services.html
lang: ru
---

Deckhouse Kubernetes Platform использует облачные возможности Huawei Cloud для работы Kubernetes-кластера. При этом поддерживаются следующие функции:

- управление ресурсами Huawei Cloud через `cloud-controller-manager`;
- заказ дисков с использованием CSI-драйвера;
- интеграция с модулем `node-manager`, позволяющая использовать [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) в [NodeGroup](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference).

## Работа с InstanceClass

Для описания параметров виртуальных машин используется [ресурс HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass). На него ссылается [параметр cloudInstances.classReference](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) NodeGroup.

Пример ресурса:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudInstanceClass
metadata:
  name: worker
spec:
  imageName: alt-p11
  flavorName: s7n.xlarge.2
  rootDiskSize: 50
  rootDiskType: SSD
```
