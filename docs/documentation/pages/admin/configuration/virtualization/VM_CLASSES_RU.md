---
title: "Классы виртуальных машин"
permalink: ru/admin/configuration/virtualization/vm-classes.html
description: "Ресурс VirtualMachineClass: класс по умолчанию, состав настроек и порядок их применения к виртуальным машинам проекта."
search: VirtualMachineClass, класс виртуальной машины, класс по умолчанию
lang: ru
---

Класс виртуальной машины (ВМ) задаёт то, что владелец проекта не настраивает сам, а именно модель виртуального процессора, допустимые сочетания ядер и памяти, а также узлы, на которых ВМ может работать. Описывает эти правила ресурс [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass), и через него вы управляете тем, как рабочие нагрузки проектов распределяются по узлам кластера.

При первичной установке модуль создаёт класс `generic` с моделью процессора Nehalem. Эта модель старая, но поддерживается любым современным процессором, поэтому ВМ такого класса запускаются на любом узле кластера и мигрируют между узлами без ограничений.

{% alert level="info" %}
Класс `generic` соответствует процессору с наименьшим набором инструкций, поэтому для рабочих нагрузок в production он не подходит.

Когда все узлы добавлены в кластер и настроены, создайте хотя бы один класс с типом процессора `Discovery`. DP подберёт для него набор инструкций, доступный на всех узлах сразу, и виртуальные машины смогут использовать возможности процессоров полнее, сохранив способность мигрировать между узлами. Набор инструкций фиксируется в момент создания ресурса и не меняется, когда узлы добавляются или удаляются.

Как настроить такой класс, показано в разделе [«Пример конфигурации vCPU Discovery»](vm-classes-cpu.html#пример-конфигурации-vcpu-discovery).
{% endalert %}

Классы существуют на уровне кластера. Чтобы вывести их список, выполните команду:

```bash
d8 k get virtualmachineclass
```

Пример вывода:

```console
NAME      PHASE   ISDEFAULT   AGE
generic   Ready               6d1h
```

Изменить у любого класса можно всё, кроме блока [`.spec.cpu`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu), потому что модель процессора фиксируется при создании ресурса. Класс `generic` разрешено и менять, и удалять, но заново он не создастся, потому что модуль добавляет его только при первичной установке.

Владелец проекта указывает класс в параметре [`.spec.virtualMachineClassName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineclassname) виртуальной машины:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  virtualMachineClassName: generic # Название ресурса VirtualMachineClass.
  # ...
```

## VirtualMachineClass по умолчанию

Один из классов можно назначить классом по умолчанию. DP подставит его имя в параметр [`.spec.virtualMachineClassName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineclassname), если владелец проекта не указал класс сам.

Класс по умолчанию помечается аннотацией `virtualmachineclass.virtualization.deckhouse.io/is-default-class` со значением `true`. Такой класс в кластере может быть только один, поэтому, чтобы назначить новый, сначала снимите аннотацию с текущего.

Не ставьте аннотацию на класс `generic`, потому что при обновлении модуля она может пропасть. Создайте собственный класс и назначьте по умолчанию его.

1. Посмотрите, какие классы есть в кластере:

   ```bash
   d8 k get vmclass
   ```

   Пример вывода, в котором класса по умолчанию нет:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                      PHASE   ISDEFAULT   AGE
   generic                   Ready               1d
   host-passthrough-custom   Ready               1d
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Назначьте класс по умолчанию:

   ```bash
   d8 k annotate vmclass host-passthrough-custom virtualmachineclass.virtualization.deckhouse.io/is-default-class=true
   ```

1. Убедитесь, что аннотация проставлена:

   ```bash
   d8 k get vmclass
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                      PHASE   ISDEFAULT   AGE
   generic                   Ready               1d
   host-passthrough-custom   Ready   true        1d
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

Теперь виртуальные машины, созданные без указания класса, получат класс `host-passthrough-custom`.

## Настройки VirtualMachineClass

Класс состоит из трёх блоков, каждый из которых отвечает за свою группу настроек:

{% tabs vmclass-create %}

{% tab "В командной строке" %}

Опишите класс в ресурсе VirtualMachineClass:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: <VMCLASS_NAME>
  # Аннотация назначает класс классом по умолчанию, её можно не указывать.
  # annotations:
  #   virtualmachineclass.virtualization.deckhouse.io/is-default-class: "true"
spec:
  # Параметры виртуального процессора. Блок обязателен и после создания ресурса не меняется.
  cpu: ...

  # Правила размещения виртуальных машин по узлам. Блок необязателен.
  # Изменения применяются ко всем машинам этого класса.
  nodeSelector: ...

  # Политика подбора ресурсов для виртуальных машин. Блок необязателен.
  # Изменения применяются ко всем машинам этого класса.
  sizingPolicies: ...
```

Здесь `<VMCLASS_NAME>` — имя создаваемого класса.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Система», далее в раздел «Виртуализация» → «Классы ВМ».
1. Нажмите кнопку «Создать».
1. В открывшейся форме в поле «Имя» введите имя класса ВМ.

{% endtab %}

{% endtabs %}

Дальше блоки разобраны по отдельности.
