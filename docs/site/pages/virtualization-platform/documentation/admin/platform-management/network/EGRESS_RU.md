---
title: "Управление исходящим трафиком"
permalink: ru/virtualization-platform/documentation/admin/platform-management/network/egress.html
lang: ru
---

{% alert level="warning" %} Функция недоступна в Community Edition {% endalert %}

В развернутом кластере ряд узлов могут не иметь доступ к внешнему миру для обеспечения требований безопасности.
В таком случае доступ в интернет осуществляет через предварительно выделенные узлы, имеющие доступ во внешний мир.

Перенаправление трафика в таком случае происходит через преднастроенные Egress-шлюзы (`EgressGateway`)
согласно описанной политике (`EgressGatewayPolicy`).

## Группы узлов Egress-шлюза

Ресурс `EgressGateway` описывает группу узлов, которые осуществляют функцию Egress-шлюза.
Чтобы добавить узел в группу, необходимо поставить на него метку:

```shell
d8 k label node <имя узла> node-role.deckhouse.io/egress=
```

### Режиме PrimaryIPFromEgressGatewayNodeInterface

В качестве IP-адреса будет использоваться primary IP-адрес на публичном сетевом интерфейсе узла.
При выходе из строя активного узла и назначении нового, IP-адрес отправителя в сетевых пакетах поменяется.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_base_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/12l4w9ZS3Hpax1B7eOptm2dQX55VVAFzRTtyihw4Ie0c/ --->

Чтобы создать Egress-шлюз примените следующий ресурс `EgressGateway`:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egress-gw
spec:
  nodeSelector:
    node-role.deckhouse.io/egress: ""
  sourceIP:
    # В качестве IP-адреса будет использоваться primary IP-адрес на публичном сетевом интерфейсе узла.
    mode: PrimaryIPFromEgressGatewayNodeInterface
    primaryIPFromEgressGatewayNodeInterface:
      # Предварительно необходимо настроить сетевую подсистему на всех Egress-узлах,
      # так как на всех узлах группы "публичный" интерфейс должен иметь одинаковое имя (например, eth1).
      interfaceName: eth1
EOF
```

### Режим VirtualIPAddress

Альтернативно можно назначить виртуальный IP-адрес для группы узлов.

Главному узлу будет назначен указанный виртуальный адрес, через который трафик будет маршрутизироваться к внешним сервисам.
Если главный узел выйдет из строя, то все текущие соединения оборвуться и будет выбран новый главный узел,
которому назначится тот же виртуальный адрес. С точки зрения внешних сервисов, адрес клиента не изменится.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_virtualip_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1tmhbydjpCwhNVist9RT6jzO1CMpc-G1I7rczmdLzV8E/ --->

Чтобы создать Egress-шлюз с виртуальным адресом, примените следующий ресурс `EgressGateway`:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egress-gw
spec:
  nodeSelector:
    node-role.deckhouse.io/egress: ""
  sourceIP:
    # В качестве IP-адреса будет использоваться primary IP-адрес на публичном сетевом интерфейсе узла.
    mode: VirtualIPAddress
    virtualIPAddress:
      # На каждом узле должны быть настроены все необходимые маршруты для доступа на все внешние публичные сервисы,
      # "публичный" интерфейс должен быть подготовлен к автоматической настройке "виртуального" IP в качестве secondary IP-адреса.
      ip: 172.18.18.242
EOF
```

### Пригодность узлов Egress-шлюза

Среди группы узлов, попадающих под указанный в ресурсе `EgressGateway` spec.nodeSelector, будут выявлены пригодные к работе.
Признаки пригодного узла:

1. Узел в состоянии Ready;
2. Узел не находится в состоянии технического обслуживания (cordon);
3. cilium-agent на узле в состоянии Ready.

Чтобы проверить пригодность узлов для включение в группу Egress-шлюза, выполните команды:

```shell
# Вывести узлы, попадающие под spec.nodeSelector:
d8 k get nodes -l node-role.deckhouse.io/egress="" -ojson | jq -r '.items[].metadata.name'

# Вывести узлы в состоянии Ready:
d8 k get nodes -ojson | jq -r '.items[] | select(.status.conditions[] | select(.type == "Ready" and .status == "True")) | .metadata.name'

# Вывести узлы, не находящиеся на техническом обслуживании:
d8 k get nodes --field-selector spec.unschedulable=false -ojson | jq -r .items[].metadata.name

# Вывести узлы, на которых запущен cilium-agent:
d8 k get pods -n d8-cni-cilium -l app=agent -ojson | jq -r '.items[].spec.nodeName'
```

Один из узлов группы будет назначен главным и именно через него начнется передача трафика наружу.
Остальные узлы будут находится в режиме ожидания (hot-standby). Если активный узел выйдет из строя, то все текущие
соединения оборувться и среди оставшихся будет выбран новый главный узел, через который возобновится передача трафика.

Разные `EgressGateway` могут использовать для работы общие узлы, при этом главные узлы будут выбираться независимо,
тем самым распределяя нагрузку между ними.

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
          # Данная политика будет применена для все виртуальных машин с меткой app=backend в пространстве default.
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
