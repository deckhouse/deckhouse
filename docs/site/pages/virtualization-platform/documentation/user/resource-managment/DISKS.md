---
title: "Disks"
permalink: en/virtualization-platform/documentation/user/resource-managment/disks.html
---

Для создания виртуальной машины используется ресурс `VirtualMachine`, его параметры позволяют сконфигурировать:

- [класс виртуальной машины](ADMIN_GUIDE_RU.md#классы-виртуальных-машин)
- ресурсы, требуемые для работы виртуальной машины (процессор, память, диски и образы);
- правила размещения виртуальной машины на узлах кластера;
- настройки загрузчика и оптимальные параметры для гостевой ОС;
- политику запуска виртуальной машины и политику применения изменений;
- сценарии начальной конфигурации (cloud-init);
- перечень блочных устройств.

С полным описанием параметров конфигурации виртуальных машин можно ознакомиться по [ссылке](cr.html#virtualmachine)

### Создание виртуальной машины

Ниже представлен пример простой конфигурации виртуальной машины, запускающей ОС Ubuntu 22.04. В примере используется сценарий первичной инициализации виртуальной машины (cloud-init), который устанавливает гостевого агента `qemu-guest-agent` и сервис `nginx`, а также создает пользователя `cloud` с паролем `cloud`:

Пароль в примере был сгенерирован с использованием команды `mkpasswd --method=SHA-512 --rounds=4096 -S saltsalt` и при необходимости вы можете его поменять на свой:

Создайте виртуальную машину с диском созданным [ранее](#создание-диска-из-образа):

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
        - systemctl daemon-relaod
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

После создания `VirtualMachine` может находиться в следующих состояниях (фазах):

- `Pending` - ожидание готовности всех зависимых ресурсов, требующихся для запуска виртуальной машины.
- `Starting` - идет процесс запуска виртуальной машины.
- `Running` - виртуальная машина запущена.
- `Stopping` - идет процесс остановки виртуальной машины.
- `Stopped` - виртуальная машина остановлена.
- `Terminating` - виртуальная машина удаляется.
- `Migrating` - виртуальная машина находится в состоянии живой миграции на другой узел.

Проверьте состояние виртуальной машины после создания:

```bash
d8 k get vm linux-vm

# NAME        PHASE     NODE           IPADDRESS     AGE
# linux-vm   Running   virtlab-pt-2   10.66.10.12   11m
```

После создания виртуальная машина автоматически получит IP-адрес из диапазона, указанного в настройках модуля (блок `virtualMachineCIDRs`).

### Подключение к виртуальной машине

Для подключения к виртуальной машине доступны следующие способы:

- протокол удаленного управления (например SSH), который должен быть предварительно настроен на виртуальной машине.
- серийная консоль (serial console)
- протокол VNC

Пример подключения к виртуальной машине с использованием серийной консоли:

```bash
d8 v console linux-vm

# Successfully connected to linux-vm console. The escape sequence is ^]

linux-vm login: cloud
Password: cloud
```

Нажмите `Ctrl+]` для завершения работы с серийной консолью.

Пример команды для подключения по VNC:

```bash
d8 v vnc linux-vm
```

Пример команды для подключения по SSH.

```bash
d8 v ssh cloud@linux-vm --local-ssh
```

### Политика запуска и управление состоянием виртуальной машины

Политика запуска виртуальной машины предназначена для автоматизированного управления состоянием виртуальной машины. Определяется она в виде параметра `.spec.runPolicy` в спецификации виртуальной машины. Поддерживается следующие политики:

- `AlwaysOnUnlessStoppedManually` - (по умолчанию) после создания ВМ всегда находится в запущенном состоянии. В случае сбоев работа ВМ восстанавливается автоматически. Остановка ВМ возможно только путем вызова команды `d8 v stop` или создания соответствующей операции.
- `AlwaysOn` - после создания ВМ всегда находится в работающем состоянии, даже в случае ее выключения средствами ОС. В случае сбоев работа ВМ восстанавливается автоматически.
- `Manual` - после создания состоянием ВМ управляет пользователь вручную с использованием команд или операций.
- `AlwaysOff` - после создания ВМ всегда находится в выключенном состоянии. Возможность включения ВМ через команды\операции - отсутствует.

Состоянием виртуальной машины можно управлять с помощью следующих методов:

Создание ресурса `VirtualMachineOperation` (`vmop`).
Использование утилиты `d8` с соответствующей подкомандой.

Ресурс `VirtualMachineOperation` декларативно определяет императивное действие, которое должно быть выполнено на виртуальной машине. Это действие применяется к виртуальной машине сразу после её создания соответствующего `vmop`. Действие применяется к виртуальной машине один раз.

Пример операции для выполнения перезагрузки виртуальной машины с именем `linux-vm`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: restart-linux-vm-$(date +%s)
spec:
  virtualMachineName: linux-vm
  # Тип применяемой операции = применяемая операция.
  type: Restart
EOF
```

Посмотреть результат действия можно с использованием команды:

```bash
d8 k get virtualmachineoperation
# или
d8 k get vmop
```

Аналогичное действие можно выполнить с использованием утилиты `d8`:

```bash
d8 v restart  linux-vm
```

Перечень возможных операций приведен в таблице ниже:

| d8             | vmop type | Действие                      |
| -------------- | --------- | ----------------------------- |
| `d8 v stop`    | `stop`    | Остановить ВМ                 |
| `d8 v start`   | `start`   | Запустить ВМ                  |
| `d8 v restart` | `restart` | Перезапустить ВМ              |
| `d8 v migrate` | `migrate` | Мигрировать ВМ на другой узел |

### Изменение конфигурации виртуальной машины

Конфигурацию виртуальной машины можно изменять в любое время после создания ресурса `VirtualMachine`. Однако, то, как эти изменения будут применены, зависит от текущей фазы виртуальной машины и характера внесённых изменений.

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

```bash
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
# 1
```

Примените следующий патч к виртуальной машине, чтобы изменить количество ядер с 1 на 2.

```bash
d8 k patch vm linux-vm --type merge -p '{"spec":{"cpu":{"cores":2}}}'
# virtualmachine.virtualization.deckhouse.io/linux-vm patched
```

Изменения в конфигурации внесены, но ещё не применены к виртуальной машине. Проверьте это, повторно выполнив:

```bash
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
# 1
```

Для применения этого изменения необходим перезапуск виртуальной машины. Выполните следующую команду, чтобы увидеть изменения, ожидающие применения (требующие перезапуска):

```bash
d8 k get vm linux-vm -o jsonpath="{.status.restartAwaitingChanges}" | jq .

# [
#   {
#     "currentValue": 1,
#     "desiredValue": 2,
#     "operation": "replace",
#     "path": "cpu.cores"
#   }
# ]
```

Выполните команду:

```bash
d8 k get vm linux-vm -o wide

# NAME        PHASE     CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
# linux-vm   Running   2       100%           1Gi      True           True    True         virtlab-pt-1   10.66.10.13   5m16s
```

В колонке `NEED RESTART` мы видим значение `True`, а это значит что для применения изменений требуется перезагрузка.

Выполним перезагрузку виртуальной машины:

```bash
d8 v restart linux-vm
```

После перезагрузки изменения будут применены и блок `.status.restartAwaitingChanges` будет пустой.

Выполните команду для проверки:

```bash
d8 v ssh cloud@linux-vm --local-ssh --command "nproc"
# 2
```

Порядок применения изменений виртуальной машины через "ручной" рестарт является поведением по умолчанию. Если есть необходимость применять внесенные изменения сразу и автоматически, для этого нужно изменит политику применения изменений:

```yaml
spec:
  disruptions:
    restartApprovalMode: Automatic
```

### Сценарии начальной инициализации

Сценарии начальной инициализации предназначены для первичной конфигурации виртуальной машины при её запуске.

В качестве сценариев начальной инициализации поддерживаются:

- [CloudInit](https://cloudinit.readthedocs.io)
- [Sysprep](https://learn.microsoft.com/ru-ru/windows-hardware/manufacture/desktop/sysprep--system-preparation--overview).

Сценарий CloudInit можно встраивать непосредственно в спецификацию виртуальной машины, но этот сценарий ограничен максимальной длиной в 2048 байт:

```yaml
spec:
  provisioning:
    type: UserData
    userData: |
      #cloud-config
      package_update: true
      ...
```

При более длинных сценариях и/или наличия приватных данных, сценарий начальной инициализации виртуальной машины может быть создан в Secret'е. Пример Secret'а со сценарием CloudInit приведен ниже:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloud-init-example
data:
  userData: <base64 data>
type: provisioning.virtualization.deckhouse.io/cloud-init
```

фрагмент конфигурации виртуальной машины с при использовании скрипта начальной инициализации CloudInit хранящегося в Secret'е:

```yaml
spec:
  provisioning:
    type: UserDataRef
    userDataRef:
      kind: Secret
      name: cloud-init-example
```

Примечание: Значение поля `.data.userData` должно быть закодировано в формате Base64.

Для конфигурирования виртуальных машин под управлением ОС Windows с использованием Sysprep, поддерживается только вариант с Secret.

Пример Secret с сценарием Sysprep приведен ниже:

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

фрагмент конфигурации виртуальной машины с использованием скрипта начальной инициализации Sysprep в Secret'е:

```yaml
spec:
  provisioning:
    type: SysprepRef
    sysprepRef:
      kind: Secret
      name: sysprep-example
```

### Размещение ВМ по узлам

Для управления размещением виртуальных машин по узлам можно использовать следующие подходы:

- Простое связывание по меткам (`nodeSelector`)
- Предпочтительное связывание (`Affinity`)
- Избежание совместного размещения (`AntiAffinity`)

#### Простое связывание по меткам (nodeSelector)

`nodeSelector` — это простейший способ контролировать размещение виртуальных машин, используя набор меток. Он позволяет задать, на каких узлах могут запускаться виртуальные машины, выбирая узлы с необходимыми метками.

```yaml
spec:
  nodeSelector:
    disktype: ssd
```

![](images/placement-node-affinity.ru.png)

В этом примере виртуальная машина будет размещена только на узлах, которые имеют метку `disktype` со значением `ssd`.

#### Предпочтительное связывание (Affinity)

`Affinity` предоставляет более гибкие и мощные инструменты по сравнению с `nodeSelector`. Он позволяет задавать "предпочтения" и "обязательности" для размещения виртуальных машин. `Affinity` поддерживает два вида: `nodeAffinity` и `virtualMachineAndPodAffinity`.

`nodeAffinity` позволяет определять, на каких узлах может быть запущена виртуальная машина, с помощью выражений меток, и может быть мягким (preferred) или жестким (required).

Пример использования nodeAffinity:

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

![](images/placement-node-affinity.ru.png)

В этом примере виртуальная машина будет размещена только на узлах, которые имеют метку `disktype` со значением `ssd`.

`virtualMachineAndPodAffinity` управляет размещением виртуальных машин относительно других виртуальных машин. Он позволяет задавать предпочтение размещения виртуальных машин на тех же узлах, где уже запущены определенные виртуальные машины.

Пример:

```yaml
spec:
  affinity:
    virtualMachineAndPodAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 1
          podAffinityTerm:
            labelSelector:
              matchLabels:
                server: database
            topologyKey: "kubernetes.io/hostname"
```

![](images/placement-vm-affinity.ru.png)

В этом примере виртуальная машина будет размещена, если будет такая возможность (тк используется preffered) только на узлах на которых присутствует виртуальная машина с меткой server и значением database.

#### Избежание совместного размещения (AntiAffinity)

`AntiAffinity` — это противоположность `Affinity`, которая позволяет задавать требования для избегания размещения виртуальных машин на одних и тех же узлах. Это полезно для распределения нагрузки или обеспечения отказоустойчивости.

Термины `Affinity` и `AntiAffinity` применимы только к отношению между виртуальными машинами. Для узлов используемые привязки называются `nodeAffinity`. В `nodeAffinity` нет отдельного антитеза, как в случае с `virtualMachineAndPodAffinity`, но можно создать противоположные условия, задав отрицательные операторы в выражениях меток: чтобы акцентировать внимание на исключении определенных узлов, можно воспользоваться `nodeAffinity` с оператором, таким как `NotIn`.

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

![](images/placement-vm-antiaffinity.ru.png)

В данном примере создаваемая виртуальная машина не будет размещена на одном узле с виртуальной машиной с меткой server: database.
