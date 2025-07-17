---
title: "Управление исходящим трафиком"
permalink: ru/stronghold/documentation/admin/platform-management/network/egress.html
lang: ru
---

{% alert level="warning" %}
Функция недоступна в Community Edition.
{% endalert %}

В развернутом кластере некоторые узлы могут быть изолированы от внешней сети в целях обеспечения безопасности.
В этом случае выход в интернет осуществляется через заранее выделенные узлы, обладающие доступом к внешним ресурсам.

Перенаправление трафика в таком случае происходит через преднастроенные Egress-шлюзы (EgressGateway), согласно описанной политике (EgressGatewayPolicy).

## Группы узлов Egress-шлюза

Ресурс EgressGateway описывает группу узлов, которые осуществляют функцию Egress-шлюза.
Для включения узла в эту группу нужно назначить ему соответствующую метку:

```shell
d8 k label node <имя узла> dedicated/egress=
```

### Режим PrimaryIPFromEgressGatewayNodeInterface

В качестве IP-адреса будет использоваться основной (primary) IP-адрес, привязанный к публичному сетевому интерфейсу узла.
В случае сбоя активного узла и назначения нового, IP-адрес отправителя в сетевых пакетах изменится.

Чтобы создать Egress-шлюз, примените ресурс `EgressGateway`:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egress-gw
spec:
  nodeSelector:
    dedicated/egress: ""
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

Альтернативным решением является назначение виртуального IP-адреса для группы узлов.

Этот виртуальный адрес будет привязан к главному узлу, обеспечивая маршрутизацию трафика к внешним сервисам.
В случае сбоя главного узла все текущие соединения будут разорваны, после чего выберется новый главный узел, которому будет назначен тот же виртуальный адрес. При этом для внешних сервисов IP-адрес клиента останется неизменным.

Чтобы создать Egress-шлюз с виртуальным адресом, примените ресурс `EgressGateway`:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egress-gw
spec:
  nodeSelector:
    dedicated/egress: ""
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

Среди узлов, соответствующих `spec.nodeSelector` ресурса EgressGateway, будут отобраны те, которые пригодны для работы.
Узел считается пригодным, если выполняются следующие условия:

1. Узел находится в состоянии Ready;
1. Узел не находится в состоянии технического обслуживания (cordon);
1. cilium-agent на узле находится в состоянии Ready.

Чтобы проверить пригодность узлов для включения в группу Egress-шлюза, выполните команды:

```shell
# Вывести узлы, подпадающие под spec.nodeSelector:
d8 k get nodes -l dedicated/egress="" -ojson | jq -r '.items[].metadata.name'

# Вывести узлы в состоянии Ready:
d8 k get nodes -ojson | jq -r '.items[] | select(.status.conditions[] | select(.type == "Ready" and .status == "True")) | .metadata.name'

# Вывести узлы, не находящиеся на техническом обслуживании:
d8 k get nodes --field-selector spec.unschedulable=false -ojson | jq -r .items[].metadata.name

# Вывести узлы, на которых запущен cilium-agent:
d8 k get pods -n d8-cni-cilium -l app=agent -ojson | jq -r '.items[].spec.nodeName'
```

В группе узлов один из них будет назначен главным, и через него будет осуществляться передача трафика наружу.
Остальные узлы будут находиться в режиме ожидания (hot-standby).
В случае сбоя активного узла текущие соединения будут разорваны, и среди оставшихся узлов будет выбран новый главный, через который передача трафика продолжится.

Разные EgressGateway могут использовать для работы общие узлы, при этом главные узлы будут выбираться независимо, тем самым распределяя нагрузку между ними.

## Политика перенаправления трафика

Ресурс EgressGatewayPolicy описывает политику перенаправления прикладного трафика с виртуальных машин на указанный Egress-шлюз.
Сопоставление политики и виртуальных машин происходит по меткам.

Чтобы добавить виртуальную машину в политику, необходимо назначить для нее метку, например:

```shell
d8 k label vm <имя виртуальной машины> app=backend
```

Чтобы настроить политику перенаправления трафика, создайте ресурс `EgressGatewayPolicy`, указав в поле `.spec.selectors` критерии для выбора виртуальных машин, к которым будет применяться эта политика.
Пример:

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
          # Данная политика будет применена для всех подов с меткой app=backend в пространстве имён default на всех виртуальных машинах.
          app: backend
          io.kubernetes.pod.namespace: default
EOF
```

Чтобы проверить, что политика EgressGatewayPolicy успешно применяется к виртуальной машине, выполните на ней команду для диагностики сетевого подключения и маршрутизации:

```shell
curl ip.flant.ru
```

В результате проверки будет отображен либо IP-адрес главного узла (если Egress-шлюз в режиме `PrimaryIPFromEgressGatewayNodeInterface`),
либо виртуальный IP-адрес (для режима `VirtualIPAddress`).
