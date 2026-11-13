---
title: Интеграция со службами Huawei Cloud
permalink: ru/admin/integrations/private/huaweicloud/services.html
lang: ru
---

Deckhouse Platform использует облачные возможности Huawei Cloud для работы Kubernetes-кластера. При этом поддерживаются следующие функции:

- управление ресурсами Huawei Cloud через `cloud-controller-manager`;
- заказ дисков с использованием [CSI-драйвера](../../../../glossary.html#csi-driver);
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

## Замена образа ОС

Образ ОС узлов задаётся параметром [`imageName`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-imagename) ресурса HuaweiCloudInstanceClass для узлов типа `CloudEphemeral` и в секции `instanceClass` ресурса HuaweiCloudClusterConfiguration для узлов типа `CloudPermanent` и master-узлов. Чтобы перевести узлы кластера на новый образ, выполните шаги из раздела [«Как заменить образ ОС в облачном кластере?»](../../../../faq.html#как-заменить-образ-ос-в-облачном-кластере).
