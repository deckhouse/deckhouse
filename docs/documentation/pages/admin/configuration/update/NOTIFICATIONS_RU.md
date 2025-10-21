---
title: Настройка уведомлений о новых релизах
permalink: ru/admin/configuration/update/notifications.html
description: "Настройка уведомлений об обновлениях в Deckhouse Kubernetes Platform. Настройка алертов, интеграция с внешними системами и автоматизированное управление уведомлениями об обновлениях платформы."
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
Вебхук указывать не обязательно: если [параметр `update.notification.webhook`](/modules/deckhouse/configuration.html#parameters-update-notification-webhook) не задан, но указано время в [параметре `update.notification.minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), применение новой версии всё равно будет отложено на указанный период. В этом случае оповещением о появлении новой версии можно считать появление в кластере ресурса [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) с именем новой версии.
{% endalert %}

После появления новой минорной версии DKP на используемом канале обновлений, но до момента её применения в кластере, на указанный адрес вебхука будет выполнен [POST-запрос](/modules/deckhouse/configuration.html#parameters-update-notification-webhook).

Параметр [minimalNotificationTime](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) позволяет отложить установку обновления на заданный период, обеспечивая время для реакции на оповещение с учётом окон обновлений. Если при этом вебхук недоступен, каждая неудачная попытка отправки будет сдвигать время применения на ту же величину, что может привести к бесконечному откладыванию обновления.

{% alert level="warning" %}
Если ваш вебхук возвращает любой код вне диапазона 2хх, DKP повторяет отправку уведомления до пяти раз с экспоненциальной задержкой между попытками. Если все попытки окажутся неуспешными, выпуск блокируется до восстановления доступности вебхука.
{% endalert %}

Для удобной обработки ошибок и отладки при возврате кодов ошибок вебхук должен возвращать JSON-ответ следующей структуры:

- `code`— необязательный внутренний код ошибки для программной обработки;
- `message`— человекочитаемое описание того, что пошло не так.

Если вебхук возвращает успешный статус HTTP (2xx), DKP считает уведомление успешным вне зависимости от содержимого ответа.

{% offtopic title="Минимальный пример вебхука на Go..." %}

```go
package main
import (
  "encoding/json"
  "fmt"
  "log"
  "net/http"
)
// Payload structure Deckhouse sends in POST body.
type WebhookData struct {
  Subject       string            `json:"subject"`
  Version       string            `json:"version"`
  Requirements  map[string]string `json:"requirements,omitempty"`
  ChangelogLink string            `json:"changelogLink,omitempty"`
  ApplyTime     string            `json:"applyTime,omitempty"`
  Message       string            `json:"message"`
}

// Response structure that Deckhouse expects from webhook on error
type ResponseError struct {
  Code    string `json:"code,omitempty"`
  Message string `json:"message"`
}

func handler(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost {
    w.WriteHeader(http.StatusMethodNotAllowed)
    return
  }
  defer r.Body.Close()

  var data WebhookData
  if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
    log.Printf("failed to decode payload: %v", err)
    w.WriteHeader(http.StatusInternalServerError)
    return
  }

  // Print payload fields
  log.Printf("subject=%s version=%s applyTime=%s changelog=%s requirements=%v",
    data.Subject, data.Version, data.ApplyTime, data.ChangelogLink, data.Requirements)
  log.Printf("message=%s", data.Message)

  // Example conditional logic: fail intentionally for testing
  if data.Version == "v0.0.0-fail" {
    // Return structured error response with error status code
    errorResp := ResponseError{
      Code:    "TEST_FAILURE",
      Message: "intentional failure for testing",
    }

    w.WriteHeader(http.StatusBadRequest)
    json.NewEncoder(w).Encode(errorResp)
    return
  }

  // Return success response with 2xx status code
  w.WriteHeader(http.StatusOK)
  w.Write([]byte("Notification processed successfully"))
}

func main() {
  mux := http.NewServeMux()
  mux.HandleFunc("/webhook", handler)

  addr := ":8080"
  fmt.Printf("listening on %s, POST to http://localhost%s/webhook\n", addr, addr)
  if err := http.ListenAndServe(addr, mux); err != nil {
    log.Fatal(err)
  }
}
```

{% endofftopic %}

### Поддерживаемые параметры

- `update.notification.webhook` — URL-адрес для отправки уведомлений. POST-запрос с информацией об обновлении отправляется сразу после появления новой минорной версии на используемом канале обновлений, но до установки обновления в кластере.
- `update.notification.auth` — параметры аутентификации при вызове вебхука. Если параметр не указан, аутентификация не используется.
  - `update.notification.auth.basic` — базовый вариант аутентификации. Имя пользователя и пароль передаются в заголовке `Authorization` в формате `Basic <base64(username:password)>`.
    - `update.notification.auth.basic.username` — имя пользователя.
    - `update.notification.auth.basic.password` — пароль.
  - `update.notification.auth.bearerToken` — аутентификация с использованием токена. Токен передаётся в заголовке `Authorization` в формате `Bearer <token>`.
- `update.notification.minimalNotificationTime` — минимальный интервал между появлением новой минорной версии на используемом канале обновлений и началом обновления. Указывается в часах и минутах: `30m`, `1h`, `2h30m`, `24h`. Если настроено [окно обновлений](configuration.html#окна-обновлений), то сначала учитывается интервал `minimalNotificationTime`, после чего произойдёт установка обновления, но только в рамках заданного окна.
- `update.notification.tlsSkipVerify` — отключение проверки TLS-сертификата при вызове вебхука (например, если используется самоподписанный сертификат). По умолчанию `false`.

Пример [настройки `update.notification`](/modules/deckhouse/configuration.html#parameters-update-notification) с базовой аутентификацией:

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

Пример [настройки `update.notification`](/modules/deckhouse/configuration.html#parameters-update-notification) без аутентификации:

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
