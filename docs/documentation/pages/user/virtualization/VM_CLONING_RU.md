---
title: "Клонирование виртуальных машин"
permalink: ru/user/virtualization/vm-cloning.html
description: "Клонирование виртуальной машины: создание копии работающей машины и восстановление копии из снимка с заменой имён и параметров."
search: клонирование ВМ, клон виртуальной машины, nameReplacements
lang: ru
---

Клон виртуальной машины создаётся либо из уже существующей ВМ, либо из ранее созданного снимка этой машины.

{% alert level="warning" %}
Клонируемой ВМ будет назначен новый IP-адрес для кластерной сети и MAC-адреса для дополнительных сетевых интерфейсов (если они есть), поэтому после клонирования потребуется перенастроить сетевые параметры гостевой ОС.
{% endalert %}

{% alert level="info" %}
Лейблы не копируются с исходной ВМ на клон. Это предотвращает маршрутизацию трафика Service (Service выбирают ВМ по меткам) на клон. Если клон должен входить в Service, добавьте нужные лейблы после клонирования. Например:

```bash
d8 k label vm <VM_NAME> label-name=label-value
```
{% endalert %}

Клонирование создаёт копию ВМ, поэтому ресурсы новой ВМ должны иметь уникальные имена. Для этого используются параметры `nameReplacements` и/или `customization`:

- `nameReplacements` — позволяет заменить имена существующих ресурсов на новые, чтобы избежать конфликтов.
- `customization` — задаёт префикс или суффикс для имен всех клонируемых ресурсов ВМ (дисков, IP-адресов и так далее).

Пример переименования конкретных ресурсов:

```yaml
nameReplacements:
  - from:
      kind: VirtualMachine
      name: <OLD_VM_NAME>
    to:
      name: <NEW_VM_NAME>
  - from:
      kind: VirtualDisk
      name: <OLD_DISK_NAME>
    to:
      name: <NEW_DISK_NAME>
  ...
```

В результате будет создана ВМ с именем `<NEW_VM_NAME>`, а указанные ресурсы будут переименованы согласно правилам замены.

Пример добавления префикса или суффикса ко всем ресурсам:

```yaml
customization:
  namePrefix: <PREFIX>
  nameSuffix: <SUFFIX>
```

В результате будет создана ВМ с именем `<PREFIX><ORIGINAL_VM_NAME><SUFFIX>`, а все ресурсы (диски, IP-адреса и так далее) получат префикс и суффикс.

Для операции клонирования возможно использовать один из трех режимов:

