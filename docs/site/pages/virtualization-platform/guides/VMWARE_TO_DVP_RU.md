---
title: Перенос ВМ из VMware в DVP
permalink: ru/virtualization-platform/guides/vmware-to-dvp.html
description: Краткое руководство по переносу виртуальных машин из VMware (OVA/VMDK) на Deckhouse Virtualization Platform.
lang: ru
layout: sidebar-guides
---

Это руководство описывает перенос существующей виртуальной машины из VMware на Deckhouse Virtualization Platform (DVP).

Для переноса могут использоваться:

- дистрибутив виртуальной машины (файл `OVA` — архив, содержащий файлы дисков в формате `vmdk`, метаданные ВМ в формате `ovf` и контрольные суммы в формате `mf`);
- отдельные файлы дисков `VMDK`.

## Этапы переноса

Перенос ВМ из VMware в DVP включает в себя следующие этапы:

1. [Установка необходимых инструментов](#установка-инструментов).
1. [Подготовка образа диска в формате `qcow2`](#конвертация-диска) (конвертация из `VMDK` в `qcow2`).
1. [Загрузка образа диска в кластер](#загрузка-образа-диска-в-кластер) в виде ресурса [VirtualDisk](/modules/virtualization/cr.html#virtualdisk).
1. [Создание виртуальной машины](#создание-виртуальной-машины) (ресурс [VirtualMachine](/modules/virtualization/cr.html#virtualmachine)), которая загружается с этого диска.

## Что необходимо для конвертации

Перед началом переноса убедитесь, что у вас есть:

- доступ к кластеру DVP с установленной утилитой Deckhouse CLI (`d8`) и правами на создание ресурсов виртуализации в нужном неймспейсе;
- машина с Linux (или другая среда), где можно установить `virt-v2v` и `libguestfs`, и достаточно места на диске под распаковку дистрибутива виртуальной машины `OVA` (либо для отдельных файлов дисков) и для размещения директории с результатом конвертации;
- файлы исходной ВМ (`OVA` или `VMDK`).

Подробнее о дисках и способах загрузки образа описано в разделе [«Диски»](/products/virtualization-platform/documentation/user/resource-management/disks.html).

## Установка инструментов

Перечень необходимых инструментов и ресурсов зависит от типа системы, установленной в ВМ (гостевой системы), которая переносится из VMware в DVP:

- для Linux достаточно пакетов из репозитория дистрибутива;
- для Windows дополнительно понадобится ISO `virtio-win`, чтобы после миграции гостевая ОС корректно работала с виртуальными устройствами в KVM.

На этом шаге вы подготавливаете рабочую станцию для конвертации. Это не обязательно должен быть узел кластера DVP, достаточно любого Linux-хоста с доступом в интернет или с локальным репозиторием пакетов.

Установите инструменты на рабочую станцию для конвертации:

{% tabs os %}
{% tab "Рабочая станция с Ubuntu/Debian" %}

Выполните следующую команду:

```bash
sudo apt update
sudo apt install -y virt-v2v libguestfs-tools
```

Если в переносимой ВМ используется ОС Windows:

1. [Скачайте драйверы VirtIO](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/) из дистрибутива `virtio-win`.

1. Укажите путь к драйверам VirtIO через переменную окружения:

   ```bash
   export VIRTIO_WIN=/path/to/virtio-win.iso
   ```

{% alert level="warning" %}
Без корректного `virtio-win` для гостевой системы Windows конвертация может завершиться ошибкой либо после запуска ВМ в DVP гостевая ОС не увидит диски или сеть.
{% endalert %}

{% endtab %}
{% tab "Рабочая станция с RHEL/AlmaLinux" %}
Выполните команду:

```bash
sudo dnf install -y virt-v2v libguestfs-tools-c virtio-win
```

{% endtab %}
{% endtabs %}

Затем перейдите к конвертации диска.

## Конвертация диска

На этом шаге вы конвертируете данные VMware в один или несколько файлов формата `qcow2`, которые DVP сможет использовать как том виртуальной машины. Если у вас уже есть готовый `VMDK`, можно сразу перейти к подразделу [«Конвертация VMDK в qcow2 через virt-v2v»](#конвертация-vmdk-в-qcow2-через-virt-v2v). Если используется дистрибутив виртуальной машины `OVA`, сначала распакуйте его.

### Распаковка OVA

Файл `OVA` — это обычный tar-архив с манифестом, описанием ВМ в формате `OVF` и одним или несколькими `VMDK`. Распаковка нужна, чтобы получить путь к файлу диска для утилиты `virt-v2v`.

Распакуйте архив целиком, если хотите сверить контрольные суммы или посмотреть OVF:

```bash
tar -xvf machine.ova
```

Типичное содержимое:

```text
machine.ova
├── machine.mf          # контрольные суммы (SHA256)
├── machine.ovf         # метаданные ВМ (CPU, RAM, диски, сети)
└── machine-disk1.vmdk  # образ диска
```

Если архив большой, достаточно извлечь только нужный `VMDK` по имени из `OVF`:

```bash
tar -xvf machine.ova machine-disk1.vmdk
```

{% alert level="info" %}
У ВМ с несколькими дисками в архиве будет несколько файлов `*.vmdk`. Каждый диск конвертируйте отдельно с помощью утилиты `virt-v2v` и при необходимости создайте в DVP несколько ресурсов `VirtualDisk`, затем перечислите их в `VirtualMachine` в нужном порядке загрузки.
{% endalert %}

### Конвертация VMDK в qcow2 через virt-v2v

Утилита `virt-v2v` в режиме `-i disk` обрабатывает локальный `VMDK` и сохраняет результат в указанную директорию. Для конвертации выполните команду:

{% tabs os_convert %}
{% tab "Для гостевой ОС Linux" %}

```bash
virt-v2v -i disk ./machine-disk1.vmdk \
    -o local -os ./out -of qcow2
```

{% endtab %}
{% tab "Для гостевой ОС Windows" %}

Для конвертации `VMDK` для гостевой системы Windows укажите в команде путь к `virtio-win.iso`:

```bash
VIRTIO_WIN=/path/to/virtio-win.iso virt-v2v -i disk ./machine-disk1.vmdk \
    -o local -os ./out -of qcow2
```

{% endtab %}
{% endtabs %}

После конвертации в директории `./out` появится файл вида `./out/machine.qcow2` (точное имя может совпадать с именем исходной ВМ из метаданных). Этот файл далее загружается в кластер.

## Загрузка образа диска в кластер

Этот подраздел описывает, как передать образ `qcow2` в DVP через API Kubernetes.

На этом этапе образ `qcow2` становится постоянным томом в кластере. В DVP для этого создаётся ресурс `VirtualDisk` с источником данных `Upload`.

Загрузка образа диска в кластер включает следующие шаги:

1. Выбор StorageClass.
1. Создание VirtualDisk для загрузки.
1. Получение URL для загрузки.
1. Загрузка образа.
1. Проверка статуса загруженного образа.

### Выбор StorageClass

StorageClass в Kubernetes определяет, где и как будет создан том — по смыслу это ближе всего к `datastore` в VMware. От класса зависят производительность, тип репликации и политика расширения томов.

Посмотрите список доступных классов в вашем кластере:

```bash
d8 k get storageclass
```

Пример вывода:

```console
NAME                 PROVISIONER                             VOLUMEBINDINGMODE   AGE
rv-thin-r1 (default) replicated.csi.storage.deckhouse.io     Immediate           48d
rv-thin-r2           replicated.csi.storage.deckhouse.io     Immediate           48d
```

Запомните имя класса, который подходит под ваши требования к хранилищу для дисков ВМ.

### Создание VirtualDisk для загрузки

Создайте ресурс диска, указав StorageClass и размер тома. Значение `spec.persistentVolumeClaim.size` должно быть не меньше фактического размера загружаемого `qcow2` (при сомнениях возьмите запас — пустой том можно будет точнее подобрать при повторном создании ресурса).

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: uploaded-disk
spec:
  persistentVolumeClaim:
    storageClassName: rv-thin-r1
    size: 10Gi
  dataSource:
    type: Upload
EOF
```

После создания ресурс перейдёт в фазу `WaitForUserUpload` — это означает, что том выделен и можно начинать передачу файла.

### Получение URL для загрузки

Платформа формирует два URL: внутренний (`imageUploadURLs.inCluster`) и внешний (`imageUploadURLs.external`). Используйте тот адрес, который доступен из вашей сети (изнутри кластера или с рабочей станции администратора).

Внутренний URL (используйте, если загрузка выполняется с узла кластера или из пода):

```bash
d8 k get vd uploaded-disk -o jsonpath="{.status.imageUploadURLs.inCluster}"
```

Внешний URL (используйте с рабочей станции администратора при настроенном доступе к DVP):

```bash
d8 k get vd uploaded-disk -o jsonpath="{.status.imageUploadURLs.external}"
```

Просмотреть оба значения одной командой (потребуется установленный `jq`):

```bash
d8 k get vd uploaded-disk -o jsonpath="{.status.imageUploadURLs}" | jq
```

{% alert level="warning" %}
Строка URL содержит секретный фрагмент пути. Не публикуйте её в открытых каналах.
{% endalert %}

### Загрузка образа

Передайте файл `qcow2` методом `PUT` на полученный на предыдущем шаге адрес. Ниже пример для внешнего URL. Подставьте свой адрес из статуса `VirtualDisk` и путь к файлу после конвертации.

```bash
curl https://virtualization.example.com/upload/<secret-url> \
    --progress-bar -T ./out/machine.qcow2 | cat
```

Дождитесь завершения загрузки без ошибок HTTP. После этого контроллер обработает образ и переведёт диск в фазу `Ready`.

### Проверка статуса

Убедитесь, что ресурс диска вышел в рабочее состояние и размер тома соответствует ожидаемому:

```bash
d8 k get vd uploaded-disk
```

Пример вывода:

```console
NAMESPACE   NAME             PHASE   CAPACITY   AGE
default     uploaded-disk    Ready   10Gi       1m
```

Если фаза долго не меняется с `WaitForUserUpload` или ресурс перешёл в `Failed`, проверьте сообщения в `kubectl describe vd uploaded-disk` и события в соответствующем неймспейсе.

Когда диск в статусе `Ready`, можно создавать виртуальную машину.

## Создание виртуальной машины

Последний шаг — описание запускаемой ВМ: сколько CPU и памяти нужно, к какой сети подключиться и какой диск считать загрузочным. Конфигурация VMware (`OVF`/`VMX`) напрямую не импортируется. Перенесите параметры вручную, используя таблицу соответствий ниже и пример YAML-манифеста виртуальной машины (ресурс VirtualMachine).

### Соответствие понятий VMware и DVP

Для администраторов, знакомых с vSphere, ниже приведено соответствие привычных объектов VMware ресурсам Kubernetes и виртуализации DVP.

| VMware                  | DVP                                                                                                                                                           | Описание                         |
|-------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------|
| Datastore               | StorageClass                                                                                                                                                  | Хранилище для дисков             |
| VM Hardware Version     | [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass)                                                                                    | Класс ВМ (CPU, память, политики) |
| VMX (конфиг ВМ)         | [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).spec                                                                                         | Спецификация ВМ                  |
| Virtual Disk (VMDK)     | [VirtualDisk](/modules/virtualization/cr.html#virtualdisk)                                                                                                    | Диск ВМ                          |
| ISO Image               | [VirtualImage](/modules/virtualization/cr.html#virtualimage) (`cdrom: true`)                                                                                  | ISO для установки или драйверов  |
| Template                | [VirtualImage](/modules/virtualization/cr.html#virtualimage)                                                                                                  | Шаблон для создания дисков       |
| Port Group / VLAN       | [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) (`networks`)                                                                                 | Сетевые настройки                |
| Resource Pool           | Project и квоты                                                                                                                                               | Ограничения ресурсов на проект   |
| Snapshot                | [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) / [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot) | Снимки диска и ВМ                |
| Folder                  | Namespace                                                                                                                                                     | Неймспейс                        |
| Cluster / Resource Pool | Project                                                                                                                                                       | Группировка неймспейсов          |
| ESXi Host               | Node                                                                                                                                                          | Физический сервер                |
| vCenter                 | Kubernetes API                                                                                                                                                | Управление кластером             |

Подробнее о подключении ВМ к сетям см. [«Сети виртуальных машин»](/products/virtualization-platform/documentation/admin/platform-management/network/vm-network.html).

### Пример VirtualMachine

Ресурс VirtualMachine ссылается на уже загруженный диск через `blockDeviceRefs`. Порядок элементов в `blockDeviceRefs` задаёт порядок загрузки: первым должен идти диск с загрузчиком ОС.

Минимальный пример для Linux после миграции диска:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: my-vm
spec:
  virtualMachineClassName: generic
  osType: Generic
  cpu:
    cores: 2
  memory:
    size: 4Gi
  networks:
    - type: Main
  blockDeviceRefs:
    - kind: VirtualDisk
      name: uploaded-disk
EOF
```

Для Windows укажите `osType: Windows`.

Если в кластере настроены дополнительные сети и модуль SDN, к основной сети можно добавить интерфейсы:

```yaml
  networks:
    - type: Main
    - type: Network
      name: user-net
```

Дополнительные возможности (cloud-init, несколько дисков, классы виртуальных машин для production-окружения) описаны в разделе [«Виртуальные машины»](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).

#### Основные параметры спецификации

Ниже перечислены поля, которые чаще всего нужно сверить после переноса с VMware.

| Параметр                      | Описание                                                       |
|-------------------------------|----------------------------------------------------------------|
| `virtualMachineClassName`     | Класс ВМ, например, `generic`, `serverful`, `high-performance` |
| `osType`                      | Тип ОС: `Generic` (Linux и прочее) или `Windows`               |
| `cpu.cores`                   | Количество виртуальных CPU                                     |
| `memory.size`                 | Объём оперативной памяти                                       |
| `blockDeviceRefs`             | Диски и образы; порядок в списке задаёт приоритет загрузки     |
| `provisioning.type: UserData` | Передача cloud-init для первичной настройки гостевой ОС        |

### Проверка статуса ВМ

После применения манифеста дождитесь, пока ВМ запустится и получит адрес (если настроена основная сеть с выдачей IP из `virtualMachineCIDRs`):

```bash
d8 k get vm my-vm
```

Пример вывода:

```console
NAME    PHASE     NODE           IPADDRESS     AGE
my-vm   Running   virtlab-pt-2   10.66.10.12   2m
```

При фазе `Pending` или ошибках запуска используйте `d8 k describe vm my-vm` и журналы компонентов виртуализации в кластере.

### Подключение к ВМ

Выберите способ доступа в зависимости от того, настроена ли гостевая ОС на приём SSH, нужен ли графический вход или достаточно последовательной консоли для диагностики загрузки.

| Способ                   | Назначение                           | Команда                            |
|--------------------------|--------------------------------------|------------------------------------|
| Последовательная консоль | Нужен вывод загрузчика и ядра        | `d8 v console my-vm`               |
| VNC                      | Графическая консоль без SSH          | `d8 v vnc my-vm`                   |
| SSH                      | Удалённый вход по сети к гостю       | `d8 v ssh cloud@my-vm --local-ssh` |

Имя пользователя для SSH задаётся в гостевой системе; в примерах из документации DVP после cloud-init часто создаётся пользователь `cloud`.
