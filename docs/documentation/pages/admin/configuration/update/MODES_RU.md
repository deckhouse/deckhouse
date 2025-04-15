---
title: Режимы обновлений
permalink: ru/admin/configuration/update/modes.html
lang: ru
---

Deckhouse Kubernetes Platform поддерживает три режима обновления. От выбора режима зависит порядок обновления:

* **Автоматический + окна обновлений не заданы.** Кластер обновится сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html).
* **Автоматический + заданы окна обновлений.** Кластер обновится в ближайшее доступное окно после появления новой версии на канале обновлений.
* **Ручной режим.** Для применения обновления требуются [ручные действия](modules/deckhouse/usage.html#ручное-подтверждение-обновлений).

Посмотреть режим обновления кластера можно в [конфигурации](modules/deckhouse/configuration.html) модуля `deckhouse`. Для этого выполните следующую команду:

```shell
kubectl get mc deckhouse -oyaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: "2022-12-14T11:13:03Z"
  generation: 1
  name: deckhouse
  resourceVersion: "3258626079"
  uid: c64a2532-af0d-496b-b4b7-eafb5d9a56ee
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
      - days:
        - Mon
        from: "19:00"
        to: "20:00"
  version: 1
status:
  state: Enabled
  status: ""
  type: Embedded
  version: "1"
```

## Ручное обновление

Чтобы обновить Deckhouse Kubernetes Platform до нужной версии в ручном режиме, выполните следующую команду. В переменную <DECKHOUSE_VERSION> введите необходимую версию Deckhouse:

```bash
kubectl patch DeckhouseRelease <DECKHOUSE_VERSION> --type=merge -p='{"approved": true}'
```

## Настройка параметра update.disruptionApprovalMode

Некоторые минорные релизы могут содержать потенциально опасные (disruptive) изменения, влияющие на базовые настройки и работу модулей. Для управления обновлением таких релизов служит параметр `update.disruptionApprovalMode`:
- Auto (автоматический) - Минорные потенциально опасные обновления устанавливаются без дополнительного подтверждения.
- Manual (ручной)
  * Каждое потенциально опасное минорное обновление Deckhouse Kubernetes Platform требует ручного подтверждения.
  * Для этого необходимо добавить аннотацию `release.deckhouse.io/disruption-approved=true` к соответствующему ресурсу `DeckhouseRelease`.

По умолчанию используется режим `Auto`.

### Отключение автоматического обновления

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/deckhouse/configuration.html) модуля `deckhouse` параметр [releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel).

В этом случае Deckhouse не проверяет обновления и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}
