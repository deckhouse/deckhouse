---
title: "История изменений"
permalink: ru/virtualization-platform/documentation/release-notes.html
lang: ru
---

## v1.5.1

### Исправления

- [vd] Исправлена проблема при создании виртуального диска из виртуального образа, хранящегося на `PersistentVolumeClaim` (при значении `.spec.storage=PersistentVolumeClaim`).

## v1.5.0

### Новые возможности

- [vm] Добавлена возможность таргетированной миграции для виртуальных машин.
  Для этого нужно создать ресурс [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) с типом `Migrate` и в нём указать `.spec.migrate.nodeSelector` для миграции машины на соответствующий узел.
- [observability] На дашборд `Namespace / Virtual Machine` добавлена таблица с операциями по виртуальной машине.

### Исправления

- [core] Исправлен запуск виртуальных машин с загрузчиком `EFIWithSecureBoot` при конфигурации с более чем 12 vCPU.
- [vmop] Исправлена проблема клонирования виртуальной машины, диски которой используют хранилище в режиме `WaitForFirstConsumer`.
- [module] Ресурсы системных компонентов, обеспечивающие запуск и работу виртуальных машин, не учитываются в квотах проекта.
- [module] При миграции виртуальной машины временное двойное потребление ресурсов больше не учитывается в квотах проекта.
- [module] Системные компоненты платформы в пользовательских проектах защищены от удаления пользователями.
- [vm] Исправлено зависание виртуальной машины в `Pending` в процессе миграции при смене StorageClass.
- [vd] Исправлена проблема живой миграции виртуальной машины между StorageClass с типом `Filesystem`.

### Прочее

- [vd] При просмотре дисков теперь отображается имя виртуальной машины, к которой они подключены (`d8 k get vd`).

## v1.4.1

### Безопасность

- [module] Исправлены уязвимости CVE-2025-61726, CVE-2025-61728, CVE-2025-61730 и CVE-2025-68121.

## v1.4.0

### Новые возможности

- [vd] Добавлена поддержка изменения StorageClass для дисков, подключённых через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug).
- [vd] Добавлена поддержка миграции виртуальных машин с локальными дисками, подключёнными через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug).
- [vm] Теперь виртуальную машину можно запускать без сети `Main`.

### Исправления

- [module] Исправлен учёт ресурсов системных компонентов в квотах проекта (для создания дисков/образов и их подключения к виртуальной машине через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug)).
- [vi,cvi] Добавлено отслеживание наличия образов в DVCR: если образ пропадает из DVCR, соответствующие ресурсы [VirtualImage](/modules/virtualization/cr.html#virtualimage) и [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) переходят в фазу `Lost` и получают ошибку в статусе.
- [vmip] Исправлено присоединение IP-адреса, если ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) был создан пользователем заранее вручную.
- [vm] Добавлена поддержка клонирования виртуальных машин в состоянии `Running` через [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) с типом `Clone`.

## v1.3.0

### Новые возможности

- [vmclass] В ресурсе [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) добавлено поле `.spec.sizingPolicies.defaultCoreFraction`, позволяющее задать значение `coreFraction` по умолчанию для виртуальных машин, использующих этот класс.

### Исправления

- [vi/cvi] Добавлена возможность использования системных узлов для создания проектных и кластерных образов.
- [vd] Ускорено подключение дисков в режиме `WaitForFirstConsumer` к виртуальной машине.
- [vd] Исправлена проблема с восстановлением меток и аннотаций на диске, созданном из снимка.
- [observability] В кластерах, работающих в HA режиме, исправлено отображение графиков по виртуальным машинам.

## v1.2.2

### Исправления

- [module] Для роли `d8:use:role:user` исправлены права доступа RBAC, которые не позволяли управлять ресурсом [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation).

## v1.2.1

### Исправления

- [module] Удалена устаревшая часть конфигурации, из‑за которой обновление модуля виртуализации могло не выполняться в кластерах с Kubernetes версии 1.34 и выше.

