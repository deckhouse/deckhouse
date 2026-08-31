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

1. `service.beta.kubernetes.io/aws-load-balancer-type` — может иметь значение `none`, что приведет к созданию **только** целевой группы, без какого-либо LoadBalanacer'а.
2. `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — используется в связке с `service.beta.kubernetes.io/aws-load-balancer-type: none`:
   * Возможные значения:
     * `tcp` (по умолчанию);
     * `tls`;
     * `http`;
     * `https`.
   * **Внимание!** При изменении этого параметра `cloud-controller-manager` попытается пересоздать целевую группу. Если к ней уже привязаны NLB или ALB, удалить целевую группу не получится и он будет бесконечно пытаться это сделать. В таком случае необходимо вручную отсоединить NLB или ALB от целевой группы.

## Настройка балансировщика в случае наличия Ingress-узлов не во всех зонах

Необходимо указать аннотацию на объекте Service: `service.beta.kubernetes.io/aws-load-balancer-subnets: subnet-foo, subnet-bar`.

Чтобы получить список текущих подсетей, используемых для конкретной установки, выполните следующую команду:

```bash
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values cloud-provider-aws -o json \
| jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'
```
