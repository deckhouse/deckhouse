---
title: "Модуль dynatrace"
---

Данный модуль устанавливает operator [Dynatrace](https://www.dynatrace.com/), а так же создает необходимые CustomResource'ы, описывающие инсталляцию агентов OneAgent. Агенты ставятся latest версии и оператор автоматически их обновляет.

**Важно!** Оператор использует внешний образ `docker.io/dynatrace/oneagent:latest`.

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения добавьте в CM `deckhouse`:

```yaml
data:
  dynatraceEnabled: "true"
```

### Параметры

* `apiURL` — адрес инсталляции Dynatrace;
    * Пример параметра для PaaS инсталляции: `https://leb77264.live.dynatrace.com/api`.
* `skipCertCheck` — если у вас используются самоподписанные сертификаты для api;
    * По-умолчанию `false`.
* `apiToken` — [токен](https://www.dynatrace.com/support/help/reference/dynatrace-concepts/what-is-an-access-token/) с правами `Access problem and event feed, metrics, and nodes` из раздела `Settings`->`Integration`->`Dynatrace API`;
* `paasToken` — токен из раздела `Settings`->`Integration`->`Platform as a Service`;
* `hostGroups` — на какие хосты выкатить OneAgent (используется для разделения хостов по группам в Dynatrace) — если не указать данный параметр, тo OneAgent будет выкачен на все хосты (и в Dynatrace не будет передаваться название группы хостов);
    * `name` – название группы хостов для Dynatrace.
    * `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
        * Необязательный параметр.
    * `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
        * Необязательный параметр.
        * Если ничего не указано – будет использовано значение `{"operator":"Exists"}`.
        * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Параметр влияет исключительно на размещение operator, а не самих OneAgent.
    * Если ничего не указано — будет [использоваться автоматика]({{ site.baseurl }}/#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Параметр влияет исключительно на размещение operator, а не самих OneAgent.
    * Если ничего не указано — будет [использоваться автоматика]({{ site.baseurl }}/#выделение-узлов-под-определенный-вид-нагрузки).
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфигурации


#### Минимальная конфигурация
```yaml
dynatraceEnabled: "true"
dynatrace: |
  apiToken: crM2eyYjvRaqbmz5uC0SRn5
  paasToken: uuaBV1LsSTCOXqgszgDst0n
  apiURL: https://leb77264.live.dynatrace.com/api
```

#### Указана группа хостов для Dynatrace

```yaml
dynatraceEnabled: "true"
dynatrace: |
  apiToken: crM2eyYjvRaqbmz5uC0SRn5
  paasToken: uuaBV1LsSTCOXqgszgDst0n
  apiURL: https://leb77264.live.dynatrace.com/api
  hostGroups:
  - name: south-eash-hosts
```

#### Разные группы хостов
```yaml
dynatraceEnabled: "true"
dynatrace: |
  apiToken: crM2eyYjvRaqbmz5uC0SRn5
  paasToken: uuaBV1LsSTCOXqgszgDst0n
  apiURL: https://leb77264.live.dynatrace.com/api
  hostGroups:
  - name: system
    nodeSelector:
      node-role.flant.com/system: ""
    tolerations:
    - key: dedicated.flant.com
      operator: Equal
      value: system
  - name: production
    nodeSelector:
      node-role/production: ""
    tolerations:
    - key: dedicated
      operator: Equal
      value: production
```
