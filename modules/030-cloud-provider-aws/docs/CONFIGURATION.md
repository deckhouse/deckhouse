---
title: "Сloud provider — AWS: конфигурация"
---

## Параметры

Модуль настраивается автоматически исходя из выбранной схемы размещения (custom resource `AWSClusterConfiguration`). В большинстве случаев нет необходимости ручной конфигурации модуля.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](/modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера AWS — это custom resource [`AWSInstanceClass`](cr.html#awsinstanceclass), в котором указываются конкретные параметры самих машин.

## Storage

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

## LoadBalancer

### Аннотации объекта Service

Поддерживаются следующие параметры в дополнение к существующим в upstream:

1. `service.beta.kubernetes.io/aws-load-balancer-type` — может иметь значение `none`, что приведёт к созданию **только** Target Group, без какого либо LoadBalanacer.
2. `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — используется в связке с `service.beta.kubernetes.io/aws-load-balancer-type: none`.
   * Возможные значения:
     * `tcp`
     * `tls`
     * `http`
     * `https`
   * По умолчанию, `tcp`.
   * **Внимание!** При изменении поля cloud-controller-manager попытается пересоздать Target Group. Если к ней уже привязаны NLB или ALB, удалить Target Group он не сможет и будет пытаться вечно. Необходимо вручную отсоединить от Target Group NLB или ALB.

## Настройка политик безопасности на узлах
Вариантов, зачем может понадобиться ограничить или наоборот расширить входящий или исходящий трафик на виртуальных
машинах кластера  в AWS, может быть множество, например:
* Разрешить подключение к нодам кластера с виртуальных машин из другой подсети
* Разрешить подключение к портам статической ноды для работы приложения
* Ограничить доступ к внешним ресурсам или другим вм в облаке по требованию службы безопасности

Для всего этого следует применять дополнительные security groups. Можно использовать только security groups, предварительно созданные в облаке.

## Установка дополнительных security groups на статических и мастер-узлах
Данный параметр можно задать либо при создании кластера, либо в уже существующем кластере. В обоих случаях дополнительные security groups указываются в `AWSClusterConfiguration`:
- для мастер-узлов, в секции `masterNodeGroup` в поле `additionalSecurityGroups`
- для статических узлов — в секции `nodeGroups` в конфигурации, описывающей соответствующую nodeGroup, в поле `additionalSecurityGroups`.

Поле `additionalSecurityGroups` — содержит массив строк с именами security groups.

## Установка дополнительных security groups на эфемерных нодах
Необходимо указать параметр `additionalSecurityGroups` для всех [`AWSInstanceClass`](cr.html#awsinstanceclass) в кластере, которым нужны дополнительные security groups.
