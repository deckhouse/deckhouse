---
title: Алерты и статусы обновлений компонентов
permalink: ru/update/notifications/procedure/
lang: ru
---

Во время обновления Deckhouse Kubernetes Platform отображаются:

- алерт `DeckhouseUpdating`;
- статус `Ready` пода `deckhouse`. Если под длительное время не переходит в статус `Ready`, значит присутствует проблема в работе Deckhouse Kubernetes Platform и необходима диагностика.

1. Чтобы просмотреть алерт `DeckhouseUpdating`, используйте команду:

   ```shell
   $ kubectl get deckhouserelease
   ```

Если алерт `DeckhouseUpdating` не отображается, значит обновление завершено.

Вывод команды позволяет прверить состояние [релизов](modules/002-deckhouse/cr.html#deckhouserelease) Deckhouse Kubernetes Platform:

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
