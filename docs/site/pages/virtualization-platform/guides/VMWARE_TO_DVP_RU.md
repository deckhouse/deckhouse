---
title: Перенос ВМ из VMware в DVP
permalink: ru/virtualization-platform/guides/vmware-to-dvp.html
description: Краткое руководство по переносу виртуальных машин из VMware (OVA/VMDK) на Deckhouse Virtualization Platform.
lang: ru
layout: sidebar-guides
---

Это руководство описывает перенос существующей виртуальной машины из VMware на Deckhouse Virtualization Platform (DVP). Исходными данными обычно служат экспорт в формате `OVA` или отдельные файлы `VMDK`. 

Последовательность действий можно свести к трём этапам:

1. На отдельной рабочей машине подготовить образ диска в формате `qcow2`;
1. Загрузить образ диска в кластер как ресурс `VirtualDisk`;
1. Создать `VirtualMachine`, которая загружается с этого диска.

Требования зависят от типа гостевой системы: для Linux достаточно пакетов из репозитория дистрибутива; для Windows дополнительно понадобится ISO `virtio-win`, чтобы после миграции гостевая ОС корректно работала с виртуальными устройствами в KVM.

Перед началом убедитесь, что у вас есть:

- доступ к кластеру DVP с установленным CLI Deckhouse (`d8`) и правами на создание ресурсов виртуализации в нужном неймспейсе;
- машина с Linux (или другая среда), где можно установить `virt-v2v` и `libguestfs`, и достаточно места на диске под распаковку `OVA` и каталог с результатом конвертации;
- файлы исходной ВМ (`OVA` или `VMDK`).

Подробнее о дисках и способах загрузки образа описано в разделе [«Диски»](/products/virtualization-platform/documentation/user/resource-management/disks.html).

## Установка инструментов

На этом шаге вы подготавливаете рабочую станцию для конвертации. Её не обязательно совмещать с узлом кластера DVP, достаточно любого Linux-хоста с доступом в интернет или локальным репозиторием пакетов.

Выберите команды для вашего дистрибутива.

Ubuntu/Debian:

```bash
sudo apt update
sudo apt install -y virt-v2v libguestfs-tools
```

RHEL/AlmaLinux:

```bash
sudo dnf install -y virt-v2v libguestfs-tools-c virtio-win
```

Для гостевых систем Windows при конвертации нужны драйверы VirtIO из дистрибутива `virtio-win`. В семействе RHEL/AlmaLinux пакет `virtio-win` ставится вместе с зависимостями; в `Debian/Ubuntu` ISO обычно [скачивают отдельно](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/), после чего указывают путь через переменную окружения:

```bash
export VIRTIO_WIN=/path/to/virtio-win.iso
```

{% alert level="warning" %}
Без корректного `virtio-win` для Windows-гостей конвертация может завершиться ошибкой либо после запуска ВМ в DVP гостевая ОС не увидит диски или сеть.
{% endalert %}

Затем перейдите к извлечению и преобразованию диска.

## Конвертация диска

Здесь вы превращаете данные VMware в один или несколько файлов формата `qcow2`, которые DVP сможет использовать как том виртуальной машины. Если у вас уже есть готовый `VMDK`, можно сразу перейти к подразделу про `virt-v2v`; если пришёл только `OVA`, сначала распакуйте архив.

### Распаковка OVA

Файл `OVA` — это обычный tar-архив с манифестом, описанием ВМ в формате `OVF` и одним или несколькими `VMDK`. Распаковка нужна, чтобы получить путь к файлу диска для `virt-v2v`.

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
У ВМ с несколькими дисками в архиве будет несколько файлов `*.vmdk`. Каждый диск конвертируйте отдельно командой `virt-v2v` и при необходимости создайте в DVP несколько ресурсов `VirtualDisk`, затем перечислите их в `VirtualMachine` в нужном порядке загрузки.
{% endalert %}

### Преобразование VMDK в qcow2 через virt-v2v

Утилита `virt-v2v` в режиме `-i disk` обрабатывает локальный `VMDK` и сохраняет результат в указанный каталог. Для гостевых систем Windows в образ добавляются драйверы из `virtio-win`, если задана переменная `VIRTIO_WIN`.

Конвертация Linux-гостя (без отдельного ISO, если гость не Windows):

```bash
virt-v2v -i disk ./machine-disk1.vmdk \
    -o local -os ./out -of qcow2
```

Для Windows-гостей укажите путь к `virtio-win.iso`:

```bash
VIRTIO_WIN=/path/to/virtio-win.iso virt-v2v -i disk ./machine-disk1.vmdk \
    -o local -os ./out -of qcow2
```

В каталоге `./out` появится файл вида `./out/machine.qcow2` (точное имя может совпадать с именем исходной ВМ из метаданных). Этот файл далее загружается в кластер.

Следующий раздел описывает, как передать образ `qcow2` в DVP через API Kubernetes.

## Загрузка образа диска в кластер

