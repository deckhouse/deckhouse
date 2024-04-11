---
title: "Модуль cni-cilium: примеры"
---

## Egress Gateway

{% alert level="warning" %} Функция доступна только в Enterprise Edition {% endalert %}

### Принцип работы

Для настройки egress-шлюза необходимо настроить два ресурса:

* `EgressGateway` — описывает группу узлов, которые осуществляют функцию egress-шлюза в режиме горячего резерва:
  * Среди группы узлов, попадающих под `spec.nodeSelector`, будут выявлены пригодные к работе и один из них будет назначен активным. Признаки пригодного узла:
    * Узел в состоянии Ready.
    * Узел не находится в состоянии технического обслуживания (cordon).
    * cilium-agent на узле в состоянии Ready.
  * При использовании `EgressGateway` в режиме `VirtualIP` на активном узле запускается агент, который эмулирует "виртуальный" IP средствами протокола ARP. При определении пригодности узла также учитывается состояние пода данного агента.
  * Разные EgressGateway могут использовать для работы общие узлы, при этом активные узлы будут выбираться независимо, тем самым распределяя нагрузку между ними.
* `EgressGatewayPolicy` — описывает политику перенаправления сетевых запросов от подов в кластере на определённый egress-шлюз, описанный с помощью `EgressGateway`.

### Сравнение с CiliumEgressGatewayPolicy

`CiliumEgressGatewayPolicy` подразумевает настройку лишь одного узла в качестве egress-шлюза. При выходе его из строя не предусмотрено failover-механизмов и сетевая связь будет нарушена.

### Пример настройки

#### EgressGateway в режиме PrimaryIPFromEgressGatewayNodeInterface

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: myegressgw
spec:
  nodeSelector:
    node-role.deckhouse.io/egress: ""
  sourceIP:
    mode: PrimaryIPFromEgressGatewayNodeInterface
    primaryIPFromEgressGatewayNodeInterface:
      interfaceName: eth1 # На всех узлах, попадающих под nodeSelector, "публичный" интерфейс должен иметь одинаковое имя.
                          # При выходе из строя активного узла, трафик будет перенаправлен через резервный и
                          # IP-адрес отправителя у сетевых пакетов поменяется.
```

#### EgressGateway в режиме VirtualIPAddress

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: myeg
spec:
  nodeSelector:
    node-role.deckhouse.io/egress: ""
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      ip: 172.18.18.242 # На каждом узле должны быть настроены все необходимые маршруты для доступа на все внешние публичные сервисы,
                        # "публичный" интерфейс должен быть подготовлен к автоматической настройке "виртуального" IP в качестве secondary IP-адреса.
                        # При выходе из строя активного узла, трафик будет перенаправлен через резервный и
                        # IP-адрес отправителя у сетевых пакетов не поменяется.
```

#### EgressGatewayPolicy

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGatewayPolicy
metadata:
  name: 
spec:
  destinationCIDRs:
  - 0.0.0.0/0
  egressGatewayName: myegressgw
  selectors:
  - podSelector:
      matchLabels:
        app: backend
        io.kubernetes.pod.namespace: myns
```
