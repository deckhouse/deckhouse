---
title: "Управление исходящим трафиком"
permalink: ru/virtualization-platform/documentation/admin/traffic/egress-gateway-policy.html
lang: ru
---

{% alert level="warning" %} Функция доступна только в Enterprise Edition {% endalert %}

## Мотивация

В развернутом кластере ряд узлов могут не иметь доступ к внешнему миру для обеспечения требований безопасности.
В таком случае доступ в интернет осуществляет через предварительно выделенные узлы, имеющие доступ во внешний мир.

Перенаправление трафика в таком случае происходит через преднастроенные Egress-шлюзы (`EgressGateway`)
согласно описанной политике (`EgressGatewayPolicy`).

## Доступные Egress-шлюзы

Преднастроенные Egress-шлюзы (`EgressGateway`) можно получить с помощью команды:

```shell
d8 k get EgressGateway

# NAME        READY   MODE
# egress-gw   True    PrimaryIPFromEgressGatewayNodeInterface
```

Информация по преднастройке и описанию группы узлов Egress-шлюза доступна в документации для администратора: [управление трафиком](./ADMIN_TRAFFIC.md/)

## Политика перенаправления трафика

Ресурс `EgressGatewayPolicy` описывает политику перенаправления прикладного трафика с виртуальных машин на указанный Egress-шлюз.
Сопоставление политики и виртуальных машин происходит по меткам.

Чтобы добавить виртуальную машину в политику, необходимо поставить на нее метку, например:

```shell
d8 k label vm <имя виртуальной машины> app=backend
```

Чтобы создать политику перенаправления трафика примените следующий ресурс `EgressGatewayPolicy`,
указав в .spec.selectors для каких виртуальных машин эта политика будет применяться:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGatewayPolicy
metadata:
  name: egress-gw-policy
spec:
  destinationCIDRs:
    - 0.0.0.0/0
  egressGatewayName: egress-gw
  selectors:
    - podSelector:
        matchLabels:
          # Данная политика будет применена для все виртуальных машин с меткой app=backend в пространстве default   
          app: backend
          io.kubernetes.pod.namespace: default
EOF
```

Чтобы убедиться в применении политики, можно выполнить следующую команду на виртуальной машине:

```shell
curl ip.flant.ru
```

Будет отображен либо IP адрес главного узла (если Egress-шлюз в режиме `PrimaryIPFromEgressGatewayNodeInterface`),
либо виртуальный IP адрес (для режима `VirtualIPAddress`) 