## v1.2.0

### Новые возможности

- [vmrestore] Ресурс [VirtualMachineRestore](/modules/virtualization/cr.html#virtualmachinerestore) помечен как устаревший (deprecated). Вместо него используйте следующие ресурсы:
  - [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) с типом `Clone` - для клонирования существующей виртуальной машины;
  - [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) с типом `Restore` - для восстановления существующей виртуальной машины до состояния из снимка;
  - [VirtualMachineSnapshotOperation](/modules/virtualization/cr.html#virtualmachinesnapshotoperation) - для создания новой виртуальной машины на основе снимка.
- [vmsop] Добавлен ресурс [VirtualMachineSnapshotOperation](/modules/virtualization/cr.html#virtualmachinesnapshotoperation) для создания виртуальной машины на основе снимка [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot).
- [vmclass] Для ресурса [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) версия `v1alpha2` помечена как устаревшая (deprecated). Вместо неё рекомендуется использовать версию `v1alpha3`:
  - в версии `v1alpha3` поле `.spec.sizingPolicies.coreFraction` теперь задаётся строкой с указанием процента (например, "50%"), аналогично полю в виртуальной машине.
- [module] Для ModuleConfig виртуализации добавлена валидация, запрещающая уменьшать размер и изменять выбранный StorageClass для DVCR.
- [module] Улучшены события аудита: сообщения стали более информативными и теперь включают имена виртуальных машин и информацию о пользователях.
- [module] Добавлена возможность очищать DVCR от несуществующих проектных и кластерных образов:
  - по умолчанию эта функция отключена;
  - чтобы включить очистку, задайте расписание в настройках модуля: `.spec.settings.dvcr.gc.schedule`.
- [vmbda] В условие `Attached` ресурса [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) добавлен подробный вывод ошибки, возникающей при недоступности блочного устройства на узле виртуальной машины.
- [module] Добавлены новые метрики для дисков:
  - `d8_virtualization_virtualdisk_capacity_bytes` - метрика, показывающая размер диска;
  - `d8_virtualization_virtualdisk_info` - метрика с информацией о конфигурации диска;
  - `d8_virtualization_virtualdisk_status_inuse` - метрика, показывающая текущее использование диска виртуальной машиной или для создания других блочных устройств.

### Исправления

- [vmclass] Добавлена возможность изменять или удалять ресурс [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) с именем generic. Теперь модуль виртуализации не будет восстанавливать его в исходное состояние.
- [vm] Исправлена ошибка `MethodNotAllowed` для операций `patch` и `watch` при запросах к ресурсу [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) через утилиты командной строки (`d8 k`, `kubectl`).
- [image] Исправлена проблема, из-за которой было невозможно удалить ресурсы [VirtualImage](/modules/virtualization/cr.html#virtualimage) и [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) для остановленной виртуальной машины.
- [module] Исправлена конфигурация RBAC для кластерных ролей `user` и `editor`.
- [module] Исправлен алерт `D8VirtualizationVirtualMachineFirmwareOutOfDate`, который мог дублироваться при работе виртуализации в HA режиме.
- [snapshot] Исправлена ошибка, которая могла приводить к неконсистентности ресурсов [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot) и [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) при создании снимка виртуальной машины с несколькими дисками.

### Безопасность

- [module] Исправлена уязвимость CVE-2025-64324.

## v1.1.3

### Безопасность

- [module] Исправлены уязвимости CVE-2025-64324, CVE-2025-64435, CVE-2025-64436, CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-58188, CVE-2025-52565, CVE-2025-52881, CVE-2025-31133.

### Прочее

- [observability] Доработаны дашборды обзора виртуальных машин (`Namespace / Virtual Machine` и `Namespace / Virtual Machines`): помимо уровня кластера, они теперь доступны и на уровне проекта.

## v1.1.2

### Исправления

- [vd] Исправлена живая миграция дисков между StorageClass, использующими разные драйверы. Ограничения:
  - Не поддерживается миграция с `Block` на `Filesystem` и наоборот. Миграция возможна только между одинаковыми режимами томов (volume mode): `Block` → `Block` и `Filesystem` → `Filesystem`.
- [vm] В состоянии `Migrating` при неуспешной живой миграции виртуальной машины добавлено отображение подробной информации об ошибке.

## v1.1.1

### Исправления

- [core] Исправлена проблема в containerd v2, из-за которой хранилище, предоставляющее PVC с типом `FileSystem`, некорректно подключалось через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment).
- [core] Добавлено отображение ошибок в статусе дисков и образов при недоступности источника данных (URL).
- [vi] Теперь при создании виртуальных образов из снимков виртуальных дисков учитывается параметр `.spec.persistentVolumeClaim.storageClassName`. Ранее он мог игнорироваться.
- [vm] Исправлен вывод условия `NetworkReady`: он больше не отображается в состоянии `Unknown` и показывается только при необходимости.
- [vm] Добавлена валидация, предотвращающая указание одной и той же сети в спецификации виртуальной машины `.spec.network` более одного раза.
- [vmip] Добавлена валидация для статических IP-адресов, предотвращающая создание ресурсов [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) с уже используемым в кластере адресом.
- [vmbda] Исправлена ошибка, из-за которой при отключении виртуального образа через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) ресурс мог зависать в состоянии `Terminating`.

### Прочее

- [observability] Добавлены метрики Prometheus для снимков виртуальных машин (`d8_virtualization_virtualmachinesnapshot_info`) и дисков (`d8_virtualization_virtualdisksnapshot_info`), показывающие, к каким объектам они относятся.

### Безопасность

- [module] Исправлены уязвимости CVE-2025-58058 и CVE-2025-54410.

## v1.1.0

### Новые возможности

- [vm] Добавлена возможность миграции ВМ, использующих диски на локальных хранилищах. Ограничения:
  - Функция недоступна в CE-редакции.
  - Миграция возможна только для запущенной ВМ (`phase: Running`)
  - Миграция ВМ с локальными дисками, подключенными через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug), пока недоступна.
- [vd] Добавлена возможность миграции хранилища для дисков ВМ (изменение StorageClass). Ограничения:
  - Функция недоступна в CE-редакции.
  - Миграция возможна только для запущенной ВМ (`phase: Running`)
  - Миграция хранилища для дисков, подключенных через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (hotplug), пока недоступна.
- [vmop] Добавлена операция с типом `Clone` для создания клона ВМ из существующей ВМ ([VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) `.spec.type: Clone`).
- [observability] Добавлен алерт `KubeNodeAwaitingVirtualMachinesEvictionBeforeShutdown`, срабатывающий при получении узлом, на котором размещены виртуальные машины, команды на завершение работы — до завершения эвакуации ВМ.
- [observability] Добавлен алерт `D8VirtualizationDVCRInsufficientCapacityRisk`, предупреждающий о риске нехватки свободного места в хранилище образов виртуальных машин (DVCR).

### Исправления

- [vmclass] Исправлена ошибка в [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) типах `Features` и `Discovery`, из-за которой на узлах с процессорами AMD не работала вложенная виртуализация.
- [vmop/restore] Исправлена ошибка, при которой контроллер иногда запускал восстановленную ВМ до завершения восстановления её дисков, в результате чего ВМ стартовала со старыми (не восстановленными) дисками.
- [vmsnapshot] Исправлено поведение при создании снимка ВМ при наличии неприменённых изменений: снимок теперь мгновенно фиксирует актуальное состояние виртуальной машины, включая все текущие изменения.
- [module] Исправлена проблема установки модуля на RedOS 8.X ОС.
- [module] Улучшена валидация, предотвращающая добавление пустых значений для параметров, определяющих StorageClass для дисков и образов.
- [vmop] Исправлена работа сборщика мусора: ранее при перезапуске virtualization-controller все объекты VMOP удалялись без учёта правил очистки.
- [observability] Дашборд виртуальной машины теперь отображает статистику по всем сетям (в том числе и дополнительным), подключённым к ВМ.
- [observability] На дашборде виртуальной машины исправлен график, отображающий статистику копирования памяти во время миграции ВМ.

## v1.0.0

### Новые возможности

- [vm] Добавлена защита от подключения cloud-образа ([VirtualImage](/modules/virtualization/cr.html#virtualimage) \ [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage)) в качестве первого диска. Ранее это приводило к невозможности запуска ВМ с ошибкой "No bootable device".
- [vmop] Добавлена операция с типом `Restore` для восстановления ВМ из ранее созданного снимка.

### Исправления

- [vmsnapshot] Теперь при восстановлении виртуальной машины из снимка корректно восстанавливаются все аннотации и метки, которые были у ресурсов в момент снимка.
- [module] Исправлена проблема с блокировкой очереди, когда параметр `settings.modules.publicClusterDomain` был пустым в глобальном ресурсе ModuleConfig.
- [module] Оптимизирована производительность хука во время установки модуля.
- [vmclass] Исправлена валидация `core`/`coreFraction` в ресурсе [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass).
- [module] При выключенном модуле `sdn` конфигурация дополнительных сетей в ВМ недоступна.

### Безопасность

- Устранено CVE-2025-47907.

## v0.25.0

### Важная информация перед обновлением

В версии v0.25.0 добавлена поддержка работы модуля с CRI containerd v2.
После обновления CRI с containerd v1 до containerd v2 необходимо пересоздать образы, которые были созданы с использованием версии модуля виртуализации v0.24.0 и ранее.

### Новые возможности

- [vm] Добавлена возможность подключения к виртуальной машине дополнительных сетевых интерфейсов к сетям, предоставляемым модулем `sdn`. Для этого модуль `sdn` должен быть включен в кластере.
- [vmmac] Для дополнительных сетевых интерфейсов добавлено управление MAC-адресами с использованием ресурсов [VirtualMachineMACAddress](/modules/virtualization/cr.html#virtualmachinemacaddress) и [VirtualMachineMACAddressLease](/modules/virtualization/cr.html#virtualmachinemacaddresslease).
- [vmclass] Добавлена аннотация для установки класса виртуальной машины по умолчанию. Чтобы назначить [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) по умолчанию, необходимо добавить на него аннотацию `virtualmachineclass.virtualization.deckhouse.io/is-default-class=true`. Это позволяет создавать ВМ с пустым полем `spec.virtualMachineClassName`, автоматически заполняя его классом по умолчанию.
- [observability] Добавлены новые метрики Prometheus для отслеживания фазы ресурсов, таких как [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot), [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot), [VirtualImage](/modules/virtualization/cr.html#virtualimage) и [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).

### Исправления

- [vm] Исправлена проблема: при изменении типа операционной системы машина уходила в циклическую перезагрузку.
- [vm] Исправлено зависание виртуальной машины в фазе `Starting` при нехватке квот проекта. Сообщение о нехватке квот будет отображаться в статусе виртуальной машины. Чтобы машина продолжила запуск, необходимо будет увеличить квоты проекта.
- [vi] Для создания виртуального образа на `PersistentVolumeClaim` должно быть использовано хранилище в режиме `RWX` и `Block`, в противном случае будет отображено предупреждение об ошибке.
- [module] Добавлена валидация, проверяющая, что подсети виртуальных машин не пересекаются с системными подсетями (`podSubnetCIDR` и `serviceSubnetCIDR`).

### Прочее

- [vmop] Улучшен сборщик мусора (GC) для завершённых операций виртуальной машины:
  - GC запускается каждый день в 00:00;
  - GC будет удалять успешно завершённые операции (`Completed` / `Failed`), если истёк их TTL (24 часа);
  - GC подчищает все завершённые операции (`Completed` / `Failed`), оставляя только 10 последних.