- `DryRun` — тестовый запуск для проверки возможных конфликтов. Результаты отображаются в поле `status.resources` соответствующего ресурса операции.
- `Strict` — строгий режим, требующий наличия всех ресурсов с новыми именами и их зависимостей (например, образов) в клонируемой ВМ.
- `BestEffort` — режим, при котором отсутствующие внешние зависимости (например, [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), [VirtualImage](/modules/virtualization/cr.html#virtualimage)) автоматически удаляются из конфигурации клонируемой ВМ.

Информацию о конфликтах, возникших при клонировании, можно просмотреть в статусе ресурса операции:

```bash
# Для клонирования из существующей ВМ.
d8 k get vmop <VMOP_NAME> -o json | jq '.status.resources'

# Для клонирования из снимка ВМ.
d8 k get vmsop <VMSOP_NAME> -o json | jq '.status.resources'
```

## Создание клона существующей ВМ

Клон собирается из временных снимков машины, поэтому останавливать её не нужно.

{% tabs vm-clone %}

{% tab "В командной строке" %}

Клонирование ВМ выполняется с использованием ресурса [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) с типом операции `Clone`.

Клонирование поддерживается как для выключенных, так и для работающих виртуальных машин. При клонировании работающей ВМ автоматически создаётся консистентный снимок, из которого затем формируется клон.

> Рекомендуется задавать параметр `.spec.runPolicy: AlwaysOff` в конфигурации клонируемой ВМ, чтобы предотвратить автоматический запуск клона ВМ. Это связано с тем, что клон наследует поведение родительской ВМ.

Перед клонированием необходимо подготовить гостевую ОС, чтобы избежать конфликтов уникальных идентификаторов и сетевых настроек.

Linux:

- очистить `machine-id` с помощью команды `sudo truncate -s 0 /etc/machine-id` (для systemd) или удалить файл `/var/lib/dbus/machine-id`;
- удалить SSH-ключи хоста: `sudo rm -f /etc/ssh/ssh_host_*`;
- очистить конфигурации сетевых интерфейсов (если используются статические настройки);
- очистить кеш Cloud-Init (если используется): `sudo cloud-init clean`.

Windows:

- выполнить генерализацию с помощью `sysprep` с параметром `/generalize` или использовать инструменты для очистки уникальных идентификаторов (SID, hostname и так далее).

Для создания клона ВМ используйте следующий ресурс:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: <VMOP_NAME>
spec:
  type: Clone
  virtualMachineName: <name of the VM to be cloned>
  clone:
    mode: DryRun | Strict | BestEffort
    nameReplacements: []
    customization: {}
```

Параметры `nameReplacements` и `customization` настраиваются в блоке [`.spec.clone`](/modules/virtualization/cr.html#virtualmachineoperation-v1alpha2-spec-clone) (общее описание выше).

> В процессе клонирования для виртуальной машины и всех её дисков автоматически создаются временные снимки. Именно из этих снимков затем собирается новая ВМ. После завершения процесса клонирования временные снимки автоматически удаляются — их не будет видно в списке ресурсов. Однако внутри спецификации клонируемых дисков будет оставаться ссылка (`dataSource`) на соответствующий снимок, даже если самого снимка уже не существует. Это ожидаемое поведение и не свидетельствует о проблемах, потому что такие ссылки корректны, потому что к моменту запуска клона все необходимые данные уже были перенесены на новые диски.

В следующем примере показано клонирование ВМ с именем `database` и подключенного к ней диска `database-root`:

Пример с переименованием конкретных ресурсов:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: clone-database
spec:
  type: Clone
  virtualMachineName: database
  clone:
    mode: Strict
    nameReplacements:
      - from:
          kind: VirtualMachine
          name: database
        to:
          name: database-clone
      - from:
          kind: VirtualDisk
          name: database-root
        to:
          name: database-clone-root
```

В результате будет создана ВМ с именем `database-clone` и диск с именем `database-clone-root`.

Пример с использованием префикса для всех ресурсов:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: clone-database
spec:
  type: Clone
  virtualMachineName: database
  clone:
    mode: Strict
    customization:
      namePrefix: clone-
      nameSuffix: -prod
```

В результате будет создана ВМ с именем `clone-database-prod` и диск с именем `clone-database-root-prod`.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Из списка выберите нужную виртуальную машину и нажмите кнопку с многоточием.
1. В открывшемся меню выберите «Клонировать».
1. В открывшемся окне «Клонирование машины» в поле «Имя снимка виртуальной машины» выберите снимок, из которого будет создан клон. Клон создаётся из снимка, поэтому снимок нужно подготовить заранее.
1. В поле «Режим клонирования» выберите `Strict` или `BestEffort`.
1. При необходимости в блоке «Кастомизация» → «Переименование ресурсов» задайте новые имена ресурсов клона, указав тип ресурса, исходное и новое имя.
1. Нажмите кнопку «Клонировать».

{% endtab %}

{% endtabs %}

## Создание клона из снимка ВМ

Клонирование ВМ из снимка выполняется с использованием ресурса [VirtualMachineSnapshotOperation](/modules/virtualization/cr.html#virtualmachinesnapshotoperation) с типом операции `CreateVirtualMachine`.

Для создания клона ВМ из снимка используйте следующий ресурс:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshotOperation
metadata:
  name: <VMSOP_NAME>
spec:
  type: CreateVirtualMachine
  virtualMachineSnapshotName: <name of the VM snapshot from which to clone>
  createVirtualMachine:
    mode: DryRun | Strict | BestEffort
    nameReplacements: []
    customization: {}
```

Параметры `nameReplacements` и `customization` настраиваются в блоке [`.spec.createVirtualMachine`](/modules/virtualization/cr.html#virtualmachinesnapshotoperation-v1alpha2-spec-createvirtualmachine) (общее описание выше).

Чтобы посмотреть список ресурсов, сохранённых в снимке, используйте команду:

```bash
d8 k get vmsnapshot <SNAPSHOT_NAME> -o jsonpath='{.status.resources}' | jq
```

{% alert level="info" %}
При клонировании ВМ из снимка связанные с ней диски также создаются из соответствующих снимков, поэтому в спецификации диска будет указан параметр `dataSource` с ссылкой на нужный снимок диска.
{% endalert %}

В следующем примере показано клонирование из снимка ВМ с именем `database-snapshot`, который содержит ВМ `database` и диск `database-root`:

Пример с переименованием конкретных ресурсов:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshotOperation
metadata:
  name: clone-database-from-snapshot
spec:
  type: CreateVirtualMachine
  virtualMachineSnapshotName: database-snapshot
  createVirtualMachine:
    mode: Strict
    nameReplacements:
      - from:
          kind: VirtualMachine
          name: database
        to:
          name: database-clone
      - from:
          kind: VirtualDisk
          name: database-root
        to:
          name: database-clone-root
```

В результате будет создана ВМ с именем `database-clone` и диск с именем `database-clone-root`.

Пример с использованием префикса для всех ресурсов:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshotOperation
metadata:
  name: clone-database-from-snapshot
spec:
  type: CreateVirtualMachine
  virtualMachineSnapshotName: database-snapshot
  createVirtualMachine:
    mode: Strict
    customization:
      namePrefix: clone-
      nameSuffix: -prod
```

В результате будет создана ВМ с именем `clone-database-prod` и диск с именем `clone-database-root-prod`.
