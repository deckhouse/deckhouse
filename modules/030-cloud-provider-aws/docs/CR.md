---
title: "Сloud provider — AWS: Custom Resources"
---

## AWSInstanceClass

Описывает параметры instance в AWS, которые будет использовать `machine-controller-manager` (модуль [node-manager](/modules/040-node-manager/)) . На этот ресурс ссылается ресурс `CloudInstanceClass` модуля `node-manager`.

Параметры указываются в поле `spec`.

* `instanceType` — тип заказываемых instances. **Внимание!** Следует убедиться, что указанный тип есть во всех зонах, указанных в `zones`.
* `ami` — образ, который поставится в заказанные instance'ы.
    * Формат — строка, AMI ID.
    * Как найти нужный AMI: `aws ec2 --region <REGION> describe-images --filters 'Name=name,Values=buntu/images/hvm-ssd/ubuntu-bionic-18.04-amd64-server-2020*' | jq '.Images[].ImageId'` (В каждом регионе AMI разные).
* `spot` — создавать ли spot instance'ы. Spot Instances будут запускаться с минимально возможной для успешного запуска ценой за час.
    * Формат — bool.
* `diskType` — тип созданного диска.
    * По умолчанию `gp2`.
    * Опциональный параметр.
* `iops` — количество iops. Применяется только для `diskType` **io1**.
    * Формат — integer.
    * Опциональный параметр.
* `diskSizeGb` — размер root диска.
    * Формат — integer. В ГиБ.
    * По умолчанию `20` ГиБ.
    * Опциональный параметр.
* `additionalSecurityGroups` — дополнительный список security groups, которые будут добавлены на заказанные instances с данными InstanceClass.
    * Формат — массив строк.
    * Опциональный параметр.
* `additionalTags` — дополнительные теги, которые будут присвоены созданным инстансам в AWS.
    * Формат — словарь.
    * Опциональный параметр.

### Пример

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AWSInstanceClass
metadata:
  name: worker
spec:
  instanceType: t3.large
  ami: ami-040a1551f9c9d11ad
  diskSizeGb: 15
  diskType:  gp2
```
