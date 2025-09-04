---
title: Настройка уведомлений о новых релизах
permalink: ru/admin/configuration/update/notifications.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) генерирует [алерты](#алерты-в-системе-мониторинга) в систему мониторинга и может автоматически отправлять уведомления о будущих минорных обновлениях во внешние системы.
Это помогает заранее планировать обновления и подготовиться к ним.

Условия отправки уведомлений во внешние системы:

- для DKP установлен [автоматический режим обновления](configuration.html#автоматическое-обновление);
- планируется смена минорной версии (уведомления о патч-версиях не отправляются);
- настроен вебхук для уведомлений.

## Алерты в системе мониторинга

Если обновление требует внесения изменений в кластер (например, обновление версии Kubernetes или ОС), DKP сгенерирует специальные предупреждения. Среди них:

- **D8NodeHasDeprecatedOSVersion** — в кластере обнаружены узлы с неподдерживаемой версией ОС;
- **HelmReleasesHasResourcesWithDeprecatedVersions** — в некоторых Helm-релизах используются устаревшие ресурсы;
- **KubernetesVersionEndOfLife** — установленная версия Kubernetes более не поддерживается.

При появлении данных предупреждений обязательно устраните их причины перед обновлением. Это поможет избежать сбоев в работе кластера и обеспечить его стабильность после обновления.

## Настройка уведомлений

В режиме обновлений `Auto` можно [настроить](/modules/deckhouse/configuration.html#parameters-update-notification) вызов вебхука для получения оповещения о предстоящем обновлении минорной версии DKP.

Кроме того, оповещения формируются не только при обновлении DKP, но и при обновлении любых модулей, включая их отдельные обновления.
В отдельных случаях система может инициировать отправку нескольких оповещений одновременно (по 10–20 оповещений) с интервалом около 15 секунд.

{% alert %}
Оповещения доступны только в режиме обновлений `Auto`, в режиме `Manual` они не формируются.
{% endalert %}

{% alert %}
Вебхук указывать не обязательно: если параметр `update.notification.webhook` не задан, но указано время в параметре `update.notification.minimalNotificationTime`, применение новой версии всё равно будет отложено на указанный период. В этом случае оповещением о появлении новой версии можно считать появление в кластере ресурса [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) с именем новой версии.
{% endalert %}

После появления новой минорной версии DKP на используемом канале обновлений, но до момента её применения в кластере, на указанный адрес вебхука будет выполнен [POST-запрос](/modules/deckhouse/configuration.html#parameters-update-notification-webhook).

Параметр [minimalNotificationTime](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) позволяет отложить установку обновления на заданный период, обеспечивая время для реакции на оповещение с учётом окон обновлений. Если при этом вебхук недоступен, каждая неудачная попытка отправки будет сдвигать время применения на ту же величину, что может привести к бесконечному откладыванию обновления.

### Поддерживаемые параметры

- `update.notification.webhook` — URL-адрес для отправки уведомлений. POST-запрос с информацией об обновлении отправляется сразу после появления новой минорной версии на используемом канале обновлений, но до установки обновления в кластере.
- `update.notification.auth` — параметры аутентификации при вызове вебхука. Если параметр не указан, аутентификация не используется.
  - `update.notification.auth.basic` — базовый вариант аутентификации. Имя пользователя и пароль передаются в заголовке `Authorization` в формате `Basic <base64(username:password)>`.
    - `update.notification.auth.basic.username` — имя пользователя.
    - `update.notification.auth.basic.password` — пароль.
  - `update.notification.auth.bearerToken` — аутентификация с использованием токена. Токен передаётся в заголовке `Authorization` в формате `Bearer <token>`.
- `update.notification.minimalNotificationTime` — минимальный интервал между появлением новой минорной версии на используемом канале обновлений и началом обновления. Указывается в часах и минутах: `30m`, `1h`, `2h30m`, `24h`. Если настроено [окно обновлений](configuration.html#окна-обновлений), то сначала учитывается интервал `minimalNotificationTime`, после чего произойдёт установка обновления, но только в рамках заданного окна.
- `update.notification.tlsSkipVerify` — отключение проверки TLS-сертификата при вызове вебхука (например, если используется самоподписанный сертификат). По умолчанию `false`.

Пример настройки `update.notification` с базовой аутентификацией:

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

Пример настройки `update.notification` без аутентификации:

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

## Формат уведомлений

При выполнении условий отправки уведомления
DKP отправляет на указанный вебхук POST-запрос с заголовком `Content-Type: application/json`.

Пример тела запроса:

```json
{
  "version": "1.68",
  "requirements":  {"k8s": "1.29.0"},
  "changelogLink": "https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.68.md",
  "applyTime": "2025-02-01T14:30:00Z00:00",
  "message": "New Deckhouse Release 1.68 is available. Release will be applied at: Wednesday, 05-Feb-25 14:30:00 UTC"
}
```

Описание полей в формате уведомлений:

- `version` — номер минорной версии (строка);
- `requirements` — объект с требованиями к новой версии (например, минимальная версия Kubernetes);
- `changelogLink` — ссылка на changelog с описанием изменений в новой минорной версии;
- `applyTime` — дата и время планируемого обновления в формате RFC3339. Учитывает заданные окна обновлений;
- `message` — короткое текстовое уведомление о доступной минорной версии и запланированном времени её установки.
