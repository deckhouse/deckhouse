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

DKP публикует собственные веб-интерфейсы по именам хостов, которые формируются из глобального параметра [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate): API, веб-интерфейс, Grafana, Dex, генератор kubeconfig и другие.
Эти имена зарезервированы в пределах кластера.
Ресурсы Ingress, HTTPRoute, GRPCRoute, TLSRoute, ListenerSet и Gateway, созданные вне неймспейса с лейблом `heritage: deckhouse`, занять такое имя не могут — запрос отклоняется с сообщением, в котором указано имя хоста:

```console
Hostname console.example.com is reserved for Deckhouse platform services
and cannot be claimed outside a system namespace
```

Проверка выполняется только при создании или изменении ресурсов.
Если существующий ресурс уже использует зарезервированное имя хоста, он продолжает обслуживать трафик до следующего изменения.

### Состав резервирования

Состав резервирования задаётся параметром `settings.reservedPublicHosts.mode` ресурса ModuleConfig `deckhouse`:

- `Template` (по умолчанию) — резервируются все имена хостов, которые способен сформировать шаблон.
- `List` — резервируются только имена хостов сервисов, о публикации которых DKP знает.

Подробнее о каждом режиме резервирования — [в описании параметра](configuration.html#parameters-reservedpublichosts-mode).

### Исключение из резервирования

В режиме `Template` резервируются не только имена хостов, используемые платформой, но и другие имена, соответствующие шаблону. Если приложению необходимо такое имя, его можно исключить из резервирования одним из следующих способов:

- Освободить одно имя хоста с помощью [параметра `settings.reservedPublicHosts.excludedServices`](configuration.html#parameters-reservedpublichosts-excludedservices).

  В параметре указывается не полное имя хоста, а имя сервиса (подставляемое в шаблон значение). Например, для шаблона `%s.example.com` значение `grafana` исключает из резервирования имя `grafana.example.com`.

- Исключить весь неймспейс с помощью лейбла `security.deckhouse.io/reserved-hosts-bypass: "true"`.

  Лейбл устанавливается на неймспейс, а не на объект, поэтому пользователь, которому разрешено управлять ресурсами только в выделенных ему неймспейсах, установить его не сможет.

- Вернуться к прежнему составу резервирования, задав режим `mode: List`.

Пример конфигурации, в котором `grafana.example.com` исключается из резервирования и дополнительно резервируется `admin.corp.example.org`, которое не соответствует шаблону:

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

[Параметр `settings.reservedPublicHosts.additionalHosts`](configuration.html#parameters-reservedpublichosts-additionalhosts) позволяет зарезервировать дополнительные имена хостов независимо от значения `excludedServices`.

{% alert level="warning" %}
Параметр `excludedServices` принимает только имя сервиса, поэтому с его помощью нельзя исключить из резервирования wildcard-имя.

Чтобы приложение могло использовать wildcard-имя, например, `*.example.com`, отключите проверку для его неймспейса с помощью упомянутого ранее лейбла или используйте режим `List`.
{% endalert %}

### Исключение для общего Gateway

Gateway, к которому подключаются ресурсы ListenerSet модулей DKP, автоматически исключается из резервирования независимо от того, какие имена заявлены в его listeners.

Таким Gateway считается:

- объект, указанный в глобальном параметре [`gatewayAPIGateway`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-gatewayapigateway);
- если параметр не задан — Gateway, указанный в ConfigMap `default-gateway` неймспейса `d8-alb`.

Исключение действует по имени и неймспейсу и только для ресурсов Gateway. Оно не распространяется на другие ресурсы, использующие те же имена хостов, например, Ingress, HTTPRoute или другой Gateway.

Исключение необходимо, потому что такой Gateway обычно содержит listener с wildcard-именем и wildcard-сертификатом и зачастую находится в собственном неймспейсе без лейбла `heritage: deckhouse`.
Без его исключения было бы невозможно изменить такой Gateway после включения резервирования, хотя от него продолжают зависеть подключённые маршруты DKP.

Любой другой Gateway с listener `*.example.com` отклоняется, и запись о том, что уже обслуживалось, его не покрывает.
Чтобы использовать такой Gateway, добавьте его неймспейсу лейбл `security.deckhouse.io/reserved-hosts-bypass: "true"` или включите режим `List`.

Если общий Gateway не указан в параметре `gatewayAPIGateway` и не определён через ConfigMap `default-gateway`, автоматическое исключение не применяется.

### Резервирование при наличии работающих приложений в кластере

Когда резервирование начинает действовать, имена хостов, уже используемые приложениями, автоматически добавляются в список разрешённых.
Без этого их следующее изменение было бы отклонено.

Список хранится в ключе `grandfatheredHosts` объекта ConfigMap `d8-reserved-public-hosts` в неймспейсе `d8-system` отдельно от имён, исключённых из резервирования вручную.

Список формируется только при первом включении режима `Template` и впоследствии не обновляется. Это предотвращает обход резервирования: приложение не может занять зарезервированное имя хоста после включения механизма и затем получить разрешение на его использование при повторном формировании списка.

Если приложение, использовавшее разрешённое таким образом имя хоста, удалено и имя больше не требуется, удалите соответствующую запись из `grandfatheredHosts`.

Перед сохранением имена хостов преобразуются в нижний регистр, а точка в конце имени удаляется. Некорректные значения, например URL вместо имени хоста, игнорируются. Если ресурс использует соответствующее зарезервированное имя, запрос на его следующее изменение будет отклонён.

Wildcard-имена (например, `*.example.com`), в `grandfatheredHosts` не добавляются. Иначе приложение могло бы использовать все имена хостов, соответствующие шаблону, в том числе имена сервисов, которые DKP начнёт публиковать в дальнейшем.
Чтобы приложение могло использовать wildcard-имя домена платформы, добавьте его неймспейсу лейбл `security.deckhouse.io/reserved-hosts-bypass: "true"` или включите режим `List`.

Это ограничение не распространяется на Gateway, к которому подключается DKP. Он автоматически исключается из проверки.

Переключение из режима `Template` в `List` и обратно не приводит к повторному формированию `grandfatheredHosts`. Поэтому имена хостов, которые приложения начали использовать в режиме `List`, после возврата в режим `Template` не добавляются в список разрешённых и при следующем изменении соответствующих ресурсов будут проверяться по общим правилам резервирования.

### Проверка состояния резервирования

Информация о текущем состоянии резервирования хранится в ConfigMap `d8-reserved-public-hosts` неймспейса `d8-system`.

Чтобы просмотреть ConfigMap, выполните следующую команду:

```shell
d8 k -n d8-system get configmap d8-reserved-public-hosts -o yaml
```

ConfigMap содержит следующие ключи:

| Ключ | Описание |
| --- | --- |
| `mode` | Действующий состав резервирования. Если значение `publicDomainTemplate` не соответствует схеме, указывается `List` |
| `hostPattern` | Регулярное выражение, описывающее имена хостов, которые способен сформировать шаблон. В режиме `List` значение будет пустым |
| `hosts` | Имена хостов, для которых используется точное сопоставление: значения `additionalHosts` и wildcard-имя домена |
| `allowedHosts` | Имена хостов, исключённые из проверки по `hostPattern`: имена из `excludedServices` и `grandfatheredHosts` |
| `excludedHosts` | Имена хостов, исключённые из резервирования через `excludedServices` |
| `grandfatheredHosts` | Имена хостов, которые использовались приложениями при включении резервирования и были автоматически добавлены в список разрешённых |
| `sharedGateway` | Gateway, к которому подключается DKP, в формате `<NAMESPACE>/<NAME>`. Если такой Gateway не задан и не обнаружен, значение будет пустым |
| `platformHosts` | Имена хостов сервисов, публикуемых DKP. Используется как справочный ключ для обоих режимов |
| `unknownExcludedServices` | Имена из `excludedServices`, которые не публикует ни один модуль. Наличие имени скорее всего указывает на опечатку в имени сервиса |

## Сбор информации для отладки

О сборе отладочной информации читайте [в FAQ](faq.html#как-собрать-информацию-для-отладки).
