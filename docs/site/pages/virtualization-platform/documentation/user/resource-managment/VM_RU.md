---
title: "Виртуальные машины"
permalink: ru/virtualization-platform/documentation/user/resource-management/virtual-machines.html
lang: ru
---

Для создания виртуальной машины используется ресурс [VirtualMachine](../../../reference/cr/virtualmachine.html), его параметры позволяют сконфигурировать:

- [класс виртуальной машины](../../admin/platform-management/virtualization/virtual-machine-classes.html);
- ресурсы, требуемые для работы виртуальной машины (процессор, память, диски и образы);
- правила размещения виртуальной машины на узлах кластера;
- настройки загрузчика и оптимальные параметры для гостевой ОС;
- политику запуска виртуальной машины и политику применения изменений;
- сценарии начальной конфигурации (cloud-init);
- перечень блочных устройств.

## Создание виртуальной машины

Ниже представлен пример простой конфигурации виртуальной машины, запускающей ОС Ubuntu 22.04. В примере используется сценарий первичной инициализации виртуальной машины (cloud-init), который устанавливает гостевого агента `qemu-guest-agent` и сервис `nginx`, а также создает пользователя `cloud` с паролем `cloud`:

Пароль в примере был сгенерирован с использованием команды `mkpasswd --method=SHA-512 --rounds=4096 -S saltsalt` и при необходимости вы можете его поменять на свой:

