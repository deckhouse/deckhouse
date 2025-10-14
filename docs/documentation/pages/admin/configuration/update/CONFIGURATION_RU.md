---
title: Настройка обновлений
permalink: ru/admin/configuration/update/configuration.html
description: "Настройка параметров обновлений Deckhouse Kubernetes Platform включая каналы релизов, режимы обновлений и политики обновлений. Автоматическое и ручное управление обновлениями."
lang: ru
---

Deckhouse Kubernetes Platform (DKP) поддерживает гибкий механизм обновлений. Вы можете выбрать [канал обновлений](../../../architecture/updating.html#каналы-обновлений) и настроить режим установки новых версий. Каналы обновлений помогают сбалансировать скорость получения новых функций и стабильность.

Настроив режим обновлений, можно включить автоматическую установку новых версий или управлять ею вручную, а также задать окна обновлений — допустимые интервалы времени для выполнения обновлений. Такая настройка исключает установку в неподходящее время и обеспечивает контроль над переходом на новые релизы.

{% alert level="info" %}
Актуальная информация о версиях DKP на разных каналах обновлений доступна на сайте [releases.deckhouse.ru](https://releases.deckhouse.ru).
{% endalert %}

## Проверка текущего канала обновлений

Чтобы узнать, какой канал обновлений используется в кластере, выполните следующую команду:

```shell
d8 k get mc deckhouse -o yaml | grep releaseChannel
```

Пример вывода:

```console
    releaseChannel: Stable
```

## Смена канала обновлений

Чтобы сменить канал обновлений, укажите его название в [параметре `settings.releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) модуля `deckhouse`.

Пример конфигурации с каналом `Stable`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
```

## Режимы обновления

DKP поддерживает три режима обновления, которые определяют порядок перехода на новую версию:

- **Автоматический режим (без окон обновлений)** — кластер обновляется сразу после появления новой версии
  [на используемом канале обновлений](../../../architecture/updating.html#каналы-обновлений).
- **Автоматический режим (с окнами обновлений)** — кластер обновляется в ближайшее доступное окно
  после появления новой версии на канале обновлений.
- **Ручной режим** — для применения обновления его необходимо подтвердить вручную.

### Проверка текущего режима обновления

Чтобы узнать, какой режим обновления используется в кластере,
проверьте конфигурацию модуля `deckhouse` с помощью следующей команды:

```shell
d8 k get mc deckhouse -o yaml
```

Пример вывода:

```console
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
      - days:
        - Mon
        from: "19:00"
        to: "20:00"
```

### Автоматическое обновление

Автоматический режим активируется при указании параметра [`releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) в конфигурации модуля `deckhouse`.
При выполнении этого условия:

1. DKP раз в минуту проверяет канал обновлений на наличие нового релиза.
1. При появлении нового релиза DKP скачивает его в кластер и создает [кастомный ресурс DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease).
1. После появления кастомного ресурса DeckhouseRelease в кластере
   DKP выполняет обновление на соответствующую версию согласно установленным параметрам обновления
   (по умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов в кластере, воспользуйтесь командой:

```shell
d8 k get deckhousereleases
```

{% alert level="warning" %}
Начиная с версии DKP 1.70, патч-версии обновлений (например, обновление на версию `1.70.2` при установленной версии `1.70.1`) устанавливаются с учетом окон обновлений. До версии DKP 1.70 патч-версии обновлений устанавливаются без учета режима и окон обновлений.
{% endalert %}

#### Закрепление релиза

Под *закреплением релиза* подразумевается полное или частичное отключение автоматического обновления.

Есть три варианта ограничения автоматического обновления Deckhouse:

- Установить режим ручного подтверждения обновлений.

  В этом случае DKP будет получать обновления в кластер,
  но для применения патч-версий и минорных версий потребуется [ручное подтверждение](#ручное-подтверждение-обновлений).
  
  Чтобы установить режим ручного подтверждения обновлений,
  задайте значение `Manual` для [параметра `settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode) в настройках модуля `deckhouse` с помощью следующей команды:

  ```shell
  d8 k patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"Manual"}}}}'
  ```

  Чтобы подтвердить обновление, выполните следующую команду, указав необходимую версию DKP вместо `<DECKHOUSE-VERSION>`:

  ```shell
  d8 k patch DeckhouseRelease <DECKHOUSE-VERSION> --type=merge -p='{"approved": true}'
  ```

- Установить режим автоматического обновления для патч-версий.

  В этом случае DKP будет получать обновления в кластер,
  но для применения минорных версий потребуется [ручное подтверждение](#ручное-подтверждение-обновлений).
  Патч-версии в рамках текущей минорной версии будут устанавливаться автоматически
  с учётом окон обновлений, если они заданы.

  Например, если у вас установлена версия DKP `v1.70.1`,
  после установки этого режима Deckhouse сможет автоматически обновиться до версии `v1.70.2`,
  но не будет обновляться до версии `v1.71.*` без ручного подтверждения.

  Чтобы установить режим автоматического обновления для патч-версий,
  задайте значение `AutoPatch` для [параметра `settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode) в настройках модуля `deckhouse` с помощью следующей команды:

  ```shell
  d8 k patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"AutoPatch"}}}}'
  ```

  Чтобы подтвердить обновление для минорной версии, выполните следующую команду,
  указав необходимую версию DKP вместо `<DECKHOUSE-VERSION>`:

  ```shell
  d8 k patch DeckhouseRelease <DECKHOUSE-VERSION> --type=merge -p='{"approved": true}'
  ```

- Установить тег необходимой версии DKP для Deployment `deckhouse`
  и удалить [параметр `releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

  В этом случае DKP останется на указанной версии
  и никакая информация о новых доступных версиях (объекты DeckhouseRelease) в кластере появляться не будет.
  
  > **Важно**. Этот режим заблокирует установку патч-релизов,
  > которые могут содержать исправления критических уязвимостей и ошибок.

  Пример установки версии `v1.66.3` для DKP EE и удаления параметра `releaseChannel` из конфигурации модуля `deckhouse`:

  ```shell
  d8 k -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- kubectl set image deployment/deckhouse deckhouse=registry.deckhouse.ru/deckhouse/ee:v1.66.3
  d8 k patch mc deckhouse --type=json -p='[{"op": "remove", "path": "/spec/settings/releaseChannel"}]'
  ```

### Ручное подтверждение обновлений

Ручное подтверждение обновления версии DKP предусмотрено в следующих случаях:

- Включен режим подтверждения обновлений DKP.

  Это значит, что для [параметра `settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode) модуля `deckhouse` задано значение `Manual`
  (подтверждение как патч-версии, так и минорной версии DKP) или `AutoPatch` (подтверждение минорной версии DKP).

  Для подтверждения обновления выполните следующую команду, указав необходимую версию DKP вместо `<DECKHOUSE-VERSION>`:

  ```shell
  d8 k patch DeckhouseRelease <DECKHOUSE-VERSION> --type=merge -p='{"approved": true}'
  ```

- Если для какой-либо группы узлов отключено автоматическое применение обновлений,
  которые могут привести к кратковременному простою в работе системных компонентов.

  Это значит,
  что для [параметра `spec.disruptions.approvalMode`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) у соответствующего ресурса NodeGroup задано значение `Manual`.

  Для обновления узлов в такой группе установите аннотацию `update.node.deckhouse.io/disruption-approved=` на каждый узел.

  Пример:

  ```shell
  d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

## Окна обновлений

DKP позволяет задавать *окна обновлений* — временные интервалы,
в которые будет выполняться установка обновлений в автоматическом режиме.
Используя окна обновлений,
вы исключаете вероятность установки нового релиза в неподходящее время или в периоды высокой нагрузки на кластер.

### Установка обновлений при настроенных окнах обновлений

- Если окна обновлений настроены, DKP будет устанавливать новые версии только в указанные временные интервалы.
- Если окна обновлений не настроены, установка начнется сразу после появления новой версии в настроенном канале обновлений.

### Настройка окон обновлений

Управлять окнами обновлений DKP можно следующими способами:

- **для общего управления обновлениями** используйте [параметр `update.windows`](/modules/deckhouse/configuration.html#parameters-update-windows) модуля `deckhouse`;
- **для управления обновлениями, которые могут привести к кратковременному простою в работе системных компонентов**, используйте параметры [`disruptions.automatic.windows`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) и [`disruptions.rollingUpdate.windows`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-rollingupdate-windows) ресурса NodeGroup.

#### Примеры конфигурации

- Два ежедневных окна обновлений с 8:00 до 10:00 и c 20:00 до 22:00 (UTC):

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: deckhouse
  spec:
    version: 1
    settings:
      releaseChannel: EarlyAccess
      update:
        windows: 
          - from: "8:00"
            to: "10:00"
          - from: "20:00"
            to: "22:00"
  ```

- Окна обновлений по вторникам и субботам с 18:00 до 19:30 (UTC):

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: deckhouse
  spec:
    version: 1
    settings:
      releaseChannel: Stable
      update:
        windows: 
          - from: "18:00"
            to: "19:30"
            days:
              - Tue
              - Sat
  ```
