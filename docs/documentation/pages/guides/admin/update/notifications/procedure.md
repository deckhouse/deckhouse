---
title: Где посмотреть статус установки минорных обновлений?
permalink: ru/notifications/procedure/
lang: ru
---

### Уведомление о процедуре обновления в кластере

Получать заранее информацию об обновлении минорных версий Deckhouse на канале обновлений можно следующими способами:
- Настроить ручной [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode). В этом случае при появлении новой версии на канале обновлений загорится алерт `DeckhouseReleaseIsWaitingManualApproval` и в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease).
- Настроить автоматический [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode) и указать минимальное время в параметре [minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease). А если указать URL в параметре [update.notification.webhook](modules/002-deckhouse/configuration.html#parameters-update-notification-webhook), дополнительно произойдет вызов webhook'а.

Во время обновления:
- горит алерт `DeckhouseUpdating`;
- под `deckhouse` не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

Если алерт `DeckhouseUpdating` погас, значит, обновление завершено.

Вы также можете проверить состояние [релизов](modules/002-deckhouse/cr.html#deckhouserelease) Deckhouse.

Пример:

```console
$ kubectl get deckhouserelease
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d              
v1.46.9    Superseded   11d              
v1.47.0    Superseded   4h12m            
v1.47.1    Deployed     4h12m            
```

Статус `Deployed` у соответствующей версии говорит о том, что переключение на соответствующую версию было выполнено (но это не значит, что оно закончилось успешно).

Проверьте состояние пода Deckhouse Kubernetes Platform:

```shell
$ kubectl -n d8-system get pods -l app=deckhouse
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

* Если статус пода `Running` и в колонке READY указано `1/1` — обновление закончилось успешно.
* Если статус пода `Running` и в колонке READY указано `0/1` — обновление еще не закончилось. Если это продолжается более 20–30 минут, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.
* Если статус пода не `Running`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

{% alert level="info" %}
Возможные варианты действий, если что-то пошло не так:
- Проверьте логи, используя следующую команду:

  ```shell
  kubectl -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
  ```

- Соберите [отладочную информацию](modules/002-deckhouse/faq.html#как-собрать-информацию-для-отладки) и свяжитесь с технической поддержкой.
- Попросите помощи у [сообщества](https://deckhouse.ru/community/about.html).
{% endalert %}
