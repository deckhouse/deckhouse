---
title: "Deckhouse Virtualization Platform"
permalink: ru/virtualization-platform/documentation/admin/platform-management/virtualization/virtual-machine-classes.html
lang: ru
---

Ресурс [VirtualMachineClass](../../../../reference/cr.html#virtualmachineclass) предназначен для централизованной конфигурации предпочтительных параметров виртуальных машин. Он позволяет определять инструкции CPU и политики конфигурации ресурсов CPU и памяти для виртуальных машин, а также определять соотношения этих ресурсов. Помимо этого, [VirtualMachineClass](../../../../reference/cr.html#virtualmachineclass) обеспечивает управление размещением виртуальных машин по узлам платформы. Это позволяет администраторам эффективно управлять ресурсами платформы виртуализации и оптимально размещать виртуальные машины на узлах платформы.

Платформа виртуализации предоставляет 3 предустановленных ресурса VirtualMachineClass:

```bash
kubectl get virtualmachineclass
NAME               PHASE   AGE
host               Ready   6d1h
host-passthrough   Ready   6d1h
generic            Ready   6d1h
```

- `host` - данный класс использует виртуальный CPU, максимально близкий к CPU узла платформы по набору инструкций. Это обеспечивает высокую производительность и функциональность, а также совместимость с живой миграцией для узлов с похожими типами процессоров. Например, миграция ВМ между узлами с процессорами Intel и AMD не будет работать. Это также справедливо для процессоров разных поколений, так как набор инструкций у них отличается.
- `host-passthrough` - используется физический CPU узла платформы напрямую без каких-либо изменений. При использовании данного класса, гостевая ВМ может быть мигрирована только на целевой узел, у которого CPU точно соответствует CPU исходного узла.
- `generic` - универсальная модель CPU, использующая достаточно старую, но поддерживаемую большинством современных процессоров модель Nehalem. Это позволяет запускать ВМ на любых узлах кластера с возможностью живой миграции.

VirtualMachineClass является обязательным для указания в конфигурации виртуальной машины, пример того как указывать класс в спецификации ВМ:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  virtualMachineClassName: generic # название ресурса VirtualMachineClass
  ...
```

{% alert level="warning" %}
Внимание! Рекомендуется создать как минимум один ресурс VirtualMachineClass в кластере с типом Discovery сразу после того как все узлы будут настроены и добавлены в кластер. Это позволит использовать в виртуальных машинах универсальный процессор с максимально возможными характеристиками с учетом ЦП на узлах кластера, что позволит виртуальным машинам использовать максимум возможностей ЦП и при необходимости беспрепятственно осуществлять миграцию между узлами кластера.
{% endalert %}

Администраторы платформы могут создавать требуемые классы виртуальных машин по своим потребностям, но рекомендуется создавать необходимый минимум. Рассмотрим на следующем примере:

### Пример конфигурации VirtualMachineClass

![](/images/virtualization-platform/vmclass-examples.ru.png)

Представим, что у нас есть кластер из четырех узлов. Два из этих узлов с лейблом `group=blue` оснащены процессором "CPU X" с тремя наборами инструкций, а остальные два узла с лейблом `group=green` имеют более новый процессор "CPU Y" с четырьмя наборами инструкций.

Для оптимального использования ресурсов данного кластера, рекомендуется создать три дополнительных класса виртуальных машин (VirtualMachineClass):

- **universal**: Этот класс позволит виртуальным машинам запускаться на всех узлах платформы и мигрировать между ними. При этом будет использоваться набор инструкций для самой младшей модели CPU, что обеспечит наибольшую совместимость.
- **cpuX**: Этот класс будет предназначен для виртуальных машин, которые должны запускаться только на узлах с процессором "CPU X". ВМ смогут мигрировать между этими узлами, используя доступные наборы инструкций "CPU X".
- **cpuY**: Этот класс предназначен для виртуальных машин, которые должны запускаться только на узлах с процессором "CPU Y". ВМ смогут мигрировать между этими узлами, используя доступные наборы инструкций "CPU Y".

> Наборы инструкций для процессора — это список всех команд, которые процессор может выполнять, таких как сложение, вычитание или работа с памятью. Они определяют, какие операции возможны, влияют на совместимость программ и производительность, а также могут меняться от одного поколения процессоров к другому.

Примерные конфигурации ресурсов для данного кластера:

```yaml
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: universal
spec:
  cpu:
    discovery: {}
    type: Discovery
  sizingPolicies: { ... }
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuX
spec:
  cpu:
    discovery: {}
    type: Discovery
  nodeSelector:
    matchExpressions:
      - key: group
        operator: In
        values: ["blue"]
  sizingPolicies: { ... }
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuY
spec:
  cpu:
    discovery:
      nodeSelector:
        matchExpressions:
          - key: group
            operator: In
            values: ["green"]
    type: Discovery
  sizingPolicies: { ... }
```

### Прочие варианты конфигурации

Пример конфигурации ресурса VirtualMachineClass:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: discovery
spec:
  cpu:
    # сконфигурировать универсальный vCPU для заданного набора узлов
    discovery:
      nodeSelector:
        matchExpressions:
          - key: node-role.kubernetes.io/control-plane
            operator: DoesNotExist
    type: Discovery
  # разрешать запуск ВМ с данным классом только на узлах группы worker
  nodeSelector:
    matchExpressions:
      - key: node.deckhouse.io/group
        operator: In
        values:
          - worker
  # политика конфигурации ресурсов
  sizingPolicies:
    # для диапазона от 1 до 4 ядер возможно использовать от 1 до 8 Гб оперативной памяти с шагом 512Mi
    # т.е 1Гб, 1,5Гб, 2Гб, 2,5Гб итд
    # запрещено использовать выделенные ядра
    # и доступны все варианты параметра corefraction
    - cores:
        min: 1
        max: 4
      memory:
        min: 1Gi
        max: 8Gi
        step: 512Mi
      dedicatedCores: [false]
      coreFractions: [5, 10, 20, 50, 100]
    # для диапазона от 5 до 8 ядер возможно использовать от 5 до 16 Гб оперативной памяти с шагом 1Гб
    # т.е. 5Гб, 6Гб, 7Гб, итд
    # запрещено использовать выделенные ядра
    # и доступны некоторые варианты параметра corefraction
    - cores:
        min: 5
        max: 8
      memory:
        min: 5Gi
        max: 16Gi
        step: 1Gi
      dedicatedCores: [false]
      coreFractions: [20, 50, 100]
    # для диапазона от 9 до 16 ядер возможно использовать от 9 до 32 Гб оперативной памяти с шагом 1Гб
    # можно использовать выделенные ядра (а можно и не использовать)
    # и доступны некоторые варианты параметра corefraction
    - cores:
        min: 9
        max: 16
      memory:
        min: 9Gi
        max: 32Gi
        step: 1Gi
      dedicatedCores: [true, false]
      coreFractions: [50, 100]
    # для диапазона от 17 до 1024 ядер возможно использовать от 1 до 2 Гб оперативной памяти из расчета на одно ядро
    # доступны для использования только выделенные ядра
    # и единственный параметр corefraction = 100%
    - cores:
        min: 17
        max: 1024
      memory:
        perCore:
          min: 1Gi
          max: 2Gi
      dedicatedCores: [true]
      coreFractions: [100]
```

Далее приведены фрагменты конфигураций VirtualMachineClass для решения различных задач:

- класс с vCPU с требуемым набором процессорных инструкций, для этого используем `type: Features`, чтобы задать необходимый набор поддерживаемых инструкций для процессора:

  ```yaml
  spec:
    cpu:
      features:
        - vmx
      type: Features
  ```

- класс c универсальным vCPU для заданного набора узлов, для этого используем `type: Discovery`:

  ```yaml
  spec:
    cpu:
      discovery:
        nodeSelector:
          matchExpressions:
            - key: node-role.kubernetes.io/control-plane
              operator: DoesNotExist
      type: Discovery
  ```

- чтобы создать vCPU конкретного процессора с предварительно определенным набором инструкций, используем тип `type: Model`. Предварительно, чтобы получить перечень названий поддерживаемых CPU для узла кластера, выполните команду:

  ```bash
  kubectl get nodes <node-name> -o json | jq '.metadata.labels | to_entries[] | select(.key | test("cpu-model")) | .key | split("/")[1]' -r

  # Примерный вывод:
  #
  # IvyBridge
  # Nehalem
  # Opteron_G1
  # Penryn
  # SandyBridge
  # Westmere
  ```

далее указать в спецификации ресурса VirtualMachineClass:

```yaml
spec:
  cpu:
    model: IvyBridge
    type: Model
```
