---
title: "Cloud provider — AWS: примеры"
---

## Пример custom resource `AWSInstanceClass`

Ниже представлен простой пример конфигурации custom resource `AWSInstanceClass`:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: worker
spec:
  instanceType: t3.large
  ami: ami-040a1551f9c9d11ad
  diskSizeGb: 15
  diskType:  gp2
```

## LoadBalancer

### Аннотации объекта Service

Поддерживаются следующие параметры в дополнение к существующим в [upstream](https://cloud-provider-aws.sigs.k8s.io/service_controller/):

1. `service.beta.kubernetes.io/aws-load-balancer-type` — может иметь значение `none`, что приведет к созданию **только** Target Group, без какого-либо LoadBalanacer'а.
2. `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — используется в связке с `service.beta.kubernetes.io/aws-load-balancer-type: none`:
   * Возможные значения:
     * `tcp` (по умолчанию);
     * `tls`;
     * `http`;
     * `https`.
   * **Внимание!** При изменении этого параметра `cloud-controller-manager` попытается пересоздать Target Group. Если к ней уже привязаны NLB или ALB, удалить Target Group не получится и он будет бесконечно пытаться это сделать. В таком случае необходимо вручную отсоединить NLB или ALB от Target Group.

## Настройка политик безопасности на узлах

Вариантов, зачем может понадобиться ограничить или, наоборот, расширить входящий или исходящий трафик на виртуальных машинах кластера в AWS, может быть множество. Например:

* Разрешить подключение к узлам кластера с виртуальных машин из другой подсети.
* Разрешить подключение к портам статического узла для работы приложения.
* Ограничить доступ к внешним ресурсам или другим виртуальным машинам в облаке по требованию службы безопасности.

Для всех них следует применять дополнительные группы безопасности (security group). Можно использовать только предварительно созданные в облаке группы безопасности.

## Установка дополнительных security groups на статических и master-узлах

Данный параметр можно задать либо при создании кластера, либо в уже существующем кластере. В обоих случаях дополнительные группы безопасности (security group) указываются в `AWSClusterConfiguration`:
- для master-узлов — в секции `masterNodeGroup` в поле `additionalSecurityGroups`;
- для статических узлов — в секции `nodeGroups` в конфигурации, описывающей соответствующую nodeGroup, в поле `additionalSecurityGroups`.

Поле `additionalSecurityGroups` содержит массив строк с именами групп безопасности.

## Установка дополнительных security groups на эфемерных узлах

Необходимо указать параметр `additionalSecurityGroups` для всех [`AWSInstanceClass`](cr.html#awsinstanceclass) в кластере, которым нужны дополнительные группы безопасности (security group).

## Настройка балансировщика в случае наличия Ingress-узлов не во всех зонах

Необходимо указать аннотацию на объекте Service: `service.beta.kubernetes.io/aws-load-balancer-subnets: subnet-foo, subnet-bar`.

Чтобы получить список текущих подсетей, используемых для конкретной установки, выполните следующую команду:

```bash
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller module values cloud-provider-aws -o json \
| jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'
```
