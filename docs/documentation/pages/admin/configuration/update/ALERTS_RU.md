---
title: Настройка автоматических оповещений
permalink: ru/admin/configuration/update/alerts.html
lang: ru
---

Deckhouse Kubernetes Platform может автоматически отправлять уведомления о будущих минорных обновлениях. Такие уведомления помогают заранее планировать обновления и подготовиться к ним.

Условия для отправки уведомлений:
- Включен автоматический режим обновлений (подробнее).
- Планируется смена минорной версии (уведомления при смене patch-версии не рассылаются).
- Автоматические оповещения настроены через webhook.

## Настройка параметров

Все параметры указываются в разделе `update.notification` модуля Deckhouse.
- `update.notification.webhook` - URL-адрес для отправки уведомлений. POST-запрос с информацией об обновлении отправляется сразу после появления новой минорной версии на текущем канале обновлений, но до установки обновления в кластере.
- `update.notification.auth` - настройка авторизации при вызове webhook. Если не указано, авторизация не используется.
  * `update.notification.auth.basic` - basic-авторизация. Имя пользователя и пароль передаются в заголовке `Authorization` в `Basic <base64(username:password)>`.
    * `update.notification.auth.basic.username` - имя пользователя.
    * `update.notification.auth.basic.password` - пароль.
  * `update.notification.auth.bearerToken` - токен для авторизации. Токен передаётся в заголовке `Authorization` в формате `Bearer <token>`.
- `update.notification.minimalNotificationTime` - минимальное время между моментом отправки уведомления и началом обновления. В течение этого периода платформа не будет обновляться. Формат задаётся в часах и минутах (например, 30m, 1h, 2h30m, 24h).Если настроено окно обновления, то сначала учитывается интервал `minimalNotificationTime`, а после его окончания установка обновления произойдёт только в рамках заданного окна.
- `update.notification.tlsSkipVerify` - `false` по умолчанию. При значении `true` отключается проверка TLS-сертификата при запросе к webhook (например, если используется самоподписанный сертификат).

Пример настройки `update.notification` с basic-авторизацией:

```yaml
update:
  notification:
    webhook: https://release-webhook.mydomain.com
    minimalNotificationTime: 4h
    auth:
      basic:
        username: myusername
        password: mypassword
    tlsSkipVerify: true
```
## Формат уведомления

Когда условие для отправки уведомления выполнено, Deckhouse Kubernetes Platform отправляет на указанный webhook POST-запрос с заголовком `Content-Type: application/json`. Пример тела запроса:

```yaml
{
  "version": "1.68",
  "requirements":  {"k8s": "1.29.0"},
  "changelogLink": "https://github.com/deckhouse/deckhouse/changelog/1.68.md",
  "applyTime": "2025-02-01T14:30:00Z00:00",
  "message": "New Deckhouse Release 1.68 is available. Release will be applied at: Wednesday, 05-Feb-25 14:30:00 UTC"
}
```

Поля JSON-запроса
- `version` — номер минорной версии (строка).
- `requirements` — объект с требованиями к новой версии (например, минимальная версия Kubernetes).
- `changelogLink` — ссылка на список изменений (changelog) для новой минорной версии.
- `applyTime` — дата и время планируемого обновления в формате RFC3339. Учитывает заданные окна обновления.
- `message` — короткое текстовое уведомление о доступной минорной версии и времени её планируемого применения.

Используйте эти настройки, чтобы своевременно получать информацию о грядущих минорных релизах, планировать обслуживание кластера и контролировать процесс обновления.
