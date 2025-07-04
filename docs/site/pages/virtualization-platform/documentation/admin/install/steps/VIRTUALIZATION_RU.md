---
title: "Настройка виртуализации"
permalink: ru/virtualization-platform/documentation/admin/install/steps/virtualization.html
lang: ru
---

## Настройка виртуализации

После настройки хранилища необходимо включить модуль виртуализации. Включение и настройка модуля производятся с помощью ресурса ModuleConfig.

В параметрах `spec` установите:

- `enabled: true` — флаг для включения модуля;
- `settings.virtualMachineCIDRs` — подсети, IP-адреса из которых будут назначаться виртуальным машинам;
- `settings.dvcr.storage.persistentVolumeClaim.size` — размер дискового пространства для хранения образов виртуальных машин.

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
sudo -i d8 k get po -n d8-virtualization
```

Пример вывода:

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

### Описание параметров

Ниже представлены описания параметров модуля виртуализации.

#### Параметры для включения/выключения модуля

Управление состоянием модуля осуществляется через поле `.spec.enabled`. Укажите:

- `true` — чтобы включить модуль;
- `false` — чтобы выключить модуль.

#### Версия конфигурации

Параметр `.spec.version` определяет версию схемы настроек. Структура параметров может меняться между версиями. Актуальные значения приведены в разделе настроек.

#### Параметры для настройки постоянного тома для хранения образов виртуальных машин (DVCR)

Блок `.spec.settings.dvcr.storage` настраивает постоянный том для хранения образов:

- `.spec.settings.dvcr.storage.persistentVolumeClaim.size` — размер тома (например, `50G`). Для расширения хранилища увеличьте значение параметра;
- `.spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName` — класс хранения (например, `sds-replicated-thin-r1`).

{% alert level="warning" %}
Хранилище, обслуживающее класс хранения (`.spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName`), должно быть доступно на узлах, где запускается DVCR (system-узлы, либо worker-узлы, при отсутствии system-узлов).
{% endalert %}

#### Сетевые настройки

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

#### Настройки классов хранения для образов

Настройки классов хранения для образов определяются в параметре `.spec.settings.virtualImages` настроек модуля.

Пример:

```yaml
spec:
  ...
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

#### Настройки классов хранения для дисков

Настройки классов хранения для дисков определяются в параметре `.spec.settings.virtualDisks` настроек модуля.

Пример:

```yaml
spec:
  ...
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

#### Настройка аудита событий безопасности

{% alert level="warning" %}
Недоступно в Community Edition.
{% endalert %}

{% alert level="warning" %}
Для активации аудита требуется, чтобы были включены следующие модули:

- `log-shipper`,
- `runtime-audit-engine`.
{% endalert %}

Чтобы включить аудит событий безопасности, установите параметр `.spec.settings.audit.enabled` настроек модуля  в `true`:

```yaml
spec:
  enabled: true
  settings:
    audit:
      enabled: true
```
