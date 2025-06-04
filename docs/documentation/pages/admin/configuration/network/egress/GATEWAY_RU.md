---
title: "Настройка Egress Gateway"
permalink: ru/admin/configuration/network/egress/gateway.html
lang: ru
---

Egress Gateway позволяет централизованно управлять исходящим трафиком и контролировать его, а также обеспечивает такие функции, как шифрование, маршрутизация и мониторинг.

{% alert level="warning"%}
Чтобы использовать Egress Gateway, включите в кластере модуль [cni-cilium](../../reference/mc/cni-cilium/)
{% endalert %}

<!-- Перенесено с небольшими доработками из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/cni-cilium/#%D0%B8%D1%81%D0%BF%D0%BE%D0%BB%D1%8C%D0%B7%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D0%B5-egress-gateway -->

## Режимы использования Egress Gateway

{% alert level="warning" %}
Функция доступна только в Enterprise Edition
{% endalert %}

### Базовый режим

Используются предварительно настроенные IP-адреса на egress-узлах.

<div data-presentation="../../../presentations/cni-cilium/egressgateway_base_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/12l4w9ZS3Hpax1B7eOptm2dQX55VVAFzRTtyihw4Ie0c/ --->

### Режим с Virtual IP

Реализована возможность динамически назначать дополнительные IP-адреса узлам.

<div data-presentation="../../../presentations/cni-cilium/egressgateway_virtualip_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1tmhbydjpCwhNVist9RT6jzO1CMpc-G1I7rczmdLzV8E/ --->

<!-- Перенесено с небольшими доработками из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/cni-cilium/examples.html#egress-gateway -->

## Принцип настройки

Для настройки egress-шлюза добавьте два кастомных ресурса:

1. `EgressGateway` — описывает группу узлов, один из которых будет выбран в качестве активного egress-шлюза, а остальные останутся в резерве на случай отказа:
   - Среди группы узлов, попадающих под `spec.nodeSelector`, будут отобраны пригодные к использованию. Один из них будет назначен активным шлюзом. Выбор активного узла осуществляется [в алфавитном порядке](https://docs.cilium.io/en/latest/network/egress-gateway/egress-gateway/index.html#selecting-and-configuring-the-gateway-node).

     Признаки пригодного узла:
     - Узел в состоянии `Ready`.
       - Узел не находится в состоянии технического обслуживания (cordon).
       - `cilium-agent` на узле в состоянии `Ready`.
     - При использовании EgressGateway в режиме `VirtualIP` на активном узле запускается агент, который эмулирует «виртуальный» IP-адрес с использованием протокола ARP. При определении пригодности узла также учитывается состояние пода данного агента.
     - Разные EgressGateway могут использовать одни и те же узлы. Выбор активного узла в каждом EgressGateway осуществляется независимо от других, что позволяет сбалансировать нагрузку между ними.
1. `EgressGatewayPolicy` — описывает политику перенаправления сетевых запросов от подов в кластере на конкретный egress-шлюз, определённый с помощью EgressGateway.

## Обслуживание узла

Для проведения работ на узле, который в данный момент является активным egress-шлюзом, выполните следующие шаги:

1. Снимите метку (label) с узла, чтобы исключить его из списка кандидатов для роли egress-шлюза. Egress-label — это метка, указанная в `spec.nodeSelector` вашего EgressGateway.

    ```bash
    kubectl label node <node-name> <egress-label>-
    ```

1. Переведите узел в режим обслуживания (cordon), чтобы предотвратить запуск новых подов:

    ```bash
    kubectl cordon <node-name>
    ```

    После этого Cilium автоматически выберет новый активный узел из оставшихся кандидатов.
    Трафик продолжит направляться через новый шлюз без прерывания.

1. Для возврата узла в работу выполните:

    ```bash
    kubectl uncordon <node-name>
    kubectl label node <node-name> <egress-label>=<value>
    ```

> Важно: повторное добавление метки может привести к тому, что узел снова будет выбран активным egress-шлюзом (если он первый в алфавитном порядке среди доступных кандидатов).
Чтобы избежать немедленного возврата узла в активное состояние, временно уменьшите количество реплик в EgressGateway или настройте приоритет выбора через дополнительные метки.

## Сравнение с CiliumEgressGatewayPolicy

`CiliumEgressGatewayPolicy` подразумевает настройку только одного узла в качестве egress-шлюза. При выходе его из строя не предусмотрено failover-механизмов и сетевая связь будет нарушена.

## Пример настройки

### EgressGateway в режиме PrimaryIPFromEgressGatewayNodeInterface (базовый режим)

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
      # На всех узлах, попадающих под nodeSelector, «публичный» интерфейс должен называться одинаково.
      # При выходе из строя активного узла, трафик будет перенаправлен через резервный и
      # IP-адрес отправителя у сетевых пакетов поменяется.
      interfaceName: eth1
```

### EgressGateway в режиме VirtualIPAddress (Режим с Virtual IP)

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
      # На каждом узле должны быть настроены все необходимые маршруты для доступа на все внешние публичные сервисы,
      # «публичный» интерфейс должен быть подготовлен к автоматической настройке «виртуального» IP в качестве дополнительного (secondary) IP-адреса.
      # При выходе из строя активного узла, трафик будет перенаправлен через резервный и
      # IP-адрес отправителя у сетевых пакетов не поменяется.
      ip: 172.18.18.242
      # Список сетевых интерфейсов для «виртуального» IP.
      interfaces:
      - eth1
```

### EgressGatewayPolicy

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGatewayPolicy
metadata:
  name: my-egressgw-policy
spec:
  destinationCIDRs:
  - 0.0.0.0/0
  egressGatewayName: my-egressgw
  selectors:
  - podSelector:
      matchLabels:
        app: backend
        io.kubernetes.pod.namespace: my-ns
```
