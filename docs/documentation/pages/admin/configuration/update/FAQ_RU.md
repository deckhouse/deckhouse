---
title: FAQ
permalink: ru/admin/configuration/update/faq.html
description: "Часто задаваемые вопросы об обновлениях платформы Deckhouse Kubernetes Platform. Устранение неполадок обновлений, конфигурация и лучшие практики обслуживания платформы."
lang: ru
---

## Как применить обновление, минуя окна обновлений, canary-release и ручной режим обновлений?

Чтобы применить обновление DKP немедленно, установите в соответствующем ресурсе [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

{% alert level="warning" %}
В этом случае будут проигнорированы окна обновления, [настройки canary-release](../../../user/network/canary-deployment.html) и [режим ручного обновления кластера](configuration.html#ручное-подтверждение-обновлений). Обновление применится сразу после установки аннотации.
{% endalert %}

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

```shell
d8 k annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Пример ресурса с установленной аннотацией пропуска окон обновлений:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
# ... остальная часть манифеста
```

## Как понять, что в кластере идет обновление?

Во время обновления:

- отображается [алерт `DeckhouseUpdating`](../../../reference/alerts.html#monitoring-deckhouse-deckhouseupdating);
- под `deckhouse` находится не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о наличии проблем в работе DKP. Необходима диагностика.

## Как понять, что обновление прошло успешно?

Если [алерт `DeckhouseUpdating`](../../../reference/alerts.html#monitoring-deckhouse-deckhouseupdating) перестал отображаться, это означает, что обновление завершено.

Также вы можете проверить состояние релизов DKP в кластере, используя следующую команду:

```shell
d8 k get deckhouserelease
```

Пример вывода:

```console
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d
v1.46.9    Superseded   11d
v1.47.0    Superseded   4h12m
v1.47.1    Deployed     4h12m
```

Статус `Deployed` говорит о том, что произошло переключение на соответствующую версию, но не гарантирует успешное завершение обновления.

Для проверки успешности обновления выведите состояние пода `deckhouse`:

```shell
d8 k -n d8-system get pods -l app=deckhouse
```

Пример вывода:

```console
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

- Если под находится в статусе `Running` и в колонке `READY` указано `1/1`, это означает, что обновление завершилось успешно.
- Если под находится в статусе `Running`, но в колонке `READY` указано `0/1`, это означает, что обновление еще не завершено. Если так продолжается более 20–30 минут, это может говорить о наличии проблем в работе DKP. Необходима диагностика.
- Если под не находится в статусе `Running`, это может говорить о наличии проблем в работе DKP. Необходима диагностика.

### Если что-то пошло не так

- Проверьте логи, используя следующую команду:

  ```shell
  d8 k -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
  ```

- Соберите отладочную информацию и свяжитесь с [технической поддержкой DKP](/tech-support/).
- Попросите помощи у [сообщества](/community/).

## Как узнать, что для кластера доступна новая версия DKP?

Как только на установленном в кластере канале обновления появляется новая версия DKP:

- Загорается [алерт `DeckhouseReleaseIsWaitingManualApproval`](../../../reference/alerts.html#monitoring-deckhouse-deckhousereleaseiswaitingmanualapproval), если кластер использует [ручной режим обновлений](configuration.html#ручное-подтверждение-обновлений).
- Появляется новый кастомный ресурс [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease). Используйте команду `d8 k get deckhousereleases`, чтобы посмотреть список релизов. Если `DeckhouseRelease` новой версии находится в состоянии `Pending`, указанная версия еще не установлена. Возможные причины, при которых `DeckhouseRelease` может находиться в `Pending`:
  - Установлен [ручной режим обновлений](configuration.html#ручное-подтверждение-обновлений).
  - Установлен автоматический режим обновлений и настроены [окна обновлений](configuration.html#окна-обновлений), интервал которых еще не наступил.
  - Установлен автоматический режим обновлений, окна обновлений не настроены, но применение версии отложено на случайный период времени из-за механизма снижения нагрузки на репозиторий образов контейнеров. В поле `status.message` ресурса DeckhouseRelease будет соответствующее сообщение.
  - Установлен [параметр `update.notification.minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) и заданный в нем интервал еще не прошел.

## Как заранее получать информацию о предстоящем обновлении?

Получать заранее информацию об обновлении минорных версий DKP на канале обновлений можно следующими способами:

- Настроить [ручной режим обновлений](configuration.html#ручное-подтверждение-обновлений). В этом случае при появлении новой версии на канале обновлений загорится [алерт `DeckhouseReleaseIsWaitingManualApproval`](../../../reference/alerts.html#monitoring-deckhouse-deckhousereleaseiswaitingmanualapproval), и в кластере появится новый [кастомный ресурс DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease).
- Настроить [автоматический режим обновлений](configuration.html#автоматическое-обновление) и указать минимальное время в [параметре `minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый [кастомный ресурс DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease). Если указать URL в параметре [`update.notification.webhook`](/modules/deckhouse/configuration.html#parameters-update-notification-webhook), через указанный вебхук будет отправлено уведомление об предстоящем обновлении.

## Как узнать версию DKP на каждом из каналов обновлений?

Актуальная информация о версиях DKP на разных каналах обновлений доступна на сайте [releases.deckhouse.ru](https://releases.deckhouse.ru).

## Что делать, если DKP не получает обновления из настроенного канала?

- Проверьте, что настроен [нужный канал обновлений](configuration.html#проверка-текущего-канала-обновлений).
- Проверьте корректность разрешения DNS-имени хранилища образов Deckhouse.

  Получите и сравните IP-адреса хранилища образов Deckhouse (`registry.deckhouse.ru`) на одном из узлов и в поде `deckhouse`. Они должны совпадать.

  Пример получения IP-адреса хранилища образов Deckhouse на узле:

  ```shell
  getent ahosts registry.deckhouse.ru
  ```

  Пример вывода:

  ```console
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Пример получения IP-адреса хранилища образов Deckhouse в поде `deckhouse`:

  ```shell
  d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.ru
  ```

  Пример вывода:

  ```console
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM  registry.deckhouse.ru
  ```

  Если полученные IP-адреса не совпадают, проверьте настройки DNS на узле. В частности, обратите внимание на список доменов в параметре `search` файла `/etc/resolv.conf` (он влияет на разрешение имен в поде `deckhouse`). Если в параметре `search` файла `/etc/resolv.conf` указан домен, в котором настроено разрешение wildcard-записей, это может привести к неверному разрешению IP-адреса хранилища образов Deckhouse (см. пример).

{% offtopic title="Пример настроек DNS, которые могут привести к ошибкам в разрешении IP-адреса хранилища образов Deckhouse..." %}

Далее описан пример, когда настройки DNS приводят к различному результату при разрешении имен на узле и в поде Kubernetes:

- Пример файла `/etc/resolv.conf` на узле:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > Обратите внимание, что по умолчанию на узле параметр `ndot` равен **1** (`options ndots:1`). Но в подах Kubernetes параметр `ndot` равен **5**. Таким образом, логика разрешения DNS-имен, имеющих в имени 5 точек и менее, различается на узле и в поде.

- В DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. То есть любое DNS-имя в зоне `company.my`, для которого нет конкретной записи в DNS, разрешается в адрес `10.0.0.100`.

Тогда с учетом параметра `search`, указанного в файле `/etc/resolv.conf`, при обращении на адрес `registry.deckhouse.ru` на узле система попробует получить IP-адрес для имени `registry.deckhouse.ru` (так как считает его полностью определенным, учитывая настройку по умолчанию параметра `options ndots:1`).

При обращении же на адрес `registry.deckhouse.ru` **из пода** Kubernetes, учитывая параметры `options ndots:5` (используется в Kubernetes по умолчанию) и `search`, система первоначально попробует получить IP-адрес для имени `registry.deckhouse.ru.company.my`. Имя `registry.deckhouse.ru.company.my` разрешится в IP-адрес `10.0.0.100`, так как в DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. В результате к хосту `registry.deckhouse.ru` будет невозможно подключиться и скачать информацию о доступных обновлениях Deckhouse.
{% endofftopic %}
