---
title: Окна обновлений
permalink: ru/admin/configuration/update/update-windows.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) позволяет задавать *окна обновлений* — временные интервалы,
в которые будет выполняться установка обновлений в автоматическом режиме.
Используя окна обновлений,
вы исключаете вероятность установки нового релиза в неподходящее время или в периоды высокой нагрузки на кластер.

## Принцип работы окон обновлений

- Если окна обновлений настроены, DKP будет устанавливать новые версии только в указанные временные интервалы.
- Если окна обновлений не настроены, установка начнется сразу после появления новой версии в настроенном канале обновлений.

## Настройка окон обновлений

Управлять окнами обновлений DKP можно следующими способами:

- **для общего управления обновлениями** используйте параметр `update.windows` модуля `deckhouse`(#TODO);
- **для управления потенциально опасными обновлениями (disruptive updates)** используйте параметры `disruptions.automatic.windows`(#TODO) и `disruptions.rollingUpdate.windows`(#TODO) ресурса NodeGroup.

## Примеры конфигурации

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