1. Создайте виртуальную машину [с диском](./disks.html#создание-диска-из-образа):

    ```yaml
    d8 k apply -f - <<"EOF"
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualMachine
    metadata:
      name: linux-vm
    spec:
      # Название класса ВМ.
      virtualMachineClassName: host
      # Блок скриптов первичной инициализации ВМ.
      provisioning:
        type: UserData
        # Пример cloud-init-сценария для создания пользователя cloud с паролем cloud и установки сервиса агента qemu-guest-agent и сервиса nginx.
        userData: |
          #cloud-config
          package_update: true
          packages:
            - nginx
            - qemu-guest-agent
          run_cmd:
            - systemctl daemon-reload
            - systemctl enable --now nginx.service
            - systemctl enable --now qemu-guest-agent.service
          ssh_pwauth: True
          users:
          - name: cloud
            passwd: '$6$rounds=4096$saltsalt$fPmUsbjAuA7mnQNTajQM6ClhesyG0.yyQhvahas02ejfMAq1ykBo1RquzS0R6GgdIDlvS.kbUwDablGZKZcTP/'
            shell: /bin/bash
            sudo: ALL=(ALL) NOPASSWD:ALL
            lock_passwd: False
          final_message: "The system is finally up, after $UPTIME seconds"
      # Настройки ресурсов ВМ.
      cpu:
        # Количество ядер ЦП.
        cores: 1
        # Запросить 10% процессорного времени одного физического ядра.
        coreFraction: 10%
      memory:
        # Объем оперативной памяти.
        size: 1Gi
      # Список дисков и образов, используемых в ВМ.
      blockDeviceRefs:
        # Порядок дисков и образов в данном блоке определяет приоритет загрузки.
        - kind: VirtualDisk
          name: linux-vm-root
    EOF
    ```

    После создания ресурс `VirtualMachine` может находиться в следующих состояниях:

      - `Pending` - ожидание готовности всех зависимых ресурсов, требующихся для запуска виртуальной машины.
      - `Starting` - идет процесс запуска виртуальной машины.
      - `Running` - виртуальная машина запущена.
      - `Stopping` - идет процесс остановки виртуальной машины.
      - `Stopped` - виртуальная машина остановлена.
      - `Terminating` - виртуальная машина удаляется.
      - `Migrating` - виртуальная машина находится в состоянии онлайн-миграции на другой узел.

1. Проверьте состояние виртуальной машины после создания:

    ```shell
    d8 k get vm linux-vm
    ```

    Пример вывода:

    ```console
    NAME       PHASE     NODE           IPADDRESS     AGE
    linux-vm   Running   virtlab-pt-2   10.66.10.12   11m
    ```

    После создания виртуальная машина автоматически получит IP-адрес из диапазона, указанного в настройках (блок `virtualMachineCIDRs`).

## Подключение к виртуальной машине

Для подключения к виртуальной машине доступны следующие способы:

- протокол удаленного управления (например SSH), который должен быть предварительно настроен на виртуальной машине;
- серийная консоль (serial console);
- протокол VNC.

Пример подключения к виртуальной машине с использованием серийной консоли:

```shell
d8 v console linux-vm
```

Пример вывода:

```console
Successfully connected to linux-vm console. The escape sequence is ^]

linux-vm login: cloud
Password: cloud
```

Для завершения работы с серийной консолью нажмите `Ctrl+]`.

Пример команды для подключения по VNC:

```bash
d8 v vnc linux-vm
```

Пример команды для подключения по SSH.

```bash
d8 v ssh cloud@linux-vm --local-ssh
```

## Политика запуска и управление состоянием виртуальной машины

Политика запуска виртуальной машины предназначена для автоматизированного управления состоянием виртуальной машины. Определяется она в виде параметра `.spec.runPolicy` в спецификации виртуальной машины. Поддерживаются следующие политики:

- `AlwaysOnUnlessStoppedManually` — (по умолчанию) после создания ВМ всегда находится в рабочем состоянии. В случае сбоев работа ВМ восстанавливается автоматически. Остановка ВМ возможна только путем вызова команды `d8 v stop` или создания соответствующей операции.
- `AlwaysOn` — после создания ВМ всегда находится в работающем состоянии, даже в случае ее выключения средствами ОС. В случае сбоев работа ВМ восстанавливается автоматически.
- `Manual` — после создания состоянием ВМ управляет пользователь вручную с использованием команд или операций.
- `AlwaysOff` — после создания ВМ всегда находится в выключенном состоянии. Возможность включения ВМ через команды\операции - отсутствует.

Состоянием виртуальной машины можно управлять с помощью следующих методов:

- Создание ресурса [VirtualMachineOperation](../../../reference/cr/virtualmachineoperation.html) (`vmop`).
- Использование утилиты [`d8`](../../../reference/console-utilities/d8.html) с соответствующей подкомандой.

Ресурс `VirtualMachineOperation` декларативно определяет действие, которое должно быть выполнено на виртуальной машине.

Пример операции для выполнения перезагрузки виртуальной машины с именем `linux-vm`:

```yaml
d8 k create -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  generateName: restart-linux-vm-
spec:
  virtualMachineName: linux-vm
  # Тип применяемой операции = применяемая операция.
  type: Restart
EOF
```

Посмотреть результат действия можно с использованием команды:

```shell
d8 k get virtualmachineoperation
# или
d8 k get vmop
```

Аналогичное действие можно выполнить с использованием утилиты `d8`:

```shell
d8 v restart linux-vm
```

Возможные операции:

| d8             | vmop type | Действие                      |
| -------------- | --------- |-------------------------------|
| `d8 v stop`    | `Stop`    | Остановить ВМ                 |
| `d8 v start`   | `Start`   | Запустить ВМ                  |
| `d8 v restart` | `Restart` | Перезапустить ВМ              |
| `d8 v evict`   | `Evict`   | Мигрировать ВМ на другой узел |

## Изменение конфигурации виртуальной машины

Конфигурацию виртуальной машины можно изменять в любое время после создания ресурса `VirtualMachine`. Однако то, как эти изменения будут применены, зависит от текущей фазы виртуальной машины и характера внесённых изменений.

Изменения в конфигурацию виртуальной машины можно внести с использованием следующей команды:

```bash
d8 k edit vm linux-vm
```

Если виртуальная машина находится в выключенном состоянии (`.status.phase: Stopped`), внесённые изменения вступят в силу сразу после её запуска.

Если виртуальная машина работает (`.status.phase: Running`), то способ применения изменений зависит от их типа:

| Блок конфигурации                       | Как применяется         |
| --------------------------------------- | ----------------------- |
| `.metadata.labels`                      | Сразу                   |
| `.metadata.annotations`                 | Сразу                   |
| `.spec.runPolicy`                       | Сразу                   |
| `.spec.disruptions.restartApprovalMode` | Сразу                   |
| `.spec.*`                               | Требуется перезапуск ВМ |

Рассмотрим пример изменения конфигурации виртуальной машины:

Предположим, мы хотим изменить количество ядер процессора. В данный момент виртуальная машина запущена и использует одно ядро, что можно подтвердить, подключившись к ней через серийную консоль и выполнив команду `nproc`.

```shell
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
```

Пример вывода:

```txt
# 1
```

Примените следующий патч к виртуальной машине, чтобы изменить количество ядер с 1 на 2.

```shell
d8 k patch vm linux-vm --type merge -p '{"spec":{"cpu":{"cores":2}}}'
```

Пример вывода:

```console
virtualmachine.virtualization.deckhouse.io/linux-vm patched
```

Изменения в конфигурацию внесены, но ещё не применены к виртуальной машине. Проверьте это, повторно выполнив:

```shell
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
```

Пример вывода:

```txt
# 1
```

Для применения этого изменения необходим перезапуск виртуальной машины. Выполните следующую команду, чтобы увидеть изменения, ожидающие применения (требующие перезапуска):

```shell
d8 k get vm linux-vm -o jsonpath="{.status.restartAwaitingChanges}" | jq .
```

Пример вывода:

```console
[
  {
    "currentValue": 1,
    "desiredValue": 2,
    "operation": "replace",
    "path": "cpu.cores"
  }
]
```

Выполните команду:

```shell
d8 k get vm linux-vm -o wide
```

Пример вывода:

```console
NAME        PHASE     CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
linux-vm   Running   2       100%           1Gi      True           True    True         virtlab-pt-1   10.66.10.13   5m16s
```

В колонке `NEED RESTART` мы видим значение `True`, а это значит что для применения изменений требуется перезагрузка.

Выполните перезагрузку виртуальной машины:

```shell
d8 v restart linux-vm
```

После перезагрузки изменения будут применены и блок `.status.restartAwaitingChanges` будет пустой.

Выполните команду для проверки:

```shell
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
# 2
```

Порядок применения изменений виртуальной машины через «ручной» рестарт является поведением по умолчанию. Если есть необходимость применять внесенные изменения сразу и автоматически, для этого нужно изменить политику применения изменений:

```yaml
spec:
  disruptions:
    restartApprovalMode: Automatic
```

## Сценарии начальной инициализации

Сценарии начальной инициализации предназначены для первичной конфигурации виртуальной машины при её запуске.

В качестве сценариев начальной инициализации поддерживаются:

- [CloudInit](https://cloudinit.readthedocs.io).
- [Sysprep](https://learn.microsoft.com/ru-ru/windows-hardware/manufacture/desktop/sysprep--system-preparation--overview).

Сценарий CloudInit можно встраивать непосредственно в спецификацию ВМ, но этот сценарий ограничен максимальной длиной в 2048 байт:

```yaml
spec:
  provisioning:
    type: UserData
    userData: |
      #cloud-config
      package_update: true
      ...
```

При более длинных сценариях и/или наличии приватных данных, сценарий начальной инициализации виртуальной машины может быть создан в ресурсе Secret. Пример Secret со сценарием CloudInit приведен ниже:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloud-init-example
data:
  userData: <base64 data>
type: provisioning.virtualization.deckhouse.io/cloud-init
```

Фрагмент конфигурации виртуальной машины при использовании скрипта начальной инициализации CloudInit хранящегося в ресурсе Secret:

```yaml
spec:
  provisioning:
    type: UserDataRef
    userDataRef:
      kind: Secret
      name: cloud-init-example
```

Примечание: значение поля `.data.userData` должно быть закодировано в формате Base64.

Для конфигурирования виртуальных машин под управлением ОС Windows с использованием Sysprep, поддерживается только вариант с Secret.

Пример Secret с сценарием Sysprep:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sysprep-example
data:
  unattend.xml: <base64 data>
type: provisioning.virtualization.deckhouse.io/sysprep
```

Примечание: Значение поля `.data.unattend.xml` должно быть закодировано в формате Base64.

Фрагмент конфигурации виртуальной машины с использованием скрипта начальной инициализации Sysprep в ресурсе Secret:

```yaml
spec:
  provisioning:
    type: SysprepRef
    sysprepRef:
      kind: Secret
      name: sysprep-example
```

## Размещение ВМ по узлам

Для управления размещением виртуальных машин по узлам можно использовать следующие подходы:

- Простое связывание по меткам — `nodeSelector`;
- Предпочтительное связывание — `Affinity`;
- Избежание совместного размещения — `AntiAffinity`.

> Параметры размещения виртуальных машин можно изменить в реальном времени (доступно только в Enterprise-редакции). Однако, если новые параметры размещения не совпадают с текущими, виртуальная машина будет перемещена на узлы, соответствующие новым требованиям.

### Простое связывание по меткам — nodeSelector

`nodeSelector` — это простейший способ контролировать размещение виртуальных машин, используя набор меток. Он позволяет задать, на каких узлах могут запускаться виртуальные машины, выбирая узлы с необходимыми метками.

```yaml
spec:
  nodeSelector:
    disktype: ssd
```

![nodeSelector](/../../../../images/virtualization-platform/placement-nodeselector.ru.png)

В этом примере виртуальная машина будет размещена только на узлах, которые имеют метку `disktype` со значением `ssd`.

### Предпочтительное связывание — Affinity

`Affinity` предоставляет более гибкие и мощные инструменты по сравнению с `nodeSelector`. Он позволяет задавать «предпочтения» и «обязательности» для размещения виртуальных машин. `Affinity` поддерживает два вида: `nodeAffinity` и `virtualMachineAndPodAffinity`.

`nodeAffinity` позволяет определять, на каких узлах может быть запущена виртуальная машина, с помощью выражений меток, и может быть предпочтительным (`preferred`) или обязательным (`required`).

Пример использования `nodeAffinity`:

```yaml
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: disktype
                operator: In
                values:
                  - ssd
```

![nodeAffinity](/../../../../images/virtualization-platform/placement-node-affinity.ru.png)

В этом примере виртуальная машина будет размещена только на узлах, которые имеют метку `disktype` со значением `ssd`.

`virtualMachineAndPodAffinity` управляет размещением одних виртуальных машин относительно других виртуальных машин. Он позволяет задавать предпочтение размещения виртуальных машин на тех же узлах, где уже запущены определенные виртуальные машины.

Пример:

```yaml
spec:
  affinity:
    virtualMachineAndPodAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 1
          virtualMachineAndPodAffinityTerm:
            labelSelector:
              matchLabels:
                server: database
            topologyKey: "kubernetes.io/hostname"
```

![virtualMachineAndPodAffinity](/../../../../images/virtualization-platform/placement-vm-affinity.ru.png)

В этом примере виртуальная машина будет размещена, если будет такая возможность (так как используется метка `preferred`), только на узлах на которых присутствует виртуальная машина с меткой `server` и значением `database`.

### Избежание совместного размещения — AntiAffinity

`AntiAffinity` — это противоположность `Affinity`, которая позволяет задавать требования для избегания размещения виртуальных машин на одних и тех же узлах. Это полезно для распределения нагрузки или обеспечения отказоустойчивости.

Термины `Affinity` и `AntiAffinity` применимы только к отношению между виртуальными машинами. Для узлов используемые привязки называются `nodeAffinity`. В `nodeAffinity` нет отдельного обратного термина, как в случае с `virtualMachineAndPodAffinity`, но можно создать противоположные условия, задав отрицательные операторы в выражениях меток. Чтобы акцентировать внимание на исключении определенных узлов, можно воспользоваться `nodeAffinity` с оператором, таким как `NotIn`.

Пример использования `virtualMachineAndPodAntiAffinity`:

```yaml
spec:
  affinity:
    virtualMachineAndPodAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchLabels:
              server: database
          topologyKey: "kubernetes.io/hostname"
```

![AntiAffinity](/../../../../images/virtualization-platform/placement-vm-antiaffinity.ru.png)

В данном примере создаваемая виртуальная машина не будет размещена на одном узле с виртуальной машиной с меткой `server: database`.

## Статические и динамические блочные устройства

Блочные устройства можно разделить на два типа по способу их подключения: статические и динамические (hotplug).

### Статические блочные устройства

Блочные устройства и их особенности представлены в таблице:

| Тип блочного устройства | Комментарий                                                      |
| ----------------------- |------------------------------------------------------------------|
| `VirtualImage`          | подключается в режиме для чтения, или как cd-rom для iso-образов |
| `ClusterVirtualImage`   | подключается в режиме для чтения, или как cd-rom для iso-образов |
| `VirtualDisk`           | подключается в режиме для чтения и записи                        |

Статические блочные устройства указываются в спецификации виртуальной машины в блоке `.spec.blockDeviceRefs` в виде списка. Порядок устройств в этом списке определяет последовательность их загрузки. Таким образом, если диск или образ указан первым, загрузчик сначала попробует загрузиться с него. Если это не удастся, система перейдет к следующему устройству в списке и попытается загрузиться с него. И так далее до момента обнаружения первого загрузчика.

Изменение состава и порядка устройств в блоке `.spec.blockDeviceRefs` возможно только с перезагрузкой виртуальной машины.

Фрагмент конфигурации VirtualMachine со статически подключенными диском и проектным образом:

```yaml
spec:
  blockDeviceRefs:
    - kind: VirtualDisk
      name: <virtual-disk-name>
    - kind: VirtualImage
      name: <virtual-image-name>
```

### Динамические блочные устройства

Динамические блочные устройства можно подключать и отключать от виртуальной машины, находящейся в запущенном состоянии, без необходимости её перезагрузки.

Для подключения динамических блочных устройств используется ресурс [VirtualMachineBlockDeviceAttachment](../../../reference/cr/virtualmachineblockdeviceattachment.html) (`vmbda`). На данный момент для подключения в качестве динамического блочного устройства поддерживается только [VirtualDisk](../../../reference/cr/virtualdisk.html).

Создайте ресурс, который подключит пустой диск blank-disk к виртуальной машине linux-vm:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  name: attach-blank-disk
spec:
  blockDeviceRef:
    kind: VirtualDisk
    name: blank-disk
  virtualMachineName: linux-vm
EOF
```

После создания `VirtualMachineBlockDeviceAttachment` может находиться в следующих состояниях:

- `Pending` - ожидание готовности всех зависимых ресурсов.
- `InProgress` - идет процесс подключения устройства.
- `Attached` - устройство подключено.

Проверьте состояние вашего ресурса:

```shell
d8 k get vmbda attach-blank-disk
```

Пример вывода:

```console
NAME                PHASE      VIRTUAL MACHINE NAME   AGE
attach-blank-disk   Attached   linux-vm              3m7s
```

Подключитесь к виртуальной машине и удостоверьтесь, что диск подключен:

```shell
d8 v ssh cloud@linux-vm --local-ssh --command "lsblk"
```

Пример вывода:

```console
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINTS
sda       8:0    0   10G  0 disk <--- статично подключенный диск linux-vm-root
|-sda1    8:1    0  9.9G  0 part /
|-sda14   8:14   0    4M  0 part
`-sda15   8:15   0  106M  0 part /boot/efi
sdb       8:16   0    1M  0 disk <--- cloudinit
sdc       8:32   0 95.9M  0 disk <--- динамически подключенный диск blank-disk
```

Для отключения диска от виртуальной машины удалите ранее созданный ресурс:

```shell
d8 k delete vmbda attach-blank-disk
```

Подключение образов, осуществляется по аналогии. Для этого в качестве `kind` указать VirtualImage или ClusterVirtualImage и имя образа:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  name: attach-ubuntu-iso
spec:
  blockDeviceRef:
    kind: VirtualImage # Или ClusterVirtualImage.
    name: ubuntu-iso
  virtualMachineName: linux-vm
EOF
```

## Миграция виртуальной машины в реальном времени

Миграция виртуальных машин является важной функцией в управлении виртуализованной инфраструктурой. Она позволяет перемещать работающие виртуальные машины с одного физического узла на другой без их отключения.

Миграция может осуществляться автоматически при:

- Обновлении «прошивки» виртуальной машины.
- Перебалансировке нагрузки на узлах кластера.
- Переводе узлов в режим обслуживания для проведения работ.

Также миграция виртуальной машины может быть выполнена по требованию пользователя. Рассмотрим на примере:

Перед запуском миграции посмотрите текущий статус виртуальной машины:

```shell
d8 k get vm
```

Пример вывода:

```console
NAME       PHASE     NODE           IPADDRESS     AGE
linux-vm   Running   virtlab-pt-1   10.66.10.14   79m
```

Виртуальная машина запущена на узле `virtlab-pt-1`.

Для осуществления миграции виртуальной машины с одного узла на другой, с учетом требований к размещению виртуальной машины используется ресурс [VirtualMachineOperation](../../../reference/cr/virtualmachineoperation.html) (`vmop`) с типом Evict.

```yaml
d8 k create -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  generateName: evict-linux-vm-
spec:
  # Имя виртуальной машины.
  virtualMachineName: linux-vm
  # Операция для миграции.
  type: Evict
EOF
```

Сразу после создания ресурса `vmop`, выполните команду:

```shell
d8 k get vm -w
```

Пример вывода:

```console
NAME       PHASE       NODE           IPADDRESS     AGE
linux-vm   Running     virtlab-pt-1   10.66.10.14   79m
linux-vm   Migrating   virtlab-pt-1   10.66.10.14   79m
linux-vm   Migrating   virtlab-pt-1   10.66.10.14   79m
linux-vm   Running     virtlab-pt-2   10.66.10.14   79m
```

Также для выполнения миграции можно использовать команду:

```shell
d8 v evict <vm-name>
```

## Режим обслуживания

При выполнении работ на узлах с запущенными виртуальными машинами существует риск нарушения их работоспособности. Чтобы этого избежать, узел можно перевести в режим обслуживания и мигрировать виртуальные машины на другие свободные узлы.
Для этого необходимо выполнить следующую команду:

```bash
d8 k drain <nodename> --ignore-daemonsets --delete-emptydir-dat
```

где `<nodename>` - узел, на котором предполагается выполнить работы и который должен быть освобожден от всех ресурсов (в том числе и от системных).

Если есть необходимость вытеснить с узла только виртуальные машины, выполните следующую команду:

```bash
d8 k drain <nodename> --pod-selector vm.kubevirt.internal.virtualization.deckhouse.io/name --delete-emptydir-data
```

После выполнения команд `d8 k drain` - узел перейдет в режим обслуживания и виртуальные машины на нем запускаться не смогут. Чтобы вывести его из режима обслуживания выполните следующую команду:

```bash
d8 k uncordon <nodename>
```

![Режим обслуживания](/../../../../images/virtualization-platform/drain.ru.png)

## IP-адреса виртуальных машин

Блок `.spec.settings.virtualMachineCIDRs` в конфигурации задает список подсетей для назначения IP-адресов виртуальным машинам (общий пул IP-адресов). Все адреса в этих подсетях доступны для использования, за исключением первого (адрес сети) и последнего (широковещательный адрес).

Ресурс [VirtualMachineIPAddressLease](../../../reference/cr/virtualmachineipaddresslease.html) (`vmipl`): кластерный ресурс, который управляет временным выделением IP-адресов из общего пула, указанного в `virtualMachineCIDRs`.

Чтобы посмотреть список временно выделенных IP-адресов (`vmipl`), используйте команду:

```shell
d8 k get vmipl
```

Пример вывода:

```console
NAME             VIRTUALMACHINEIPADDRESS                             STATUS   AGE
ip-10-66-10-14   {"name":"linux-vm-7prpx","namespace":"default"}     Bound    12h
```

Ресурс [VirtualMachineIPAddress](../../../reference/cr/virtualmachineipaddress.html) (`vmip`) — это ресурс проекта или пространства имен, который отвечает за резервирование выделенных IP-адресов и их привязку к виртуальным машинам. IP-адреса могут выделяться автоматически или по запросу.

Проверить назначенный IP-адрес можно с помощью команды:

```shell
d8 k get vmip
```

Пример вывода:

```console
NAME             ADDRESS       STATUS     VM         AGE
linux-vm-7prpx   10.66.10.14   Attached   linux-vm   12h
```

Алгоритм автоматического присвоения IP-адреса виртуальной машине выглядит следующим образом:

- Пользователь создает виртуальную машину с именем `<vmname>`.
- Контроллер автоматически создает ресурс `vmip` с именем `<vmname>-<hash>`, чтобы запросить IP-адрес и связать его с виртуальной машиной.
- Для этого `vmip` создается ресурс аренды `vmipl`, который выбирает случайный IP-адрес из общего пула.
- Как только ресурс `vmip` создан, виртуальная машина получает назначенный IP-адрес.

По умолчанию IP-адрес для виртуальной машины назначается автоматически, из подсетей, и закрепляется за ней до её удаления. После удаления виртуальной машины ресурс `vmip` также удаляется, но IP-адрес временно остается закрепленным за проектом/пространством имен и может быть повторно запрошен.

## Запрос требуемого IP-адреса

Создайте ресурс `vmip`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineIPAddress
metadata:
  name: linux-vm-custom-ip
spec:
  staticIP: 10.66.20.77
  type: Static
EOF
```

Создайте новую или измените существующую виртуальную машину и в спецификации укажите требуемый ресурс `vmip` явно:

```yaml
spec:
  virtualMachineIPAddressName: linux-vm-custom-ip
```

## Сохранение IP-адреса, присвоенного виртуальной машине

Чтобы автоматически выданный IP-адрес виртуальной машины не удалился вместе с самой виртуальной машиной, выполните следующие действия.

Получите название ресурса `vmip` для заданной виртуальной машины:

```shell
d8 k get vm linux-vm -o jsonpath="{.status.virtualMachineIPAddressName}"
```

Пример вывода:

```console
linux-vm-7prpx
```

Удалите блоки `.metadata.ownerReferences` из найденного ресурса:

```shell
d8 k patch vmip linux-vm-7prpx --type=merge --patch '{"metadata":{"ownerReferences":null}}'
```

После удаления виртуальной машины, ресурс `vmip` сохранится и его можно будет использовать во вновь созданной виртуальной машине:

```yaml
spec:
  virtualMachineIPAddressName: linux-vm-7prpx
```

Даже если ресурс `vmip` будет удален, он остается арендованным для текущего проекта/пространства имен еще 10 минут и существует возможность вновь его занять по запросу:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineIPAddress
metadata:
  name: linux-vm-custom-ip
spec:
  staticIP: 10.66.20.77
  type: Static
EOF
```
