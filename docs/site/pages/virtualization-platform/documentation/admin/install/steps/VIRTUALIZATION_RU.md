---
title: "Настройка виртуализации"
permalink: ru/virtualization-platform/documentation/admin/install/steps/virtualization.html
lang: ru
---

{% alert level="info" %}
Для выполнения приведенных ниже команд необходима установленная утилита [d8](/products/kubernetes-platform/documentation/v1/cli/d8/) (Deckhouse CLI) и настроенный контекст kubectl для доступа к кластеру. Также, можно подключиться к master-узлу по SSH и выполнить команду от пользователя `root` с помощью `sudo -i`.
{% endalert %}

После настройки хранилища необходимо включить модуль `virtualization`. Включение и настройка модуля производятся с помощью веб-интерфейса администратора или с помощью следующей команды:

```shell
d8 s module enable virtualization
```

Отредактируйте конфигурацию модуля [одним из способов](#конфигурация-модуля-virtualization).

В конфигурации модуля укажите:

- [settings.virtualMachineCIDRs](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) — подсети, IP-адреса из которых будут назначаться виртуальным машинам;
- [settings.dvcr.storage.persistentVolumeClaim.size](/modules/virtualization/configuration.html#parameters-dvcr-storage-persistentvolumeclaim-size) — размер дискового пространства для хранения образов виртуальных машин;
- [settings.dvcr.storage.persistentVolumeClaim.storageClassName](/modules/virtualization/configuration.html#parameters-dvcr-storage-persistentvolumeclaim-storageclassname) — имя StorageClass, используемого для создания PersistentVolumeClaim (если не указан, то будет использоваться StorageClass используемый по умолчанию);
- [settings.dvcr.storage.type](/modules/virtualization/configuration.html#parameters-dvcr-storage-type) — укажите `PersistentVolumeClaim`.

Пример базовой настройки модуля виртуализации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  version: 1
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
          storageClassName: sds-replicated-thin-r1
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
      - 10.66.10.0/24
```

Дождитесь, пока все поды модуля не перейдут в статус `Running`:

```shell
d8 k get po -n d8-virtualization
```

{% offtopic title="Пример вывода..." %}

```console
NAME                                         READY   STATUS    RESTARTS      AGE
cdi-apiserver-858786896d-rsfjw               3/3     Running   0             10m
cdi-deployment-6d9b646b5b-8dgmj              3/3     Running   0             10m
cdi-operator-5fdc989d9f-zmk55                3/3     Running   0             10m
dvcr-74dc9c94b-pczhx                         2/2     Running   0             10m
virt-api-78d49dcbbf-qwggw                    3/3     Running   0             10m
virt-controller-6f8fff445f-w866w             3/3     Running   0             10m
virt-handler-g6l9h                           4/4     Running   0             10m
virt-handler-t5fgb                           4/4     Running   0             10m
virt-handler-ztj77                           4/4     Running   0             10m
virt-operator-58dc5459d5-hpps8               3/3     Running   0             10m
virtualization-api-5d69f55947-k6h9n          1/1     Running   0             10m
virtualization-controller-69647d98c6-9rkht   3/3     Running   0             10m
vm-route-forge-288z7                         1/1     Running   0             10m
vm-route-forge-829wm                         1/1     Running   0             10m
vm-route-forge-nq9xr                         1/1     Running   0             10m
```

{% endofftopic %}

## Конфигурация модуля `virtualization`

Изменить конфигурацию модуля `virtualization` можно через веб-интерфейс администратора или через CLI.

### Через веб-интерфейс администратора

- Перейдите на вкладку «Система», далее в раздел «Deckhouse» → «Модули».
- Из списка выберите модуль `virtualization`.
- Во всплывающем окне выберите вкладку «Конфигурация».
- Для отображения настроек нажмите переключатель «Дополнительные настройки».
- Укажите необходимые параметры модуля.
- Для применения настроек нажмите кнопку «Сохранить».

### Через CLI

```shell
d8 k edit mc virtualization
```

## Описание параметров

Ниже представлены описания параметров модуля виртуализации.

### Версия конфигурации

Параметр `.spec.version` определяет версию схемы настроек. Структура параметров может меняться между версиями. Актуальные значения приведены в разделе настроек.

### Параметры для настройки постоянного тома для хранения образов виртуальных машин (DVCR)

Блок `.spec.settings.dvcr.storage` настраивает постоянный том для хранения образов:

- `.spec.settings.dvcr.storage.persistentVolumeClaim.size` — размер тома (например, `50G`). Для расширения хранилища увеличьте значение параметра;
- `.spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName` — класс хранения (например, `sds-replicated-thin-r1`).

{% alert level="warning" %}
Хранилище, обслуживающее класс хранения (параметр `.spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName`), должно быть доступно на узлах, где запускается DVCR (system-узлы, либо worker-узлы, при отсутствии system-узлов).
{% endalert %}

### Сетевые настройки

В блоке `.spec.settings.virtualMachineCIDRs` указываются подсети в формате CIDR (например, `10.66.10.0/24`). IP-адреса для виртуальных машин распределяются из этих - диапазонов автоматически или по запросу.

Пример:

```yaml
spec:
  settings:
    virtualMachineCIDRs:
      - 10.66.10.0/24
      - 10.66.20.0/24
      - 10.77.20.0/16
```

Первый и последний адреса подсети зарезервированы и недоступны для использования.

{% alert level="warning" %}
Подсети блока `.spec.settings.virtualMachineCIDRs` не должны пересекаться с подсетями узлов кластера, подсетью сервисов или подсетью подов (`podCIDR`).

Запрещено удалять подсети, если адреса из них уже выданы виртуальным машинам.
{% endalert %}

### Настройки классов хранения для образов

Настройки классов хранения для образов определяются в параметре `.spec.settings.virtualImages` настроек модуля.

Пример:

```yaml
spec:
  #...
  settings:
    virtualImages:
      allowedStorageClassNames:
      - sc-1
      - sc-2
      defaultStorageClassName: sc-1
```

Здесь:

- `allowedStorageClassNames` (опционально) — это список допустимых StorageClass для создания VirtualImage, которые можно явно указать в спецификации ресурса;
- `defaultStorageClassName` (опционально) — это StorageClass, используемый по умолчанию при создании VirtualImage, если параметр `.spec.persistentVolumeClaim.storageClassName` не задан.

### Настройки классов хранения для дисков

Настройки классов хранения для дисков определяются в параметре `.spec.settings.virtualDisks` настроек модуля.

Пример:

```yaml
spec:
  #...
  settings:
    virtualDisks:
      allowedStorageClassNames:
      - sc-1
      - sc-2
      defaultStorageClassName: sc-1
```

Здесь:

- `allowedStorageClassNames` (опционально) — это список допустимых StorageClass для создания VirtualDisk, которые можно явно указать в спецификации ресурса;
- `defaultStorageClassName` (опционально) — это StorageClass, используемый по умолчанию при создании VirtualDisk, если параметр `.spec.persistentVolumeClaim.storageClassName` не задан.

### Настройка аудита событий безопасности

{% alert level="warning" %}
Недоступно в Community Edition.
{% endalert %}

{% alert level="warning" %}
Для активации аудита требуется, чтобы были включены следующие модули:
- `log-shipper`,
- `runtime-audit-engine`.
{% endalert %}

Чтобы включить аудит событий безопасности, установите параметр `.spec.settings.audit.enabled` настроек модуля в `true`:

```yaml
spec:
  enabled: true
  settings:
    audit:
      enabled: true
```
