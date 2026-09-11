---
title: "Виртуальный процессор виртуальных машин"
permalink: ru/admin/configuration/virtualization/vm-classes-cpu.html
description: "Типы виртуального процессора в VirtualMachineClass и автоматический подбор набора инструкций через vCPU Discovery."
search: виртуальный процессор, vCPU Discovery, тип CPU, набор инструкций
lang: ru
---

Класс виртуальной машины задаёт, какой процессор и какой набор инструкций увидит гостевая система.

## Виртуальный процессор

Блок [`.spec.cpu`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu) определяет, какой процессор увидит гостевая ОС. От него зависит и то, между какими узлами ВМ сможет мигрировать.

{% alert level="warning" %}
Блок [`.spec.cpu`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu) после создания ресурса изменить нельзя. Чтобы задать другой процессор, создайте новый класс.
{% endalert %}

Ниже приведены примеры для каждого типа процессора.

- Набор процессорных инструкций, обязательных для ВМ. Задаётся типом `Features`:

  ```yaml
  spec:
    cpu:
      features:
        - vmx
      type: Features
  ```

  Как настроить vCPU в веб-интерфейсе в [форме создания классов ВМ](vm-classes.html#настройки-virtualmachineclass):

  1. В блоке «Настройки ЦП» в поле «Тип» выберите `Features`.
  1. В поле «Обязательный набор поддерживаемых инструкций» выберите нужные инструкции.
  1. Нажмите кнопку «Создать».

- Универсальный процессор для заданного набора узлов. Задаётся типом `Discovery`:

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

  Как выполнить операцию в веб-интерфейсе в [форме создания классов ВМ](vm-classes.html#настройки-virtualmachineclass):

  1. В блоке «Настройки ЦП» в поле «Тип» выберите `Discovery`.
  1. Нажмите кнопку «Добавить» в блоке «Условия для создания универсального процессора» → «Лейблы и выражения».
  1. Задайте «Ключ», «Оператор» и «Значение», они соответствуют параметру [`.spec.cpu.discovery.nodeSelector`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu-discovery-nodeselector).
  1. Нажмите клавишу «Enter», чтобы подтвердить параметры ключа.
  1. Нажмите кнопку «Создать».

- Процессор, близкий к процессору узла. Задаётся типом `Host`. Гостевая ОС получает почти полный набор инструкций узла, поэтому производительность выше, чем у фиксированной модели.
  ВМ такого класса мигрирует только между узлами со схожими процессорами. Например, между узлами с процессорами Intel и AMD миграция невозможна, как и между процессорами разных поколений, если их наборы инструкций различаются.

  ```yaml
  spec:
    cpu:
      type: Host
  ```

  Как выполнить операцию в веб-интерфейсе в [форме создания классов ВМ](vm-classes.html#настройки-virtualmachineclass):

  1. В блоке «Настройки ЦП» в поле «Тип» выберите `Host`.
  1. Нажмите кнопку «Создать».

- Процессор узла без изменений. Задаётся типом `HostPassthrough`. ВМ такого класса мигрирует только на узел, процессор которого в точности совпадает с процессором исходного узла.

  ```yaml
  spec:
    cpu:
      type: HostPassthrough
  ```

  Как выполнить операцию в веб-интерфейсе в [форме создания классов ВМ](vm-classes.html#настройки-virtualmachineclass):

  1. В блоке «Настройки ЦП» в поле «Тип» выберите `HostPassthrough`.
  1. Нажмите кнопку «Создать».

- Конкретная модель процессора с заранее известным набором инструкций. Задаётся типом `Model`.
  Сначала посмотрите, какие модели поддерживает нужный узел:

  ```bash
  d8 k get nodes <NODE_NAME> -o json | jq '.metadata.labels | to_entries[] | select(.key | test("cpu-model.node.virtualization.deckhouse.io")) | .key | split("/")[1]' -r
  ```

  Здесь `<NODE_NAME>` — имя узла кластера.

  Пример вывода:

  ```console
  Broadwell-noTSX
  Broadwell-noTSX-IBRS
  Haswell-noTSX
  Haswell-noTSX-IBRS
  IvyBridge
  IvyBridge-IBRS
  Nehalem
  Nehalem-IBRS
  Penryn
  SandyBridge
  SandyBridge-IBRS
  Skylake-Client-noTSX-IBRS
  Westmere
  Westmere-IBRS
  ```

  Затем укажите выбранную модель в спецификации класса:

  ```yaml
  spec:
    cpu:
      model: IvyBridge
      type: Model
  ```

  Как выполнить операцию в веб-интерфейсе в [форме создания классов ВМ](vm-classes.html#настройки-virtualmachineclass):

  1. В блоке «Настройки ЦП» в поле «Тип» выберите `Model`.
  1. В поле «Модель» выберите модель процессора.
  1. Нажмите кнопку «Создать».

## Пример конфигурации vCPU Discovery

Ниже показано, как подобрать типы процессора в кластере с разнородными узлами.

![Пример конфигурации VirtualMachineClass](../../../images/virtualization/vmclass-examples.ru.png)

Ниже разобран кластер из четырёх узлов. Два узла с лейблом `group=blue` оснащены процессором «CPU X» с тремя наборами инструкций, два других с лейблом `group=green` — более новым процессором «CPU Y» с четырьмя наборами.

{% alert level="info" %}
Набор инструкций процессора — это все команды, которые он умеет выполнять, от сложения до работы с памятью. От набора зависит, какие программы запустятся и насколько быстро, а у разных поколений процессоров наборы различаются.
{% endalert %}

Такому кластеру подойдут три класса:

- `universal` — ВМ запускаются на любом узле и мигрируют между всеми четырьмя. Модуль возьмёт набор инструкций, общий для обоих процессоров, поэтому совместимость максимальная, а часть возможностей «CPU Y» останется неиспользованной;
- `cpuX` — ВМ запускаются только на узлах с «CPU X» и мигрируют между ними, используя все инструкции этого процессора;
- `cpuY` — то же самое для узлов с «CPU Y».

Классы для такого кластера выглядят так:

```yaml
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: universal
spec:
  cpu:
    # Пустой discovery означает, что учитываются все узлы кластера.
    discovery: {}
    type: Discovery
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuX
spec:
  cpu:
    discovery:
      nodeSelector:
        matchExpressions:
          - key: group
            operator: In
            values: ["blue"]
    type: Discovery
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
```
