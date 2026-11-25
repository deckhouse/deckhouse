---
title: Управление узлами в VMware Cloud Director
permalink: ru/admin/integrations/virtualization/vcd/node-management.html
lang: ru
---

Количество и параметры процесса заказа машин в облаке настраиваются в кастомном ресурсе [NodeGroup](/modules/node-manager/cr.html#nodegroup), в котором также указывается название используемого для этой [группы узлов](../../../../glossary.html#node-group) [инстанс-класса](../../../../glossary.html#instance-class) (параметр `cloudInstances.classReference`). Инстанс-класс для cloud-провайдера VCD — это кастомный ресурс [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass), в котором указываются конкретные параметры самих машин.

Ниже представлен пример конфигурации VCDInstanceClass для эфемерных узлов cloud-провайдера VMware Cloud Director.

## Пример конфигурации кастомного ресурса VCDInstanceClass

```yaml
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: test
spec:
  rootDiskSizeGb: 90
  sizingPolicy: payg-4-8
  storageProfile: SSD-dc1-pub1-cl1
  template: MyOrg/Linux/ubuntu2204-cloud-ova
```
