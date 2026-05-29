---
title: "Модуль cni-cilium: примеры"
description: Примеры настройки Egress Gateway, экспорта данных Hubble и точечного включения BPF-трейсов для модуля cni-cilium.
---

## Egress Gateway

{% alert level="warning" %}
Доступно в следующих редакциях: SE+, EE, CSE Lite (1.73), CSE Pro (1.73).
{% endalert %}

### Принцип работы

Для настройки egress-шлюза необходимы два кастомных ресурса:

1. EgressGateway — описывает группу узлов, один из которых будет выбран в качестве активного egress-шлюза, а остальные останутся в резерве на случай отказа:
   - Среди группы узлов, попадающих под `spec.nodeSelector`, будут отобраны пригодные к использованию. Один из них будет назначен активным шлюзом. Выбор активного узла осуществляется [в алфавитном порядке](https://docs.cilium.io/en/latest/network/egress-gateway/egress-gateway/index.html#selecting-and-configuring-the-gateway-node).

     Признаки пригодного узла:
     - Узел в состоянии `Ready`.
       - Узел не находится в состоянии технического обслуживания (cordon).
       - `cilium-agent` на узле в состоянии `Ready`.
     - При использовании EgressGateway в режиме `VirtualIP` на активном узле запускается агент, который эмулирует «виртуальный» IP-адрес с использованием протокола ARP. При определении пригодности узла также учитывается состояние пода данного агента.
     - Разные EgressGateway могут использовать одни и те же узлы. Выбор активного узла в каждом EgressGateway осуществляется независимо от других, что позволяет сбалансировать нагрузку между ними.
1. EgressGatewayPolicy — описывает политику перенаправления сетевых запросов от подов в кластере на конкретный egress-шлюз, определённый с помощью EgressGateway.

### Обслуживание узла

Для проведения работ на узле, который в данный момент является активным egress-шлюзом, выполните следующие шаги:

1. Снимите лейбл с узла, чтобы исключить его из списка кандидатов для роли egress-шлюза. Egress-label — это лейбл, указанный в `spec.nodeSelector` вашего EgressGateway.

    ```bash
    d8 k label node <node-name> <egress-label>-
    ```

1. Переведите узел в режим обслуживания (cordon), чтобы предотвратить запуск новых подов:

    ```bash
    d8 k cordon <node-name>
    ```

    После этого Cilium автоматически выберет новый активный узел из оставшихся кандидатов.
    Трафик продолжит направляться через новый шлюз без прерывания.

1. Для возврата узла в работу выполните:

    ```bash
    d8 k uncordon <node-name>
    d8 k label node <node-name> <egress-label>=<value>
    ```

{% alert level="info" %}
Повторное добавление лейбла может привести к тому, что узел снова будет выбран активным egress-шлюзом (если он первый в алфавитном порядке среди доступных кандидатов).
{% endalert %}

Чтобы избежать немедленного возврата узла в активное состояние, временно уменьшите количество реплик в EgressGateway или настройте приоритет выбора через дополнительные лейблы.

### Сравнение с CiliumEgressGatewayPolicy

CiliumEgressGatewayPolicy подразумевает настройку только одного узла в качестве egress-шлюза. При выходе его из строя не предусмотрено failover-механизмов и сетевая связь будет нарушена.

### Примеры настроек Egress Gateway

#### EgressGateway в режиме PrimaryIPFromEgressGatewayNodeInterface (базовый режим)

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: myegressgw
spec:
  nodeSelector:
    dedicated/egress: ""
  sourceIP:
    mode: PrimaryIPFromEgressGatewayNodeInterface
    primaryIPFromEgressGatewayNodeInterface:
      # На всех узлах, попадающих под nodeSelector, «публичный» интерфейс должен называться одинаково.
      # При выходе из строя активного узла, трафик будет перенаправлен через резервный и
      # IP-адрес отправителя у сетевых пакетов поменяется.
      interfaceName: eth1
```

#### EgressGateway в режиме VirtualIPAddress (режим с Virtual IP)

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: myeg
spec:
  nodeSelector:
    dedicated/egress: ""
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

#### EgressGatewayPolicy

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

## HubbleMonitoringConfig

Кластерный ресурс [HubbleMonitoringConfig](cr.html#hubblemonitoringconfig) предназначен для настройки экспорта данных из Hubble, работающего внутри агентов Cilium.

### Примеры настроек HubbleMonitoringConfig

#### Включение расширенных метрик и экспорта flow logs (с фильтрами и маской полей)

{% alert level="warning" %}
Ресурс [HubbleMonitoringConfig](cr.html#hubblemonitoringconfig) должен иметь имя `hubble-monitoring-config`.
{% endalert %}

Пример включения метрик и экспорта:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: HubbleMonitoringConfig
metadata:
  name: hubble-monitoring-config
spec:
  extendedMetrics:
    enabled: true
    collectors:
      - name: drop
        # Добавить дополнительный контекст (лейблы) для выбранного коллектора.
        contextOptions: "labelsContext=source_ip,source_namespace,source_pod,destination_ip,destination_namespace,destination_pod"
      - name: flow
  flowLogs:
    enabled: true
    # Записывать в лог-файл /var/log/cilium/hubble/flow.log только указанные события.
    allowFilterList:
      - verdict:
        - DROPPED
        - ERROR
    # Исключить из лог-файла события, соответствующие denyFilterList.
    denyFilterList:
      - source_pod:
        - kube-system/
      - destination_pod:
        - kube-system/
    # Сохранять в каждой записи только указанные поля.
    fieldMaskList:
      - time
      - verdict
    # Максимальный размер лог-файла (в МБ) перед ротацией.
    fileMaxSizeMB: 30
```

### Сбор Hubble flow logs с помощью модуля log-shipper

Для сбора flow logs используйте модуль [`log-shipper`](/modules/log-shipper/).

Создайте ресурс ClusterLoggingConfig, который читает лог-файл с файловой системы узла:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingConfig
metadata:
  name: cilium-hubble-flow-logs
spec:
  type: File
  file:
    include:
      - /var/log/cilium/hubble/flow.log
```

## Включение BPF-трейсов по требованию

Отслеживание событий BPF (`bpf-events-trace-enabled`) отключено во всём кластере по
умолчанию. На нагруженных узлах с высокой плотностью подов они являются основным
источником потребления CPU и памяти `cilium-agent`: каждый пересланный пакет
(с учётом `monitor-aggregation`) генерирует запись в буфер событий BPF, которую агент должен распарсить, добавить лейблы и передать в Hubble. События `drop` и
`policy verdict` не зависят от этой опции и остаются доступными в Hubble.

Если требуется увидеть forwarded-flow (например, для диагностики проблемы со
связностью) — включите трейсы через ресурс
[CiliumNodeConfig](https://docs.cilium.io/en/stable/configuration/per-node-config/).
Один и тот же ресурс может покрывать один узел, группу узлов или весь кластер
— в зависимости от `spec.nodeSelector`.

### Включение трейсов на одном узле

Для включения трейсов на определенном узле укажите его имя в параметре `spec.nodeSelector.matchLabels`:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNodeConfig
metadata:
  name: trace-debug-node-1
  namespace: d8-cni-cilium
spec:
  nodeSelector:
    matchLabels:
      kubernetes.io/hostname: <node-name>
  defaults:
    bpf-events-trace-enabled: "true"
    # Уровень агрегации "none" отключает логику агрегации событий пакетов одной сессии:
    # для каждого пакета создаётся отдельное событие трассировки.
    # Если строку не добавлять, на узле останется кластерная настройка "medium".
    monitor-aggregation: "none"
```

### Включение трейсов на всех узлах кластера

Пустой `matchLabels` выбирает все узлы:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNodeConfig
metadata:
  name: trace-debug-all-nodes
  namespace: d8-cni-cilium
spec:
  nodeSelector:
    matchLabels: {}
  defaults:
    bpf-events-trace-enabled: "true"
```

### Применение изменений

Aгент Cilium при старте читает итоговую конфигурацию, поэтому после
создания или изменения CiliumNodeConfig нужно перезапустить соответствующие
поды `cilium-agent` (Deckhouse не делает это автоматически):

- на одном узле:

  ```bash
  d8 k -n d8-cni-cilium delete pod \
  -l app=agent --field-selector spec.nodeName=<node-name>
  ```

- на всех узлах кластера (rolling restart):

  ```bash
  d8 k -n d8-cni-cilium rollout restart daemonset/agent
  ```

Чтобы откатить изменения (выключить трейсы), удалите ресурс CiliumNodeConfig и перезапустите те же
поды.
