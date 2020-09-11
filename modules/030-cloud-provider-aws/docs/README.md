---
title: "Модуль cloud-provider-aws"
---

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами облака из Kubernetes.
    * Создаёт route'ы для PodNetwork в cloud provider'е.
    * Создаёт LoadBalancer'ы для Service-объектов Kubernetes с типом LoadBalancer.
    * Синхронизирует метаданные AWS Instances и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в AWS.
2. CSI storage — для заказа дисков в AWS.
3. Включение необходимого CNI ([simple bridge]({{ site.baseurl }}/modules/035-cni-simple-bridge/)).
4. Регистрация в модуле [node-manager]({{ site.baseurl }}/modules/040-node-manager/), чтобы [AWSInstanceClass'ы](#awsinstanceclass-custom-resource) можно было использовать в [NodeGroups]({{ site.baseurl }}/modules/040-node-manager/#nodegroup-custom-resource).

## Конфигурация

### Параметры

Модуль настраивается автоматически на основании [выбранной схемы размещения](/candi/cloud-providers/aws/README.md). Предусмотрены только параметры в отдельных [AWSInstanceClass](AWSInstanceClass custom resource).

### AWSInstanceClass custom resource

Ресурс описывает параметры группы AWS Instances, которые будет использовать machine-controller-manager из модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `instanceType` — тип заказываемых instances. **Внимание!** Следует убедиться, что указанный тип есть во всех зонах, указанных в `zones`.
* `ami` — образ, который поставится в заказанные instance'ы.
    * Формат — строка, AMI ID.
    * Как найти нужный AMI: `aws ec2 --region <REGION> describe-images --filters 'Name=name,Values=buntu/images/hvm-ssd/ubuntu-bionic-18.04-amd64-server-2020*' | jq '.Images[].ImageId'` (В каждом регионе AMI разные).
* `spot` — создавать ли spot instance'ы. Spot Instances будут запускаться с минимально возможной для успешного запуска ценой за час.
    * Формат — bool.
* `diskType` — тип созданного диска.
    * По-умолчанию `gp2`.
    * Опциональный параметр.
* `iops` — количество iops. Применяется только для `diskType` **io1**.
    * Формат — integer.
    * Опциональный параметр.
* `diskSizeGb` — размер root диска.
    * Формат — integer. В ГиБ.
    * По-умолчанию `20` ГиБ.
    * Опциональный параметр.
* `additionalSecurityGroups` — дополнительный список security groups, которые будут добавлены на заказанные instances с данными InstanceClass.
    * Формат — массив строк.
    * Опциональный параметр.

#### Пример AWSInstanceClass

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

### Storage

По умолчанию создаётся один (default) StorageClass с именем `gp2` и типом диска `gp2`.

Создать StorageClass с типом диска `io1` можно, например, так:

```yaml
---
apiVersion: storage.k8s.io/v1beta1
kind: StorageClass
metadata:
  name: io1
provisioner: ebs.csi.aws.com
parameters:
  type: io1
  iopsPerGB: "20"
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer # обязательно!
```

Список всех возможных `parameters` для EBS CSI драйвера представлен в его [документации](https://github.com/kubernetes-sigs/aws-ebs-csi-driver).

### LoadBalancer

#### Аннотации объекта Service

Поддерживаются следующие параметры в дополнение к существующим в upstream:

1. `service.beta.kubernetes.io/aws-load-balancer-type` — может иметь значение `none`, что приведёт к созданию **только** Target Group, без какого либо LoadBalanacer.
2. `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — используется в связке с `service.beta.kubernetes.io/aws-load-balancer-type: none`.
   * Возможные значения:
     * `tcp`
     * `tls`
     * `http`
     * `https`
   * По-умолчанию, `tcp`.
   * **Внимание!** При изменении поля cloud-controller-manager попытается пересоздать Target Group. Если к ней уже привязаны NLB или ALB, удалить Target Group он не сможет и будет пытаться вечно. Необходимо вручную отсоединить от Target Group NLB или ALB.
