---
title: "Модуль deckhouse"
search: releaseChannel, стабилизация релизного канала, автосмена канала обновлений
---

Этот модуль настраивает в Deckhouse:

- **[Уровень логирования](configuration.html#parameters-loglevel)**
- **[Набор модулей](configuration.html#parameters-bundle), включенных по умолчанию**

  Обычно используется набор модулей `Default`, который подходит в большинстве случаев.

  Независимо от используемого набора включенных по умолчанию модулей любой модуль может быть явно включен или выключен в конфигурации Deckhouse (подробнее [про включение и отключение модуля](../../#включение-и-отключение-модуля)).
- **[Канал обновлений](configuration.html#parameters-releasechannel)**

  В Deckhouse реализован механизм автоматического обновления. Этот механизм использует [5 каналов обновлений](../../deckhouse-release-channels.html), различающиеся стабильностью и частотой выхода версий. Ознакомьтесь подробнее с тем, [как работает механизм автоматического обновления](../../deckhouse-faq.html#как-работает-автоматическое-обновление-deckhouse) и [как установить желаемый канал обновлений](../../deckhouse-faq.html#как-установить-желаемый-канал-обновлений).
- **[Режим обновлений](configuration.html#parameters-update-mode)** и **[окна обновлений](configuration.html#parameters-update-windows)**

  Deckhouse может использовать **ручной** или **автоматический** режим обновлений.

  В ручном режиме обновлений автоматически применяются только важные исправления (patch-релизы), и для перехода на новый релиз Deckhouse требуется [ручное подтверждение](../../cr.html#deckhouserelease-v1alpha1-approved).

  В автоматическом режиме обновлений, если в кластере **не установлены** [окна обновлений](configuration.html#parameters-update-windows), переход на новый релиз Deckhouse осуществляется сразу после его появления на соответствующем канале обновлений. Если же в кластере **установлены** окна обновлений, переход на более свежий релиз Deckhouse начнется в ближайшее доступное окно обновлений после появления новой версии на канале обновлений.
  
- **Сервис валидирования кастомных ресурсов**

  Сервис валидирования предотвращает создание кастомных ресурсов с некорректными данными или внесение таких данных в уже существующие кастомные ресурсы. Отслеживаются только ресурсы, находящиеся под управлением модулей Deckhouse.

## Обновление релизов Deckhouse

### Просмотр статуса релизов Deckhouse

Список последних релизов в кластере можно получить командной `kubectl get deckhousereleases`. По умолчанию хранятся 10 последних релизов и все будущие.
Каждый релиз может иметь один из следующих статусов:

- `Pending` — релиз находится в ожидании, ждет окна обновления, настроек канареечного развертывания и т. д. Подробности можно увидеть с помощью команды `kubectl describe deckhouserelease $name`.
- `Deployed` — релиз применен. Это значит, что образ пода Deckhouse уже поменялся на новую версию,
 но при этом процесс обновления всех компонентов кластера идет асинхронно, так как зависит от многих настроек.
- `Superseded` — релиз устарел и больше не используется.
- `Suspended` — релиз отменен (например, в нем обнаружилась ошибка). Релиз переходит в этот статус, если его отменили и при этом он еще был применен в кластере.

### Процесс обновления

В момент перехода в статус `Deployed` релиз меняет версию (tag) образа Deckhouse. После запуска Deckhouse начнет проверку и обновление всех модулей, которые поменялись с предыдущего релиза. Длительность обновления зависит от настроек и размера кластера.
Например, если у вас много `NodeGroup`, они будут обновляться продолжительное время, если много `IngressNginxController` — они будут
обновляться по одному и это тоже займет некоторое время.

### Ручное применение релизов

Если выбран [ручной режим обновления](usage.html#ручное-подтверждение-обновлений) и скопилось несколько релизов,
их можно сразу одобрить к применению. В этом случае Deckhouse будет обновляться последовательно, сохраняя порядок релизов и меняя статус каждого примененного релиза.

### Закрепление релиза

Под *закреплением* релиза подразумевается полное или частичное отключение автоматического обновления версий Deckhouse.

Есть три варианта ограничения автоматического обновления Deckhouse:

- Установить ручной режим обновления.

  В этом случае вы остановитесь на текущей версии, сможете получать обновления в кластер, но для применения обновления необходимо будет выполнить [ручное действие](usage.html#ручное-подтверждение-обновлений). Это носится и к patch-версиям, и к минорным версиям.
  
  Для установки ручного режима обновления необходимо в ModuleConfig `deckhouse` установить параметр [settings.update.mode](configuration.html#parameters-update-mode) в `Manual`:

  ```shell
  kubectl patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"Manual"}}}}'
  ```
  
- Установить режим автоматического обновления для патч-версий.

  В этом случае вы остановитесь на текущем релизе, но будете получать patch-версии текущего релиза. Для применения обновления минорной версии релиза необходимо будет выполнить [ручное действие](usage.html#ручное-подтверждение-обновлений).
  
  Например: текущая версия DKP `v1.65.2`, после установки режима автоматического обновления для патч-версий, Deckhouse сможет обновиться до версии `v1.65.6`, но не будет обновляться до версии `v1.66.*` и выше.

  Для установки режима автоматического обновления для патч-версий необходимо в ModuleConfig `deckhouse` установить параметр [settings.update.mode](configuration.html#parameters-update-mode) в `AutoPatch`:

  ```shell
  kubectl patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"AutoPatch"}}}}'
  ```

- Установить конкретный тег для Deployment `deckhouse` и удалить параметр [releaseChannel](configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

  В таком случае DKP останется на конкретной версии, никакой информации о новых доступных версиях (объекты DeckhouseRelease) в кластере появляться не будет.

  Пример установки версии `v1.66.3` для DKP EE и удаления параметра `releaseChannel` из конфигурации модуля `deckhouse`:

  ```shell
  kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- kubectl set image deployment/deckhouse deckhouse=registry.deckhouse.ru/deckhouse/ee:v1.66.3
  kubectl patch mc deckhouse --type=json -p='[{"op": "remove", "path": "/spec/settings/releaseChannel"}]'
  ```
