---
title: "Модуль deckhouse: примеры конфигурации"

---


## Настройка режима обновления

Управлять обновлением DKP можно следующими способами:
- С помощью параметра [settings.update](configuration.html#parameters-update) ModuleConfig `deckhouse`;
- С помощью секции параметров [disruptions](../node-manager/cr.html#nodegroup-v1-spec-disruptions) NodeGroup.

### Конфигурация окон обновлений

Управлять временными окнами, когда Deckhouse будет устанавливать обновления автоматически, можно следующими способами:
- в параметре [update.windows](configuration.html#parameters-update-windows) ModuleConfig `deckhouse`, для общего управления обновлениями;
- в параметрах [disruptions.automatic.windows](../node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) и [disruptions.rollingUpdate.windows](../node-manager/cr.html#nodegroup-v1-spec-disruptions-rollingupdate-windows) NodeGroup, для управления обновлениями, которые могут привести к кратковременному простою в работе системных компонентов.

Пример настройки двух ежедневных окон обновлений: с 8:00 до 10:00 и c 20:00 до 22:00 (UTC):

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

Также можно настроить обновления в определенные дни, например по вторникам и субботам с 18:00 до 19:30 (UTC):

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

<div id="ручное-подтверждение-потенциально-опасных-disruptive-обновлений"></div>

### Ручное подтверждение обновлений

Ручное подтверждение обновления версии Deckhouse предусмотрено в следующих случаях:
- Включен режим подтверждения обновлений Deckhouse.

  Это значит, что параметр [settings.update.mode](configuration.html#parameters-update-mode) ModuleConfig `deckhouse` установлен в `Manual` (подтверждение как patch-версии, так и минорной версии Deckhouse) или в `AutoPatch` (подтверждение минорной версии Deckhouse).
  
  Для подтверждения обновления выполните следующую команду (укажите необходимую версию Deckhouse):

  ```shell
  kubectl patch DeckhouseRelease <VERSION> --type=merge -p='{"approved": true}'
  ```

- Если для какой-либо группы узлов отключено автоматическое применение обновлений, которые могут привести к кратковременному простою в работе системных компонентов.

  Это значит, что у NodeGroup, соответствующего группе узлов, установлен параметр [spec.disruptions.approvalMode](../node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) в `Manual`.

  Для обновления **каждого** узла в такой группе на узел нужно установить аннотацию `update.node.deckhouse.io/disruption-approved=`.
  
  Пример:

  ```shell
  kubectl annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

### Оповещение об обновлении Deckhouse

В режиме обновлений `Auto` можно [настроить](configuration.html#parameters-update-notification) вызов вебхука для получения оповещения о предстоящем обновлении минорной версии Deckhouse.

Кроме того, оповещения формируются не только при обновлении Deckhouse, но и при обновлении любых модулей, включая их отдельные обновления.
В отдельных случаях система может инициировать отправку нескольких оповещений одновременно (по 10–20 оповещений) с интервалом около 15 секунд.

{% alert %}
Оповещения доступны только в режиме обновлений `Auto`, в режиме `Manual` они не формируются.
{% endalert %}

{% alert %}
Вебхук указывать не обязательно: если параметр `update.notification.webhook` не задан, но указано время в параметре `update.notification.minimalNotificationTime`, применение новой версии всё равно будет отложено на указанный период. В этом случае оповещением о появлении новой версии можно считать появление в кластере ресурса [DeckhouseRelease](../../cr.html#deckhouserelease) с именем новой версии.
{% endalert %}

Оповещения отправляются только один раз для конкретного обновления. Если что-то пошло не так (например, вебхук получил некорректные данные), повторная отправка автоматически не произойдёт. Чтобы отправить оповещение повторно, необходимо удалить соответствующий ресурс [DeckhouseRelease](../../cr.html#deckhouserelease).

Пример настройки оповещения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
```

После появления новой минорной версии Deckhouse на используемом канале обновлений, но до момента применения ее в кластере на адрес вебхука будет выполнен [POST-запрос](configuration.html#parameters-update-notification-webhook).

Параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime) позволяет отложить установку обновления на заданный период, обеспечивая время для реакции на оповещение с учётом окон обновлений. Если при этом вебхук недоступен, каждая неудачная попытка отправки будет сдвигать время применения на ту же величину, что может привести к бесконечному откладыванию обновления.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
        minimalNotificationTime: 8h
```

## Сбор информации для отладки

О сборе отладочной информации читайте [в FAQ](faq.html#как-собрать-информацию-для-отладки).