На этом этапе образ `qcow2` становится постоянным томом в кластере. В DVP для этого создаётся ресурс `VirtualDisk` с источником данных `Upload`.

### Выбор StorageClass

StorageClass в Kubernetes определяет, где и как будет создан том — по смыслу это ближе всего к `datastore` в VMware. От класса зависят производительность, тип репликации и политика расширения томов.

Посмотрите список доступных классов в вашем кластере:

```bash
d8 k get storageclass
```

Пример вывода:

```console
NAME                 PROVISIONER                             VOLUMEBINDINGMODE   AGE
rv-thin-r1 (default) replicated.csi.storage.deckhouse.io    Immediate           48d
rv-thin-r2           replicated.csi.storage.deckhouse.io    Immediate           48d
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

Строка URL содержит секретный фрагмент пути; не публикуйте её в открытых каналах.

### Загрузка образа

Передайте файл `qcow2` методом `PUT` на полученный адрес. Ниже пример для внешнего URL; подставьте свой адрес из статуса `VirtualDisk` и путь к файлу после конвертации.

```bash
curl https://virtualization.example.com/upload/<secret-url> \
    --progress-bar -T ./out/machine.qcow2 | cat
```

Дождитесь завершения загрузки без ошибок HTTP. После этого контроллер обработает образ и переведёт диск в фазу `Ready`.

### Проверка статуса

Убедитесь, что ресурс диска вышел в рабочее состояние и размер тома отображается ожидаемо:

```bash
d8 k get vd uploaded-disk
```

Пример вывода:

```console
NAMESPACE   NAME             PHASE   CAPACITY   AGE
default     uploaded-disk    Ready   10Gi       1m
```

Если фаза долго не меняется с `WaitForUserUpload` или ресурс перешёл в `Failed`, проверьте сообщения в `kubectl describe vd uploaded-disk` и события в неймспейсе.

Когда диск в статусе `Ready`, можно создавать виртуальную машину.

## Создание виртуальной машины

Последний шаг — описать запускаемую ВМ: сколько CPU и памяти нужно, к какой сети подключиться и какой диск считать загрузочным. Конфигурация VMware (`OVF`/`VMX`) напрямую не импортируется; параметры переносятся вручную по таблице соответствий ниже и с помощью примера YAML.

### Соответствие понятий VMware и DVP

Для администраторов, знакомых с vSphere, ниже приведено соответствие привычных объектов VMware ресурсам Kubernetes и виртуализации DVP.

| VMware                  | DVP                                          | Описание                         |
|-------------------------|----------------------------------------------|----------------------------------|
| Datastore               | StorageClass                                 | Хранилище для дисков             |
| VM Hardware Version     | VirtualMachineClass                          | Класс ВМ (CPU, память, политики) |
| VMX (конфиг ВМ)         | VirtualMachine.spec                          | Спецификация ВМ                  |
| Virtual Disk (VMDK)     | VirtualDisk                                  | Диск ВМ                          |
| ISO Image               | VirtualImage (`cdrom: true`)                 | ISO для установки или драйверов  |
| Template                | VirtualImage                                 | Шаблон для создания дисков       |
| Port Group / VLAN       | VirtualMachine (`networks`)                  | Сетевые настройки                |
| Resource Pool           | Project и квоты                              | Ограничения ресурсов на проект   |
| Snapshot                | VirtualDiskSnapshot / VirtualMachineSnapshot | Снимки диска и ВМ                |
| Folder                  | Namespace                                    | Неймспейс                        |
| Cluster / Resource Pool | Project                                      | Группировка неймспейсов          |
| ESXi Host               | Node                                         | Физический сервер                |
| vCenter                 | Kubernetes API                               | Управление кластером             |

Подробнее о подключении ВМ к сетям см. [Сети виртуальных машин](/products/virtualization-platform/documentation/admin/platform-management/network/vm-network.html).

### Пример VirtualMachine

Ресурс `VirtualMachine` ссылается на уже загруженный диск через `blockDeviceRefs`. Порядок элементов в `blockDeviceRefs` задаёт порядок загрузки: первым должен идти диск с загрузчиком ОС.

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

Дополнительные возможности (cloud-init, несколько дисков, классы виртуальных машин для продуктивной среды) описаны в разделе [«Виртуальные машины»](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).

### Основные параметры спецификации

Ниже перечислены поля, которые чаще всего нужно сверить после переноса с VMware.

| Параметр                      | Описание                                                      |
|-------------------------------|---------------------------------------------------------------|
| `virtualMachineClassName`     | Класс ВМ, например `generic`, `serverful`, `high-performance` |
| `osType`                      | Тип ОС: `Generic` (Linux и прочее) или `Windows`              |
| `cpu.cores`                   | Количество виртуальных CPU                                    |
| `memory.size`                 | Объём оперативной памяти                                      |
| `blockDeviceRefs`             | Диски и образы; порядок в списке задаёт приоритет загрузки    |
| `provisioning.type: UserData` | Передача cloud-init для первичной настройки гостя             |

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
