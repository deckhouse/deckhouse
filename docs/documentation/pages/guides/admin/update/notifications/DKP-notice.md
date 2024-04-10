---
title: Получение уведомлений
permalink: ru/update/notifications/
lang: ru
---

## Оповещение об уведомлении

Информацию о всех версиях Deckhouse Kubernetes Platform можно найти в [списке релизов](https://github.com/deckhouse/deckhouse/releases) Deckhouse Kubernetes Platform.

Сводную информацию о важных изменениях, об обновлении версий компонентов, а также о том, какие компоненты в кластере буду перезапущены в процессе обновления, можно найти в описании нулевой patch-версии релиза. Например, [v1.58.0](https://github.com/deckhouse/deckhouse/releases/tag/v1.58.0) для релиза v1.58 Deckhouse Kubernetes Platform.

Подробный список изменений можно найти в Changelog, ссылка на который есть в каждом [релизе](https://github.com/deckhouse/deckhouse/releases). При настройке автоматического оповещения, ссылка на Changelog отображается в строке `changelogLink`.
Опционально: перед обновлением необходимо ознакомиться с *Changelog* и внести соответствующие изменения в кластер.

Чтобы получать оповещения об обновлении Deckhouse Kubernetes Platform в автоматическом режиме, выполните следующие шаги:

1. Пропишите URL-адрес вебхука.

   Вызов вебхука произойдет после появления новой минорной версии Deckhouse на используемом канале обновлений, но до момента ее применения в кластере.

2. Используйте параметр `minimalNotificationTime`, чтобы установить минимальное время, которое должно пройти перед обновлением с момента появления новой минорной версии Deckhouse Kubernetes Platform на используемом канале обновлений.

   На адрес вебхука выполится POST-запрос с `Content-Type: application/json`. Пример содержания запроса с параметрами вебхука:

   ```
   {
     "version": "1.36",
     "requirements":  {"k8s": "1.20.0"},
     "changelogLink": "https://github.com/deckhouse/deckhouse/changelog/1.36.md",
     "applyTime": "2023-01-01T14:30:00Z00:00",
     "message": "New Deckhouse Release 1.36 is available. Release will be applied at: Friday, 01-Jan-23 14:30:00 UTC"
   }
   ```

   Где параметры вебхука - это:

   * `version` — строка, номер минорной версии;
   * `requirements` — объект, требования к версии;
   * `changelogLink` — строка, ссылка на список изменений (changelog) минорной версии;
   * `applyTime` — строка, дата и время запланированного обновления (с учетом установленных окон обновлений) в формате RFC3339;
   * `message` — строка, текстовое сообщение о доступности новой минорной версии и запланированном времени обновления.

   Шаблон:

   ```
   ^https?://[^\s/$.?#].[^\s]*$
   ```
   Пример вебхука: `https://webhook.site/#!/bc8f71ac-c182-4181-9159-6ba6950afffa`

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

3. Чтобы иметь время для реакции на оповещение об обновлении Deckhouse Kubernetes Platform, настройте параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime), как показано на примере ниже. В этом случае обновление случится по прошествии указанного времени с учетом окон обновлений.

   Пример настройки параметра `minimalNotificationTime`:

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

{% alert level="info" %}
Если не указать адрес в параметре [update.notification.webhook](configuration.html#parameters-update-notification-webhook), но указать время в параметре [update.notification.minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime), применение новой версии будет отложено на указанное в параметре `minimalNotificationTime` время. В этом случае оповещением о появлении новой версии ,eltn cxbnfnmcz - появление в кластере ресурса [DeckhouseRelease](cr.html#deckhouserelease), имя которого соответствует новой версии.
{% endalert %}

## Алерты и статусы обновлений

Во время обновления Deckhouse Kubernetes Platform отображаются следующие уведомления:

- алерт `DeckhouseUpdating`;
- статус `Ready` пода `deckhouse`.

> Если под длительное время не переходит в статус `Ready`, значит присутствует проблема в работе Deckhouse Kubernetes Platform и необходима диагностика.

1. Чтобы просмотреть алерт `DeckhouseUpdating`, используйте команду:

   ```shell
   $ kubectl get deckhouserelease
   ```

   Если алерт `DeckhouseUpdating` не отображается, значит обновление завершено.

   Вывод команды проверяет состояние [релизов](modules/002-deckhouse/cr.html#deckhouserelease) Deckhouse Kubernetes Platform:

   ```console
   NAME       PHASE        TRANSITIONTIME   MESSAGE
   v1.46.8    Superseded   13d              
   v1.46.9    Superseded   11d              
   v1.47.0    Superseded   4h12m            
   v1.47.1    Deployed     4h12m            
   ```

2. Чтобы посмотреть статус пода, используйте команду:

   ```shell
   $ kubectl -n d8-system get pods -l app=deckhouse
   ```

   Статус `Deployed` у соответствующей версии говорит о том, что переключение на соответствующую версию было выполнено (но это не значит, что оно закончилось успешно).

   Вывод команды позволит проверить состояние пода Deckhouse Kubernetes Platform:

   ```shell

   NAME                   READY  STATUS   RESTARTS  AGE
   deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
   ```

   * Если статус пода `Running` и в колонке `READY` указано `1/1` — обновление закончилось успешно.
   * Если статус пода `Running` и в колонке `READY` указано `0/1` — обновление еще не закончилось. Если это продолжается более 20–30 минут, это может говорить о наличии проблем в работе Deckhouse Kubernetes Platform. Необходима диагностика.
   * Если статус пода не `Running`, это может говорить о наличии проблем в работе Deckhouse Kubernetes Platform. Необходима диагностика.

{% alert level="info" %}
Если выявились проблемы, выполните следующие шаги:

1. Проверьте логи, используя команду:

   ```shell
   kubectl -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
   ```

2. Соберите [отладочную информацию](modules/002-deckhouse/faq.html#как-собрать-информацию-для-отладки) и свяжитесь с технической поддержкой.
3. Запросите помощи у [сообщества](https://deckhouse.ru/community/about.html).
{% endalert %}
