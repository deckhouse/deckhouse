---
title: "Снимки"
permalink: ru/virtualization-platform/documentation/user/resource-management/snapshots.html
lang: ru
---

Снимки позволяют зафиксировать текущее состояние ресурса для последующего восстановления или клонирования: снимок диска сохраняет только данные выбранного диска, а снимок виртуальной машины включает в себя параметры ВМ и состояние всех её дисков.

## Консистентные снимки

Снимки могут быть консистентными и неконсистентными. За это отвечает параметр `requiredConsistency`, по умолчанию его значение равно `true`, что означает требование консистентного снимка.

Консистентный снимок фиксирует согласованное и целостное состояние данных диска. Такой снимок можно создать при выполнении одного из следующих условий:

- диск не подключён ни к одной виртуальной машине — снимок всегда будет консистентным;
- виртуальная машина выключена;
- в гостевой ОС установлен и запущен [`qemu-guest-agent`](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#агент-гостевой-ос). При создании снимка он временно приостанавливает («замораживает») работу файловой системы, чтобы обеспечить согласованность данных.

Неконсистентный снимок может не отражать согласованное состояние дисков виртуальной машины и её компонентов. Такой снимок создаётся, если ВМ запущена, и в гостевой ОС не установлен или не запущен `qemu-guest-agent`.
Если в манифесте снимка явно указан параметр `requiredConsistency: false`, но `qemu-guest-agent` при этом запущен, будет также предпринята попытка заморозки файловой системы, чтобы снимок получился консистентным.

QEMU Guest Agent поддерживает скрипты hooks, которые позволяют подготовить приложения к созданию снимка без остановки сервисов, обеспечивая согласованное состояние на уровне приложений. Подробнее о настройке скриптов hooks см. в разделе [«Агент гостевой ОС»](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#агент-гостевой-ос).

{% alert level="warning" %}
При восстановлении из такого снимка возможны проблемы с целостностью файловой системы, поскольку состояние данных может быть не согласовано.
{% endalert %}

## Создание снимков дисков

Для создания снимков виртуальных дисков используется ресурс `VirtualDiskSnapshot` . Эти снимки могут служить источником данных при создании новых дисков, например, для клонирования или восстановления информации.

Чтобы гарантировать целостность данных, снимок диска можно создать в следующих случаях:

- Диск не подключен ни к одной виртуальной машине.
- ВМ выключена.
- ВМ запущена, но установлен qemu-guest-agent в гостевой ОС.
  Файловая система успешно «заморожена» (операция fsfreeze).

Если консистентность данных не требуется (например, для тестовых сценариев), снимок можно создать:

- На работающей ВМ без «заморозки» файловой системы.
- Даже если диск подключен к активной ВМ.

Для этого в манифесте VirtualDiskSnapshot укажите:

```yaml
spec:
  requiredConsistency: false
```

Пример манифеста для создания снимка диска:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDiskSnapshot
metadata:
  name: linux-vm-root-snapshot
spec:
  requiredConsistency: true
  virtualDiskName: linux-vm-root
EOF
```

Для просмотра списка снимков дисков, выполните следующую команду:

```bash
d8 k get vdsnapshot
```

Пример вывода:

```console
NAME                   PHASE     CONSISTENT   AGE
linux-vm-root-snapshot Ready     true         3m2s
```

Поле `CONSISTENT` показывает, является ли снимок консистентным (`true`) или нет (`false`). Значение определяется автоматически на основе условий создания снимка и не может быть изменено.

После создания `VirtualDiskSnapshot` может находиться в следующих состояниях (фазах):

- `Pending` — ожидание готовности всех зависимых ресурсов, требующихся для создания снимка.
- `InProgress` — идет процесс создания снимка виртуального диска.
- `Ready` — создание снимка успешно завершено, и снимок виртуального диска доступен для использования.
- `Failed` — произошла ошибка во время процесса создания снимка виртуального диска.
- `Terminating` — ресурс находится в процессе удаления.

Диагностика проблем с ресурсом осуществляется путем анализа информации в блоке `.status.conditions`.

С полным описанием параметров конфигурации ресурса `VirtualDiskSnapshot` машин можно ознакомиться [в документации ресурса](/modules/virtualization/cr.html#virtualdisksnapshot).

Как создать снимок диска в веб-интерфейсе:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» → «Снимки дисков».
- Нажмите «Создать снимок диска».
- В поле «Имя снимка диска» введите имя для снимка.
- На вкладке «Конфигурация» в поле «Имя диска» выберите диск, с которого будет создан снимок.
- Включите переключатель «Гарантия целостности».
- Нажмите кнопку «Создать».
- Статус образа отображается слева вверху, под именем снимка.

## Восстановление дисков из снимков

Для того чтобы восстановить диск из ранее созданного снимка диска, необходимо в качестве `dataSource` указать соответствующий объект:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root
spec:
  # Настройки параметров хранения диска.
  persistentVolumeClaim:
    # Укажем размер больше чем значение .
    size: 10Gi
    # Подставьте ваше название StorageClass.
    storageClassName: rv-thin-r2
  # Источник из которого создается диск.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDiskSnapshot
      name: linux-vm-root-snapshot
EOF
```

Как восстановить диск из ранее созданного снимка в веб-интерфейсе:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» → «Диски ВМ».
- Нажмите «Создать диск».
- В открывшейся форме в поле «Имя диска» введите имя для диска.
- В поле «Источник» убедитесь, что установлен чек-бокс «Снимки».
- Из выпадающего списка выберите снимок диска, из которого хотите восстановиться.
- В поле «Размер» установите размер такой же или больше, чем размер оригинального диска.
- В поле «Имя StorageClass» введите «StorageClass» оригинального диска.
- Нажмите кнопку «Создать».
- Статус диска отображается слева вверху, под именем диска.

## Создание снимков ВМ

Снимок виртуальной машины — это сохранённое состояние виртуальной машины в определённый момент времени. Для создания снимков виртуальных машин используется ресурс `VirtualMachineSnapshot`.

{% alert level="warning" %}
Рекомендуется отключить все образы (VirtualImage/ClusterVirtualImage) от виртуальной машины перед созданием её снимка. Образы дисков не сохраняются вместе со снимком ВМ, и их отсутствие в кластере при восстановлении может привести к тому, что виртуальная машина не сможет запуститься и будет находиться в состоянии `Pending`, ожидая доступности образа.
{% endalert %}

### Создание снимков

Создание снимка виртуальной машины будет неудачным, если выполнится хотя бы одно из следующих условий:

- Не все зависимые устройства виртуальной машины готовы;
- Среди зависимых устройств есть диск, находящийся в процессе изменения размера.

{% alert level="warning" %}
Если на момент создания снимка в виртуальной машине есть изменения, ожидающие перезапуска, в снимок попадёт обновлённая конфигурация.
{% endalert %}

При создании снимка динамический IP-адрес ВМ автоматически преобразуется в статический и сохраняется для восстановления.

Если не требуется преобразование и использование старого IP-адреса виртуальной машины, можно установить соответствующую политику в значение `Never`. В этом случае будет использован тип адреса без преобразования (`Auto` или `Static`).

```yaml
spec:
  keepIPAddress: Never
```

Пример манифеста для создания снимка виртуальной машины:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshot
metadata:
  name: linux-vm-snapshot
spec:
  virtualMachineName: linux-vm
  requiredConsistency: true
  keepIPAddress: Never
EOF
```

После успешного создания снимка, в его статусе будет отражен перечень ресурсов, которые были сохранены в снимке.

Пример вывода:

```yaml
status:
  ...
  resources:
  - apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualMachine
    name: linux-vm
  - apiVersion: v1
    kind: Secret
    name: cloud-init
  - apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualDisk
    name: linux-vm-root
```

Как создать снимок ВМ в веб-интерфейсе:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» → «Виртуальные машины».
- Из списка выберите необходимую ВМ и нажмите на её имя.
- Перейдите на вкладку «Снимки».
- Нажмите кнопку «Создать».
- В открывшейся форме в поле «Имя снимка» введите `linux-vm-snapshot`.
- На вкладке «Конфигурация» в поле «Политика преобразования IP-адреса» выберите значение `Never`.
- Включите переключатель «Гарантия целостности».
- В поле «Класс хранилища снимка» выберите класс для снимка диска.
- Нажмите кнопку «Создать».
- Статус снимка отображается слева вверху, под именем снимка.

## Восстановление ВМ

Для восстановления ВМ из снимка используется ресурс `VirtualMachineOperation` с типом `restore`:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: <vmop-name>
spec:
  type: Restore
  virtualMachineName: <name of the VM to be restored>
  restore:
    mode: DryRun | Strict | BestEffort
    virtualMachineSnapshotName: <name of the VM snapshot from which to restore>
```

Для данной операции возможно использовать один из трех режимов:

- `DryRun` — холостой запуск операции восстановления, необходим для проверки возможных конфликтов, которые будут отображены в статусе ресурса (`status.resources`).
- `Strict` — режим строгого восстановления, когда требуется восстановление ВМ «как в снимке», отсутствующие внешние зависимости могут привести к тому, что ВМ после восстановления будет в `Pending`.
- `BestEffort` — отсутствующие внешние зависимости (`ClusterVirtualImage`, `VirtualImage`) игнорируются и удаляются из конфигурации ВМ.

Восстановление виртуальной машины из снимка возможно только при выполнении всех следующих условий:

- Восстанавливаемая ВМ присутствует в кластере (ресурс `VirtualMachine` существует, а его `.metadata.uid` совпадает с идентификатором, использованным при создании снимка).
- Восстанавливаемые диски (определяются по имени) либо не подключены к другим ВМ, либо отсутствуют в кластере.
- Восстанавливаемый IP-адрес либо не занят другой ВМ, либо отсутствует в кластере.
- Восстанавливаемые MAC-адреса либо не используются другими ВМ, либо отсутствуют в кластере.

{% alert level="warning" %}
Если некоторые ресурсы, от которых зависит ВМ (например, `VirtualMachineClass`, `VirtualImage`, `ClusterVirtualImage`), отсутствуют в кластере, но существовали на момент создания снимка, ВМ после восстановления останется в состоянии `Pending`.
В этом случае необходимо вручную отредактировать конфигурацию ВМ и обновить или удалить отсутствующие зависимости.
{% endalert %}

Информацию о конфликтах при восстановлении ВМ из снимка можно посмотреть в статусе ресурса:

```bash
d8 k get vmop <vmop-name> -o json | jq '.status.resources'
```

{% alert level="warning" %}
Не рекомендуется отменять операцию восстановления (удалять ресурс `VirtualMachineOperation` в фазе `InProgress`) из снимка, так как это может привести к неконсистентному состоянию восстанавливаемой виртуальной машины.
{% endalert %}

{% alert level="info" %}
При восстановлении ВМ из снимка связанные с ней диски также восстанавливаются из соответствующих снимков, поэтому в спецификации диска будет указан параметр `dataSource` со ссылкой на нужный снимок диска.
{% endalert %}

## Создание клона ВМ

Вы можете создать клон виртуальной машины двумя способами: либо на основании уже существующей ВМ, либо используя предварительно созданный снимок этой машины.

{% alert level="warning" %}
Клонируемой ВМ будет назначен новый IP-адрес для кластерной сети и MAC-адреса для дополнительных сетевых интерфейсов (если они есть), поэтому после клонирования потребуется перенастроить сетевые параметры гостевой ОС.
{% endalert %}

{% alert level="info" %}
Лейблы не копируются с исходной ВМ на клон. Это предотвращает маршрутизацию трафика Service (Service выбирают ВМ по меткам) на клон. Если клон должен входить в Service, добавьте нужные лейблы после клонирования. Например:

```bash
d8 k label vm <vm-name> label-name=label-value
```

{% endalert %}

Клонирование создает копию ВМ, поэтому ресурсы новой ВМ должны иметь уникальные имена. Для этого используются параметры `nameReplacements` и/или `customization`:

- `nameReplacements` — позволяет заменить имена существующих ресурсов на новые, чтобы избежать конфликтов;
- `customization` — задает префикс или суффикс для имен всех клонируемых ресурсов ВМ (дисков, IP-адресов и т. д.).

Пример переименования конкретных ресурсов:

```yaml
nameReplacements:
  - from:
      kind: VirtualMachine
      name: <old-vm-name>
    to:
      name: <new-vm-name>
  - from:
      kind: VirtualDisk
      name: <old-disk-name>
    to:
      name: <new-disk-name>
# ...
```

В результате будет создана ВМ с именем `<new-vm-name>`, а указанные ресурсы будут переименованы согласно правилам замены.

Пример добавления префикса или суффикса ко всем ресурсам:

```yaml
customization:
  namePrefix: <prefix>
  nameSuffix: <suffix>
```

В результате будет создана ВМ с именем `<prefix><original-vm-name><suffix>`, а все ресурсы (диски, IP-адреса и т. д.) получат префикс и суффикс.

Для операции клонирования возможно использовать один из трех режимов:

- `DryRun` — тестовый запуск для проверки возможных конфликтов. Результаты отображаются в поле `status.resources` соответствующего ресурса операции.
- `Strict` — строгий режим, требующий наличия всех ресурсов с новыми именами и их зависимостей (например, образов) в клонируемой ВМ;
- `BestEffort` — режим, при котором отсутствующие внешние зависимости (например, ClusterVirtualImage, VirtualImage) автоматически удаляются из конфигурации клонируемой ВМ.

Информацию о конфликтах, возникших при клонировании, можно просмотреть в статусе ресурса операции:

```bash
# Для клонирования из существующей ВМ.
d8 k get vmop <vmop-name> -o json | jq '.status.resources'
# Для клонирования из снимка ВМ.
d8 k get vmsop <vmsop-name> -o json | jq '.status.resources'
```

### Создание клона существующей ВМ

Клонирование ВМ выполняется с использованием ресурса VirtualMachineOperation с типом операции `Clone`.

Клонирование поддерживается как для выключенных, так и для работающих виртуальных машин. При клонировании работающей ВМ автоматически создаётся консистентный снимок, из которого затем формируется клон.

{% alert level="info" %}
Рекомендуется задавать параметр `.spec.runPolicy: AlwaysOff` в конфигурации клонируемой ВМ, чтобы предотвратить автоматический запуск клона ВМ. Это связано с тем, что клон наследует поведение родительской ВМ.
{% endalert %}

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
  name: <vmop-name>
spec:
  type: Clone
  virtualMachineName: <name of the VM to be cloned>
  clone:
    mode: DryRun | Strict | BestEffort
    nameReplacements: []
    customization: {}
```

Параметры `nameReplacements` и `customization` настраиваются в блоке `.spec.clone` (см. [общее описание](#создание-клона-вм) выше).

{% alert level="info" %}
В процессе клонирования для виртуальной машины и всех её дисков автоматически создаются временные снимки. Именно из этих снимков затем собирается новая ВМ. После завершения процесса клонирования временные снимки автоматически удаляются — их не будет видно в списке ресурсов. Однако внутри спецификации клонируемых дисков будет оставаться ссылка (`dataSource`) на соответствующий снимок, даже если самого снимка уже не существует. Это ожидаемое поведение и не свидетельствует о проблемах: такие ссылки корректны, потому что к моменту запуска клона все необходимые данные уже были перенесены на новые диски.
{% endalert %}

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

### Создание клона из снимка ВМ

Клонирование ВМ из снимка выполняется с использованием ресурса VirtualMachineSnapshotOperation с типом операции `CreateVirtualMachine`.

Для создания клона ВМ из снимка используйте следующий ресурс:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshotOperation
metadata:
  name: <vmsop-name>
spec:
  type: CreateVirtualMachine
  virtualMachineSnapshotName: <name of the VM snapshot from which to clone>
  createVirtualMachine:
    mode: DryRun | Strict | BestEffort
    nameReplacements: []
    customization: {}
```

Параметры `nameReplacements` и `customization` настраиваются в блоке `.spec.createVirtualMachine` (см. [общее описание](#создание-клона-вм) выше).

Чтобы посмотреть список ресурсов, сохранённых в снимке, используйте команду:

```bash
d8 k get vmsnapshot <snapshot-name> -o jsonpath='{.status.resources}' | jq
```

{% alert level="info" %}
При клонировании ВМ из снимка связанные с ней диски также создаются из соответствующих снимков, поэтому в спецификации диска будет указан параметр `dataSource` со ссылкой на нужный снимок диска.
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

## USB-устройства

{% alert level="warning" %}
Проброс USB-устройств доступен только в **Enterprise Edition (EE)** платформы Deckhouse Virtualization Platform.
{% endalert %}

DVP поддерживает проброс USB-устройств в виртуальные машины с использованием DRA (Dynamic Resource Allocation). В этом разделе описано, как использовать USB-устройства с виртуальными машинами.

Для проброса USB требуются:

- `containerd v2` — подробные требования к узлам кластера описаны в параметре [`defaultCRI`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri);
- [Kubernetes](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) версии не ниже 1.34;
- [Deckhouse Kubernetes Platform (DKP)](https://releases.deckhouse.ru/) версии не ниже 1.75.

### Обзор

DVP предоставляет два пользовательских ресурса для управления USB-устройствами:

- [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) (cluster-scoped) — представляет USB-устройство, обнаруженное на конкретном узле. Создаётся автоматически системой DRA при обнаружении USB-устройства на узле.
- [USBDevice](/modules/virtualization/cr.html#usbdevice) (namespace-scoped) — представляет USB-устройство, доступное для подключения к виртуальным машинам в заданном неймспейсе.

### Принцип работы

Проброс USB-устройства проходит через последовательный жизненный цикл — от обнаружения устройства на узле до подключения к виртуальной машине:

1. Драйвер DRA автоматически обнаруживает USB-устройства на узлах кластера и создаёт ресурсы [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice).

1. Администратор назначает неймспейс ресурсу [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice), установив поле `.spec.assignedNamespace`. Это делает устройство доступным в этом неймспейсе.

1. После назначения неймспейса контроллер автоматически создаёт соответствующий ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) в этом неймспейсе.

1. Устройство [USBDevice](/modules/virtualization/cr.html#usbdevice) подключается к виртуальной машине путём добавления в поле `.spec.usbDevices` ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).

### Быстрый старт

Следующие шаги описывают минимальный сценарий подключения USB-устройства к виртуальной машине:

1. Подключите USB-устройство к узлу кластера.
1. Убедитесь, что создан ресурс [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice):

   ```bash
   d8 k get nodeusbdevice
   ```

1. Назначьте неймспейс ресурсу [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice), установив `.spec.assignedNamespace`:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: NodeUSBDevice
   metadata:
     name: logitech-webcam
   spec:
     assignedNamespace: my-project
   EOF
   ```

1. Убедитесь, что в целевом неймспейсе создан соответствующий ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice):

   ```bash
   d8 k get usbdevice -n my-project
   ```

1. Добавьте устройство в поле `.spec.usbDevices` ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) и убедитесь, что ВМ размещена на том же узле, к которому физически подключено USB-устройство:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: linux-vm
   spec:
     # ... другие настройки ВМ ...
     usbDevices:
       - name: logitech-webcam
   EOF
   ```

### NodeUSBDevice

Ресурс [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) отражает состояние физического USB-устройства, обнаруженного на узле кластера. Это cluster-scoped ресурс, представляющий физическое USB-устройство на узле. Он создаётся автоматически системой DRA.

Пример просмотра всех обнаруженных USB-устройств:

```bash
d8 k get nodeusbdevice
```

Пример вывода:

```console
NAME                 NODE           READY   ASSIGNED   NAMESPACE   AGE
usb-flash-drive     node-1         True    False                  10m
logitech-webcam     node-2         True    True      my-project   15m
```

#### Условия NodeUSBDevice

Состояние ресурса [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) описывается набором условий, которые отражают готовность устройства и факт назначения неймспейса:

- **Ready**: Указывает, готово ли устройство к использованию.
  - `Ready` — устройство готово к использованию;
  - `NotReady` — устройство существует, но не готово;
  - `NotFound` — устройство отсутствует на хосте.

- **Assigned**: Указывает, назначен ли неймспейс устройству.
  - `Assigned` — неймспейс назначен и ресурс USBDevice создан;
  - `Available` — для устройства не назначен неймспейс;
  - `InProgress` — подключение устройства к неймспейсу выполняется.

#### Назначение неймспейса USB-устройству

Перед подключением USB-устройства к виртуальной машине его необходимо сделать доступным в конкретном неймспейсе. Для этого установите поле `.spec.assignedNamespace`:

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: NodeUSBDevice
metadata:
  name: logitech-webcam
spec:
  assignedNamespace: my-project
EOF
```

После назначения неймспейса соответствующий ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) автоматически создаётся в указанном неймспейсе.

### USBDevice

[USBDevice](/modules/virtualization/cr.html#usbdevice) — это namespace-scoped ресурс, представляющий USB-устройство, доступное для подключения к виртуальным машинам в заданном неймспейсе. Создаётся автоматически, когда [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) имеет назначенный неймспейс.

Пример просмотра USB-устройств в неймспейсе:

```bash
d8 k get usbdevice -n my-project
```

Пример вывода:

```console
NAME               NODE     MANUFACTURER   PRODUCT              SERIAL       ATTACHED   AGE
logitech-webcam    node-2   Logitech       Webcam C920         ABC123456   False      10m
```

#### Атрибуты USBDevice

Ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) содержит подробную информацию о физическом USB-устройстве в статусных полях. Эти атрибуты доступны в `.status.attributes`:

- `vendorID` — USB идентификатор производителя (шестнадцатеричный формат);
- `productID` — USB идентификатор продукта (шестнадцатеричный формат);
- `bus` — номер USB-шины;
- `deviceNumber` — номер USB-устройства на шине;
- `serial` — серийный номер устройства;
- `manufacturer` — название производителя устройства;
- `product` — название продукта устройства;
- `name` — имя устройства.

#### Условия USBDevice

Ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) содержит условия, отражающие готовность устройства и его состояние подключения:

- **Ready**: Указывает, готово ли устройство к использованию.
  - `Ready` — устройство готово к использованию;
  - `NotReady` — устройство существует, но не готово;
  - `NotFound` — устройство отсутствует на хосте.

- **Attached**: Указывает, подключено ли устройство к виртуальной машине.
  - `AttachedToVirtualMachine` — устройство подключено к ВМ;
  - `Available` — устройство доступно для подключения;
  - `NoFreeUSBIPPort` — устройство запрошено ВМ, но не может быть подключено, так как на целевом узле нет свободных USBIP-портов. В этом случае `Attached=False`.

### Подключение USB-устройства к ВМ

После появления ресурса [USBDevice](/modules/virtualization/cr.html#usbdevice) в неймспейсе его можно подключить к виртуальной машине. Для этого добавьте устройство в поле `.spec.usbDevices` ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine):

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  # ... другие настройки ВМ ...
  usbDevices:
    - name: logitech-webcam
EOF
```

После создания или обновления ВМ USB-устройство будет подключено к указанной виртуальной машине.

{% alert level="info" %}
Виртуальная машина должна быть размещена на том же узле, к которому физически подключено USB-устройство.
{% endalert %}

{% alert level="warning" %}
Во время миграции ВМ USB-устройство ненадолго отключится и подключится на новом узле в момент переключения ВМ. При сбое миграции устройство останется на старом узле.
{% endalert %}

### Просмотр информации об USB-устройстве

Для просмотра подробной информации об USB-устройстве:

```bash
d8 k describe nodeusbdevice <device-name>
```

Пример вывода:

```console
Name:         logitech-webcam
Namespace:
Labels:       <none>
Annotations:  <none>
API Version:  virtualization.deckhouse.io/v1alpha2
Kind:         NodeUSBDevice
Metadata:
  Creation Timestamp:  2024-01-15T10:30:00Z
  Generation:          1
  UID:                 abc123-def456-ghi789
Spec:
  Assigned Namespace:  my-project
Status:
  Node Name:           node-2
  Attributes:
    Bus:               1
    Device Number:     2
    Manufacturer:      Logitech
    Name:              Webcam C920
    Product:           Webcam C920
    Product ID:        082d
    Serial:            ABC123456
    Vendor ID:         046d
  Conditions:
    Type:              Ready
    Status:            True
    Reason:            Ready
    Message:           Device is ready to use
    Type:              Assigned
    Status:            True
    Reason:            Assigned
    Message:           Namespace is assigned for the device
  Observed Generation: 1
```

{% alert level="info" %}
Если USB-устройство физически отключено от узла, условие `Attached` принимает значение `False`.  
Статусы ресурсов `USBDevice` и `NodeUSBDevice` обновляются и указывают на отсутствие устройства на хосте.
{% endalert %}

### Требования и ограничения

При использовании проброса USB-устройств необходимо учитывать следующие требования и ограничения:

- Драйвер DRA должен быть установлен на узлах, где требуется обнаружение USB-устройств.
- USB-устройства пробрасываются на узел ВМ по сети с использованием USBIP. Виртуальная машина не обязана работать на том же узле, где физически подключено устройство. При подключении по сети действуют следующие ограничения по количеству устройств и выбору концентратора:
  - Узел может подключить не более 16 USB-устройств: до 8 на концентратор USB 2.0 и до 8 на концентратор USB 3.0.
  - Концентратор определяется скоростью устройства и не может быть выбран вручную. Устройство, работающее на USB 2.0, не может быть подключено к концентратору USB 3.0, и наоборот.
- USB-устройства поддерживают hot-plug — их можно подключать и отключать от работающей ВМ без её остановки.
- Для проброса USB-устройств требуются соответствующие модули ядра на узле.

## Экспорт данных

Экспортировать диски и снимки дисков виртуальных машин можно с помощью утилиты `d8` (версия 0.20.7 и выше). Для работы этой функции должен быть включен модуль [`storage-volume-data-manager`](/modules/storage-volume-data-manager/).

{% alert level="warning" %}
Диск не должен использоваться в момент экспорта. Если диск подключён к виртуальной машине, ВМ необходимо предварительно остановить.
{% endalert %}

Пример: экспорт диска (выполняется на узле кластера):

```bash
d8 data export download -n <namespace> vd/<virtual-disk-name> -o file.img
```

Пример: экспорт снимка диска (выполняется на узле кластера):

```bash
d8 data export download -n <namespace> vd/<virtual-disk-name> -o file.img
```

Если вы выполняете экспорт данных не с узла кластера (например, с вашей локальной машины), используйте флаг `--publish`.

{% alert level="warning" %}
Чтобы импортировать скачанный диск обратно в кластер, загрузите его как [образ](#загрузка-образа-из-командной-строки) или как [диск](#загрузка-диска-из-командной-строки).
{% endalert %}
