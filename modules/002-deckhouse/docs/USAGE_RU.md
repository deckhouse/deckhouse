---
title: "Модуль deckhouse: примеры конфигурации"
search: "release.deckhouse.io/approved"
---


## Настройка режима обновления

Управлять обновлением DKP можно следующими способами:

- с помощью [параметра `settings.update`](configuration.html#parameters-update) ресурса ModuleConfig `deckhouse`;
- с помощью [секции параметров `disruptions`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions) ресурса NodeGroup.

### Конфигурация окон обновлений

Управлять временными окнами, когда Deckhouse будет устанавливать обновления автоматически, можно следующими способами:

- для общего управления обновлениями — в [параметре `update.windows`](configuration.html#parameters-update-windows) ресурса ModuleConfig `deckhouse`;
- для управления обновлениями, которые могут привести к кратковременному простою в работе системных компонентов — в параметрах [`disruptions.automatic.windows`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) и [`disruptions.rollingUpdate.windows`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-rollingupdate-windows) ресурса NodeGroup.

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
  d8 k patch DeckhouseRelease <VERSION> --type=merge -p='{"approved": true}'
  ```

- Если для какой-либо группы узлов отключено автоматическое применение обновлений, которые могут привести к кратковременному простою в работе системных компонентов.

  Это значит, что у NodeGroup, соответствующего группе узлов, [параметр `spec.disruptions.approvalMode`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) установлен в `Manual`.

  Для обновления **каждого** узла в такой группе на узел нужно установить аннотацию `update.node.deckhouse.io/disruption-approved=`.
  Пример:

  ```shell
  d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

### Оповещение об обновлении Deckhouse

В режиме обновлений `Auto` можно [настроить](configuration.html#parameters-update-notification) вызов вебхука для получения оповещения о предстоящем обновлении минорной версии Deckhouse.

Кроме того, оповещения формируются не только при обновлении Deckhouse, но и при обновлении любых модулей, включая их отдельные обновления.
В отдельных случаях система может инициировать отправку нескольких оповещений одновременно (по 10–20 оповещений) с интервалом около 15 секунд.

{% alert %}
Оповещения доступны только в режиме обновлений `Auto`, в режиме `Manual` они не формируются.
{% endalert %}

{% alert %}
Вебхук указывать не обязательно: если параметр `update.notification.webhook` не задан, но указано время в параметре `update.notification.minimalNotificationTime`, применение новой версии всё равно будет отложено на указанный период. В этом случае оповещением о появлении новой версии можно считать появление в кластере ресурса [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) с именем новой версии.
{% endalert %}

Оповещения отправляются только один раз для конкретного обновления. Если что-то пошло не так (например, вебхук получил некорректные данные), повторная отправка автоматически не произойдёт. Чтобы отправить оповещение повторно, необходимо удалить соответствующий ресурс [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease).

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

После появления новой минорной версии DKP на используемом канале обновлений, но до момента применения ее в кластере на адрес вебхука будет выполнен [POST-запрос](configuration.html#parameters-update-notification-webhook).

Параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime) позволяет отложить установку обновления на заданный период, обеспечивая время для реакции на оповещение с учётом окон обновлений. Если при этом вебхук недоступен, каждая неудачная попытка отправки будет сдвигать время применения на ту же величину, что может привести к бесконечному откладыванию обновления.

{% alert level="warning" %}
Если ваш вебхук возвращает любой код вне диапазона 2хх, DKP повторяет отправку уведомления до пяти раз с экспоненциальной задержкой между попытками. Если все попытки окажутся неуспешными, выпуск блокируется до восстановления доступности вебхука.
{% endalert %}

Для удобной обработки ошибок и отладки при возврате кодов ошибок вебхук должен возвращать JSON-ответ следующей структуры:

- `code`— необязательный внутренний код ошибки для программной обработки;
- `message`— человекочитаемое описание того, что пошло не так.

Если вебхук возвращает успешный статус HTTP (2xx), DKP считает уведомление успешным вне зависимости от содержимого ответа.

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

## Резервирование имён хостов веб-интерфейсов платформы

Deckhouse публикует собственные веб-интерфейсы по именам хостов, которые формируются из глобального параметра [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate): API, веб-интерфейс, Grafana, Dex, генератор kubeconfig и другие.
Эти имена зарезервированы в пределах кластера.
Ресурсы Ingress, HTTPRoute, GRPCRoute, TLSRoute, ListenerSet и Gateway, созданные вне неймспейса с лейблом `heritage: deckhouse`, занять такое имя не могут — запрос отклоняется с сообщением, в котором указано имя хоста:

```console
Hostname console.example.com is reserved for Deckhouse platform services
and cannot be claimed outside a system namespace
```

Проверяются только создание и изменение.
Объект, который уже занимает зарезервированное имя, продолжает обслуживать трафик, пока его не изменят.

### Что зарезервировано

Состав резервирования задаётся [параметром `settings.reservedPublicHosts.mode`](configuration.html#parameters-reservedpublichosts-mode) ресурса ModuleConfig `deckhouse`:

- `Template` (по умолчанию) — все имена хостов, которые способен сформировать шаблон.
  Для шаблона `%s.example.com` это любое имя из одной метки под `example.com`: и `console.example.com`, который платформа публикует, и `shop.example.com`, который пока не обслуживает ни один модуль.
  Модулю не нужно просить о включении в резервирование, а имя защищено раньше, чем его начнут обслуживать.
- `List` — только имена хостов тех сервисов, о публикации которых Deckhouse знает.
  Резервирование уже, но публичный домен, который Deckhouse начнёт публиковать позже, остаётся вне резервирования, пока список не обновят.

В режиме `Template` резервируется и wildcard-форма домена, `*.example.com`: приложение, занявшее её, перекрыло бы сразу все имена платформы.
Wildcard резервируется только там, где `%s` занимает первую метку целиком: для шаблона `kube-%s.example.com` не резервируются ни `kube-*.example.com`, ни `*.example.com`.

Имя на одну метку глубже резервирование не затрагивает, поэтому приложения экосистемы вне его области: глобальный параметр [`applications.publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-applications-publicdomaintemplate) размещает имя приложения и его неймспейс в двух разных метках.

### Как вернуть имя хоста приложению

В режиме `Template` резервирование охватывает больше, чем платформа обслуживает, поэтому приложение, которому такое имя нужно обоснованно, следует указать явно.
Сделать это можно тремя способами:

1. Освободить одно имя хоста [параметром `settings.reservedPublicHosts.excludedServices`](configuration.html#parameters-reservedpublichosts-excludedservices).
   Он принимает имя сервиса — ту часть, которую подставляет шаблон, — а не полное имя хоста: значение `grafana` освобождает `grafana.example.com`.
1. Исключить весь неймспейс лейблом `security.deckhouse.io/reserved-hosts-bypass: "true"`.
   Лейбл устанавливается на неймспейс, а не на объект, поэтому арендатор, ограниченный своими неймспейсами, установить его не сможет.
1. Вернуться к прежнему составу резервирования, задав `mode: List`.

Пример, который освобождает имя хоста Grafana и дополнительно резервирует имя, недостижимое для шаблона:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    reservedPublicHosts:
      mode: Template
      excludedServices:
        - grafana
      additionalHosts:
        - admin.corp.example.org
```

[Параметр `settings.reservedPublicHosts.additionalHosts`](configuration.html#parameters-reservedpublichosts-additionalhosts) резервирует всегда, независимо от значения `excludedServices`, поэтому противоречить друг другу эти параметры не могут.

{% alert level="warning" %}
Wildcard-форму нельзя освободить через `excludedServices`, который принимает имя сервиса.
Приложению, обслуживающему `*.example.com`, требуется лейбл на неймспейсе или режим `List`.
{% endalert %}

### Включение резервирования в кластере с работающими приложениями

Когда резервирование начинает действовать, имена хостов, которые приложения уже обслуживают, однократно записываются и остаются разрешёнными.
Без этого их следующее изменение было бы отклонено.

Запись хранится в ConfigMap `d8-reserved-public-hosts` неймспейса `d8-system`, в ключе `grandfatheredHosts`, отдельно от имён, освобождённых вручную.
Снимок делается один раз — на первой конвергенции, где действует режим `Template`, — и повторно не снимается: иначе резервирование можно было бы обойти, заняв имя и дождавшись, пока следующий снимок его узаконит.

Чтобы перестать разрешать записанное имя, когда стоящее за ним приложение удалено, уберите запись из этого ключа.
Запись приводится к тому виду, в котором её сравнивает резервирование — строчными буквами и без точки на конце, — поэтому ручная правка ключа не может помешать модулю сойтись.
Запись, которая и после этого не является именем хоста, например вставленный URL, отбрасывается, а занимающий это имя объект отклоняется при следующем изменении.

Wildcard-форма — исключение: она не записывается, поскольку иначе все имена, которые формирует шаблон, остались бы перекрыты, включая те, что Deckhouse начнёт публиковать позже.
Приложению, обслуживающему wildcard над доменом платформы, нужен лейбл на неймспейсе или режим `List`, установленные до обновления.

### Как узнать, что зарезервировано и почему

Тот же ConfigMap отвечает, что именно охватывает резервирование в этом кластере:

| Ключ | Содержимое |
| --- | --- |
| `mode` | Действующий состав резервирования. Показывает `List`, если значение `publicDomainTemplate` не проходило проверку по схеме |
| `hostPattern` | Множество имён хостов, которые способен сформировать шаблон, в виде регулярного выражения. В режиме `List` пустой |
| `hosts` | Имена хостов, сравниваемые точно: значения `additionalHosts` и wildcard-форма |
| `allowedHosts` | Что разрешено обратно из-под регулярного выражения: имена, освобождённые через `excludedServices`, и записанные имена |
| `excludedHosts` | Имена хостов, освобождённые через `excludedServices` |
| `grandfatheredHosts` | Имена хостов, записанные в момент, когда резервирование начало действовать |
| `platformHosts` | Имена хостов сервисов, которые Deckhouse публикует сегодня. Справочный ключ для обоих режимов |
| `unknownExcludedServices` | Имена из `excludedServices`, которые не публикует ни один модуль. Место, куда смотреть, если имя хоста осталось зарезервированным: скорее всего, в имени опечатка |

Чтобы прочитать его, выполните:

```shell
d8 k -n d8-system get configmap d8-reserved-public-hosts -o yaml
```

## Сбор информации для отладки

О сборе отладочной информации читайте [в FAQ](faq.html#как-собрать-информацию-для-отладки).
