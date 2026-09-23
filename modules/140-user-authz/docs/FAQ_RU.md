---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

<div style="height: 0;" id="как-ограничить-права-пользователю-конкретными-пространствами-имён-устаревшая-ролевая-модель"></div>

## Как ограничить права пользователю конкретными неймспейсами?

Чтобы ограничить права пользователя конкретными неймспейсами в гранулярной ролевой модели, используйте в RoleBinding [namespace-роль](./#namespace-роли) с соответствующим уровнем доступа. [Пример](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-неймспейса).

В упрощённой ролевой модели используйте параметры `namespaceSelector` или `limitNamespaces` (устарел) в кастомном ресурсе [ClusterAuthorizationRule](cr.html#clusterauthorizationrule).

## Что, если два ClusterAuthorizationRules подходят для одного пользователя?

В примере пользователь `jane.doe@example.com` состоит в группе `administrators`. Созданы два ClusterAuthorizationRules:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: jane
spec:
  subjects:
    - kind: User
      name: jane.doe@example.com
  accessLevel: User
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: Group
    name: administrators
  accessLevel: ClusterAdmin
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: env
        operator: In
        values:
        - prod
        - stage
```

1. `jane.doe@example.com` имеет право запрашивать и просматривать объекты среди всех пространств имён, помеченных `env=review`.
2. `Administrators` могут запрашивать, редактировать, получать и удалять объекты на уровне кластера и из пространств имён, помеченных `env=prod` и `env=stage`.

Так как для `Jane Doe` подходят два правила, итоговые права определяются следующим образом:

* `Jane Doe` будет иметь максимальный accessLevel среди всех подходящих правил — `ClusterAdmin`.
* Опции `namespaceSelector` будут объединены так, что `Jane Doe` будет иметь доступ в пространства имён, помеченные лейблом `env` со значением `review`, `stage` или `prod`.

{% alert level="warning" %}
Если есть правило без опции `namespaceSelector` и без опции `limitNamespaces` (устаревшая), это значит, что доступ разрешён во все пространства имён, кроме системных, что повлияет на результат вычисления доступных пространств имён для пользователя.
{% endalert %}

## Можно ли использовать упрощённую и гранулярную ролевые модели одновременно?

Да. Обе модели в итоге сводятся к стандартному механизму RBAC Kubernetes, а RBAC — разрешающая модель: права из всех источников **суммируются**. Если действие разрешено хотя бы одним источником — ClusterAuthorizationRule, AuthorizationRule, RoleBinding на роль гранулярной модели или ProjectRoleBinding, — оно будет разрешено. Ничего специально «переключать» не нужно: можно оставить существующие ClusterAuthorizationRule и постепенно добавлять привязки ролей гранулярной модели.

Это верно и в режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), с одним правилом об уровнях. Ограничение неймспейсов в ClusterAuthorizationRule (`limitNamespaces` или `namespaceSelector`) ограничивает права только этого правила: в неймспейсе вне списка пользователь сохраняет то, что ему дают RoleBinding, AuthorizationRule или ProjectRoleBinding, на уровне этой привязки — `accessLevel` из ClusterAuthorizationRule туда не распространяется. Неймспейс, в котором у пользователя нет ни одного источника прав, остаётся недоступным. Полный список источников и пример — [в описании модуля](./#совместное-использование-clusterauthorizationrule-authorizationrule-и-rbac).

## Как обновление на релиз с контроллером влияет на биндинги?

Ничего не создаётся заново, доступ не прерывается. Компонент `user-authz-controller` принимает под управление биндинги, которые раньше создавал Helm-чарт: те же объекты с теми же именами, к ним добавляется `ownerReference` на правило.

Спланируйте релиз, в котором выполняется миграция: он один раз обрабатывает все существующие биндинги, поэтому идёт дольше обычного. На последующие релизы это не влияет.

| Биндинги в релизе | Чего ожидать |
|---|---|
| До 5000 | Релиз идёт дольше обычного и завершается сам |
| Больше 5000 | Релиз может не уложиться в 20-минутный таймаут модуля и завершиться с ошибкой |

Посчитайте биндинги перед обновлением. Для получения количества кластерных биндингов используйте команду:

```bash
d8 k get clusterrolebindings -l heritage=deckhouse,module=user-authz --no-headers | wc -l
```

Для получения количества namespaced-биндингов используйте команду:

```bash
d8 k get rolebindings -A -l heritage=deckhouse,module=user-authz --no-headers | wc -l
```

Если их больше 5000, на время обновления увеличьте таймаут релиза или переключитесь на предыдущий движок раскатки, а после обновления верните настройку:

Чтобы увеличить таймаут релиза, используйте команду:

```bash
d8 k -n d8-system set env deploy/deckhouse HELM_TIMEOUT=60m
```

Чтобы переключиться на предыдущий движок, используйте команду:

```bash
d8 k -n d8-system set env deploy/deckhouse USE_NELM=false
```

Если миграция не смогла подготовить все биндинги, релиз не стартует, а биндинги остаются в прежнем состоянии.

## Как проверить, что контроллер поддерживает биндинги в актуальном состоянии?

Компонент `user-authz-controller` синхронизирует ClusterRoleBinding и RoleBinding каждого ClusterAuthorizationRule и AuthorizationRule, выдачу `d8:use:dict` в экспериментальной ролевой модели и проекции manage-ролей в use-RoleBinding'и неймспейсов. О его состоянии говорят три источника (статус объекта, метрики и алерты).

**Статус объекта.** У каждого правила есть условие `Ready` и число созданных для него ClusterRoleBinding и RoleBinding:

Чтобы получить список всех ClusterAuthorizationRule в кластере, выполните команду:

```bash
d8 k get clusterauthorizationrules
```

Пример вывода:

```console
NAME        ACCESS LEVEL   READY   BINDINGS   AGE
my-rule     Admin          True    3          5d
```

Чтобы получить список всех AuthorizationRule во всех неймспейсах кластера, выполните команду:

```bash
d8 k get authorizationrules -A
```

Пример вывода:

```console
NAMESPACE   NAME       ACCESS LEVEL   READY   BINDINGS   AGE
default     app-rule   Editor         True    2          3d
team-a      dev-rule   User           False   0          1h
```

Чтобы получить только массив conditions указанного ClusterAuthorizationRule, выполните команду:

```bash
d8 k get clusterauthorizationrule <name> -o jsonpath='{.status.conditions}'
```

Пример вывода:

```json
[
  {
    "lastTransitionTime": "2026-09-11T10:00:00Z",
    "message": "3 bindings applied",
    "observedGeneration": 1,
    "reason": "BindingsApplied",
    "status": "True",
    "type": "Ready"
  }
]
```

`Ready=True` с причиной `BindingsApplied` означает, что биндинги соответствуют правилу. `Ready=False` указывает на проблему: `InvalidSpec` (правило нельзя превратить в биндинги, в сообщении сказано почему) или `ApplyError` (API-сервер отклонил запись; в сообщении указаны биндинг и ошибка). На правиле записываются события с теми же причинами.

**Метрики.** Контроллер отдаёт их на порту `metrics` своего пода, собирает их `PodMonitor` `user-authz-controller` (нужен включённый модуль `operator-prometheus`). Лейбл `kind` принимает значения `ClusterAuthorizationRule`, `AuthorizationRule`, `dict` и `manage`.

| Метрика | Описание                                                                                                                                                                                  |
|---|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `d8_user_authz_authorization_rules{kind}` | Известные контроллеру объекты данного вида                                                                                                                                                |
| `d8_user_authz_bindings_desired{kind}` | Биндинги, которые должны быть у объектов данного вида                                                                                                                                     |
| `d8_user_authz_bindings_actual{kind}` | Существующие биндинги данного вида                                                                                                                                                        |
| `d8_user_authz_bindings_drift{kind,reason}` | Биндинги, не приведённые к желаемому состоянию на последнем reconcile; `reason` — `missing`, `extra` или `changed`. Больше нуля, пока контроллер не привёл биндинги к желаемому состоянию |
| `d8_user_authz_bindings_apply_total{kind,op,result}` | Операции записи контроллера (`op`: `create`, `update`, `delete`; `result`: `success`, `error`)                                                                                            |
| `d8_user_authz_authorization_rules_invalid{kind,reason}` | Правила с условием `Ready=False` по виду и причине                                                                                                                                        |
| `d8_user_authz_authorization_rule_invalid{kind,name,rule_namespace,reason}` | `1` для каждого такого правила (по имени экспортируется не более 50 правил; агрегат выше всегда полный)                                                                                   |
| `d8_user_authz_custom_cluster_roles{level}` | Кастомные ClusterRole (с аннотацией `user-authz.deckhouse.io/access-level`) по уровням доступа                                                                                            |
| `d8_user_authz_custom_aggregation_missing{level}` | `1`, если у агрегированной ClusterRole `user-authz:<level>:custom` нет правил, хотя кастомные роли для неё существуют                                                                     |
| `d8_user_authz_bindings_keep_stamped_total`, `d8_user_authz_keep_stamp_duration_seconds` | Работа миграционного хука, защищающего созданные чартом биндинги от удаления релизом: сколько объектов помечено и длительность последнего прогона                                         |
| `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds` | Стандартные метрики controller-runtime по каждому контроллеру (лейбл `controller`: `clusterauthorizationrule-bindings`, `authorizationrule-bindings`, `dict-bindings`, `manage-bindings`) |

**Алерты** (в правилах Prometheus `d8_user_authz`):

- `D8UserAuthzControllerUnavailable` и `D8UserAuthzControllerTargetDown` — у контроллера недоступные реплики или его метрики не собираются 5 минут;
- `D8UserAuthzControllerReconcileErrorsHigh` — устойчивый поток ошибок reconcile;
- `D8UserAuthzBindingsDrift` — биндинги какого-то вида 15 минут не приводятся к желаемому состоянию;
- `D8UserAuthzAuthorizationRulesInvalid` и `D8UserAuthzAuthorizationRuleInvalid` — число и имена правил, биндинги которых не применяются 10 минут;
- `D8UserAuthzCustomRolesNotAggregated` — агрегированная ClusterRole уровня доступа остаётся пустой, хотя кастомные роли для неё есть.

Если правило остаётся в `Ready=False` с `ApplyError` из-за того, что у биндинга вручную изменили `roleRef`, удалите этот биндинг: `roleRef` неизменяем, контроллер пересоздаст биндинг с правильным.

## Как проверить, что вебхук авторизации знает актуальные правила?

Вебхук авторизации и Permission Browser читают параметры мультитенантности из ClusterAuthorizationRules (`limitNamespaces`, `namespaceSelector`, `allowAccessToSystemNamespaces`) напрямую из API, поэтому изменение доходит до них за несколько секунд.

Чтобы проверить, что поды вебхука запущены, выполните команду:

```bash
d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
```

Пример вывода (по одному поду на каждый master, в сети узла; готовы должны быть оба контейнера):

```console
NAME                       READY   STATUS    RESTARTS   AGE   IP           NODE       NOMINATED NODE   READINESS GATES
user-authz-webhook-qrm2n   2/2     Running   0          2d    10.0.0.11    master-0   <none>           <none>
user-authz-webhook-h6jbh   2/2     Running   0          2d    10.0.0.12    master-1   <none>           <none>
user-authz-webhook-97fvs   2/2     Running   0          2d    10.0.0.13    master-2   <none>           <none>
```

Чтобы вывести последние 100 строк логов контейнера вебхука из всех таких подов, выполните команду:

```bash
d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100
```

Пример вывода:

```console
2026/09/11 10:00:00 server is starting to listen on  127.0.0.1:40443 ...
2026/09/11 10:00:01 rules source: directory rebuilt from 12 rules (18 subjects, 0 quarantined) in 4.2ms
```

Строка `rules source: directory rebuilt from N rules` показывает, что informer получил список правил, и сколько их в каталоге; `quarantined` — число правил, у которых не скомпилировались шаблоны `limitNamespaces`.

Экземпляр, который ещё не прочитал правила, не готов и отвечает на запросы авторизации ошибкой, а не запретом. В те секунды, пока созданное правило не дошло до вебхука, его субъектам временно запрещён доступ ко всем неймспейсам.

Метрики вебхук отдаёт через сайдкар-контейнер `kube-rbac-proxy`; их собирает PodMonitor `user-authz-webhook`, для этого должен быть включён модуль `operator-prometheus`. На каждый master-узел приходится своя серия.

| Метрика | Описание |
|---|---|
| `user_authz_webhook_rules_informer_synced` | `1`, если экземпляр хотя бы раз прочитал ClusterAuthorizationRules |
| `user_authz_webhook_rules_observed` | Правила, которые экземпляр использует сейчас |
| `user_authz_webhook_rules_subjects` | Число различных субъектов в этих правилах |
| `user_authz_webhook_rules_max_resource_version` | Наибольший `resourceVersion` среди прочитанных правил. Сравните его с кластером, чтобы измерить отставание |
| `user_authz_webhook_rules_quarantined` | Правила, которые экземпляр не смог использовать полностью: не компилируется паттерн `limitNamespaces` или `namespaceSelector` либо правило не удалось прочитать |
| `user_authz_webhook_rules_directory_updated_timestamp_seconds` | Время последнего обновления правил |
| `user_authz_webhook_rules_directory_rebuilds_total`, `user_authz_webhook_rules_directory_rebuild_duration_seconds` | Число обновлений правил и их длительность |
| `user_authz_webhook_rules_watch_errors_total` | Ошибки list и watch при чтении правил |

Permission Browser отдаёт те же метрики с префиксом `user_authz_permission_browser`; их собирает PodMonitor `permission-browser-apiserver`.

Для обоих компонентов модуль поставляет следующие алерты:

| Алерт | Когда срабатывает |
|---|---|
| `D8UserAuthzWebhookTargetDown` | Prometheus 5 минут не может собрать метрики хотя бы с одного экземпляра вебхука |
| `D8UserAuthzWebhookRulesQuarantined` | Правило 10 минут не компилируется |
| `D8UserAuthzWebhookRulesWatchErrors` | Вебхук 10 минут не может следить за изменениями правил |
| `D8UserAuthzWebhookDirectoryDiverged` | Экземпляры 10 минут используют разные наборы правил, поэтому на один и тот же запрос приходит разный ответ в зависимости от master-узла, на который он попал |
| `D8UserAuthzRulePropagationLag` | Один экземпляр час не обновлял правила, а другой обновлял |
| `D8UserAuthzPermissionBrowserUnavailable` | У Permission Browser есть недоступные реплики |

## Почему изменение ClusterAuthorizationRule применяется не сразу?

API-сервер кеширует ответы вебхука авторизации. В ресурсе AuthorizationConfiguration, который создаёт `control-plane-manager`, для вебхука заданы `authorizedTTL: 5m`, `unauthorizedTTL: 30s` и `timeout: 3s`. Ключ кеша — весь SubjectAccessReview, поэтому ответ на повторный идентичный запрос приходит из кеша.

Вебхук никогда не разрешает запрос: он либо запрещает его, либо не имеет мнения, и оба ответа API-сервер кеширует на `unauthorizedTTL`, то есть на 30 секунд. Поэтому создание правила или его сужение применяется к конкретному запросу в течение 30 секунд с момента последнего идентичного запроса:

```bash
d8 k auth can-i --as=user@example.com get pods -n other-namespace
sleep 30
d8 k auth can-i --as=user@example.com get pods -n other-namespace
```

Сбросить кеш без перезапуска `kube-apiserver` нельзя.

Для ресурса только что установленного CRD, задержка достигает 40 секунд. Вебхук узнаёт из discovery, принадлежит ли ресурс неймспейсу, и запрашивает discovery не чаще раза в 10 секунд, после чего API-сервер кеширует ответ ещё на 30 секунд.

В эти секунды ограничения по неймспейсам к такому ресурсу не применяются: вебхук о нём ещё не знает, мнения не имеет, и отвечает один только RBAC — поэтому субъект, которого правило ограничивает несколькими неймспейсами, может запросить новый ресурс по всему кластеру. Установка CRD требует значительно более широких прав, чем может предоставить это окно, поэтому discovery запрашивается с ограниченной частотой, а не на каждый запрос. У Permission Browser есть такое же по порядку окно для его отчёта — до 30 секунд.

## Почему правило требует мультитенантности?

Параметры `limitNamespaces`, `namespaceSelector` и `allowAccessToSystemNamespaces` применяет вебхук авторизации, а он разворачивается, только если включён параметр [`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy). Правило, в котором эти параметры заданы при выключенной мультитенантности, применяется без них: субъекты правила получают его уровень доступа во **всех** неймспейсах кластера, включая системные.

В метриках учитываются только параметры, которые задают ограничение. Правило с `allowAccessToSystemNamespaces: false` или с пустым списком `limitNamespaces` в метрики не попадает.

О ситуации сообщают две метрики:

| Метрика | Описание |
|---|---|
| `d8_user_authz_rule_needs_multitenancy{name,options}` | Одна серия на каждое затронутое правило, не более 50, с параметрами, которые не применятся |
| `d8_user_authz_rules_needing_multitenancy` | Общее число затронутых правил, включая те, что не попали в первые 50 |

Алерт `D8UserAuthzRuleNeedsMultiTenancy` называет отдельное правило, а `D8UserAuthzRulesNeedMultiTenancy` срабатывает, когда затронутых правил больше, чем называет первый алерт. Оба входят в группу `D8UserAuthzMisconfigured`.

Чтобы устранить ситуацию, включите мультитенантность или уберите параметры из правил.

Чтобы посмотреть, включена ли мультитенантность, используйте команду:

```bash
d8 k get moduleconfig user-authz -o jsonpath='{.spec.settings.enableMultiTenancy}'
```

Чтобы получить имена ClusterAuthorizationRule, которые ограничивают доступ по неймспейсу или разрешают доступ к системным неймспейсам, используйте команду:

```bash
d8 k get clusterauthorizationrule -o json | jq -r '.items[] | select((.spec.limitNamespaces // [] | length > 0) or (.spec.namespaceSelector != null) or (.spec.allowAccessToSystemNamespaces == true)) | .metadata.name'
```

## Как получить аналог ролей ClusterAdmin и SuperAdmin в гранулярной модели?

В гранулярной модели нет ролей, объединяющих права в одну сущность, как `ClusterAdmin` и `SuperAdmin` в упрощённой модели. В ней разделяется управление платформой (системные роли) и доступ к приложениям (namespace- и проектные роли). Эквивалент собирается из **двух привязок**: ClusterRoleBinding на системную роль и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) на проектную роль (она действует во всех проектах, включая создаваемые позже).

Приблизительное соответствие уровней:

| Роль упрощённой модели | Эквивалент в гранулярной модели |
|---------------------|----------------------------------------|
| `User` | `d8:namespace:viewer` (через `RoleBinding` или `ProjectRoleBinding`). |
| `PrivilegedUser` | `d8:namespace:user`. |
| `Editor` | `d8:namespace:manager` — тот же уровень; отличается область: роль гранулярной модели выдаётся на неймспейс (RoleBinding) или на проект (ProjectRoleBinding), а не на весь кластер с фильтром по неймспейсам. |
| `Admin` | `d8:namespace:admin`. |
| `ClusterEditor` | Прямого аналога нет. `ClusterEditor` — уровень `Editor` во всех неймспейсах, включая системные, плюс cluster-scoped-объекты. Собирается из ClusterProjectRoleBinding на `d8:project:manager` (все проекты) и системной роли для платформенной части; `d8:system:manager` сам по себе эквивалентом не является: доступа к пользовательским неймспейсам он не даёт. |
| `ClusterAdmin` | `d8:system:manager` + `ClusterProjectRoleBinding` на `d8:project:admin`. |
| `SuperAdmin` | `d8:system:superadmin` + `ClusterProjectRoleBinding` на `d8:project:superadmin`. |

Пример для `ClusterAdmin` (группа `k8s-admins`):

```yaml
# Платформа: конфигурация модулей DP, cluster-wide-ресурсы, системные неймспейсы.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-admins-platform
subjects:
  - kind: Group
    name: k8s-admins
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:system:manager
  apiGroup: rbac.authorization.k8s.io
---
# Приложения: администратор во всех неймспейсах всех проектов (включая будущие).
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: k8s-admins-projects
spec:
  subjects:
    - kind: Group
      name: k8s-admins
  roleRef:
    kind: ClusterRole
    name: d8:project:admin
```

Для `SuperAdmin` замените роли на `d8:system:superadmin` и `d8:project:superadmin`.

Особенности:

- При включённом [автоматическом создании проектов](/modules/multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects) каждый пользовательский неймспейс является проектом, поэтому пара «системная роль + ClusterProjectRoleBinding» покрывает и платформу, и все пользовательские неймспейсы. Не покрывается только неймспейс `default` — он не относится ни к проектам, ни к системным.
- Создать собственную роль «со всеми правами» (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`) не получится: такая роль даёт, в том числе права на управление проектами и будет отклонена [встроенной защитой](./#встроенные-защиты-ролевой-модели). Если нужен именно неограниченный доступ ко всему API (вне ролевой модели платформы), используйте ClusterRoleBinding на встроенную роль Kubernetes `cluster-admin` — назначить её может только тот, у кого такие права уже есть.

## Как дать пользователю доступ только к ресурсам одного модуля?

Типовой запрос: пользователь в неймспейсе должен работать только с ресурсами одного модуля (например, только с виртуальными машинами), не видя остальных ресурсов (Pod, Deployment и т. п.).

Каждый модуль DP поставляет отдельные capabilities на свои ресурсы, поэтому такой доступ выдаётся без написания RBAC-правил. Соберите [собственную роль](#создание-собственной-namespace--или-проектной-роли), агрегирующую только capabilities нужного модуля (селектор по лейблу `module`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:virtualization-only
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"   # Разрешает использовать роль в RoleBinding внутри проектов.
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: capability
        rbac.deckhouse.io/scope: namespace
        module: virtualization
rules: []
```

Выдайте роль через RoleBinding в нужном неймспейсе или через [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) на весь проект. Пользователь получит доступ только к ресурсам модуля — стандартные ресурсы Kubernetes ему видны не будут.

Вне проектов то же самое можно сделать, привязав capability модуля напрямую, без создания роли:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: virtualization-view
  namespace: my-namespace
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:namespace-capability:virtualization:view
  apiGroup: rbac.authorization.k8s.io
```

{% alert level="info" %}
Внутри неймспейсов **проектов** обычный RoleBinding может ссылаться только на роли, [доступные проекту](/modules/multitenancy-manager/usage.html#какие-роли-доступны-в-rolebinding-внутри-проекта), — capabilities туда по умолчанию не входят, поэтому для проектов используйте вариант с собственной ролью (лейбл `rbac.deckhouse.io/delegatable: "true"` в примере выше как раз делает её доступной) либо ProjectRoleBinding.
{% endalert %}

## Как расширить роль или создать новую?

[Гранулярная ролевая модель](./#гранулярная-ролевая-модель) построена на принципе агрегации, она собирает более мелкие роли в более обширные,
тем самым предоставляя лёгкие способы расширения модели собственными ролями.

### Создание новой роли подсистемы

Предположим, что текущие подсистемы не подходят под ролевое распределение в компании и требуется создать новую [подсистему](./#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `deckhouse`, подсистемы `kubernetes` и модуля user-authn.

Для решения этой задачи создайте следующую роль:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
    rbac.deckhouse.io/aggregate-to-system-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
```

В начале указаны лейблы для новой роли:

- показывает, какую namespace-роль хук должен использовать при создании RoleBinding в неймспейсах модулей:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- показывает, что роль является кастомной (кастомные роли не определяют собственных правил, а только агрегируют capabilities):

  ```yaml
  rbac.deckhouse.io/kind: custom-role
  ```

  > Этот лейбл обязателен.

- показывает, что роль является ролью подсистемы, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/scope: subsystem
  ```

- указывает подсистему, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/subsystem: mycustom
  ```

- позволяет роли `d8:system:manager` агрегировать эту роль в себя:

  ```yaml
  rbac.deckhouse.io/aggregate-to-system-as: manager
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль менеджера из подсистемы `deckhouse`:

  ```yaml
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- агрегирует все системные (scope `system`) capabilities модуля `user-authn`:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

Таким образом роль получает права от подсистем `deckhouse`, `kubernetes` и от модуля `user-authn`.

Особенности:

* кастомные роли и capabilities должны иметь префикс имени `d8:custom:` (остальное пространство имён `d8:` зарезервировано за встроенными объектами DP). Имя должно согласовываться с объявленной областью: подсистемная роль — `d8:custom:<подсистема>:<имя>` (сегмент — сама подсистема, как в примере выше), namespace- или проектная роль — `d8:custom:namespace:<имя>` и `d8:custom:project:<имя>`, capability — `d8:custom:<область>-capability:<имя>`. Имя, расходящееся с лейблом `rbac.deckhouse.io/scope`, будет отклонено;
* RoleBinding с namespace-ролью (`d8:namespace:<уровень>`) будут созданы в неймспейсах модулей агрегированных подсистем, уровень задаётся лейблом `rbac.deckhouse.io/use-role`.

### Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для manage-роли) CRD-объект — MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом.

Первым делом нужно дополнить роль новым селектором:

```yaml
rbac.deckhouse.io/aggregate-to-mycustom-as: manager
```

Этот селектор позволит агрегировать capabilities к новой подсистеме через указание этого лейбла. После добавления нового селектора роль будет выглядеть так:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   name: d8:custom:mycustom:manager
   labels:
     rbac.deckhouse.io/use-role: admin
     rbac.deckhouse.io/kind: custom-role
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
     rbac.deckhouse.io/aggregate-to-system-as: manager
 aggregationRule:
   clusterRoleSelectors:
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
     - matchLabels:
         rbac.deckhouse.io/scope: system
         module: user-authn
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-mycustom-as: manager
 rules: []
 ```

 Далее нужно создать новую capability, в которой следует определить права для нового ресурса. Например, только чтение:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-mycustom-as: manager
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
     rbac.deckhouse.io/capability: "custom.subsystem-capability.mycustom.superresource_view"
   name: d8:custom:subsystem-capability:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

Capability добавит свои права в роль подсистемы, дав права на просмотр нового объекта.

Особенности:

* кастомные capabilities должны иметь префикс имени `d8:custom:`; остальная часть имени не ограничена, но для читаемости лучше использовать этот стиль.

### Расширение существующих подсистемных ролей

Если необходимо расширить существующую роль, нужно выполнить те же шаги, что и в пункте выше, но изменив лейблы и название роли.

Пример для расширения роли менеджера из подсистемы `deckhouse` (`d8:subsystem:deckhouse:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: deckhouse
    rbac.deckhouse.io/capability: "custom.subsystem-capability.deckhouse.superresource_view"
  name: d8:custom:subsystem-capability:deckhouse:superresource:view
rules:
- apiGroups:
  - mygroup.io
  resources:
  - mysuperresources
  verbs:
  - get
  - list
  - watch
```

Таким образом новая capability расширит роль `d8:subsystem:deckhouse:manager`.

### Расширение подсистемных ролей с добавлением нового неймспейса

Если необходимо добавить новый неймспейс (для создания в нём хуком RoleBinding с namespace-ролью), потребуется добавить лишь один лейбл:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

Этот лейбл сообщает хуку, что в этом неймспейсе нужно создать RoleBinding с namespace-ролью:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: deckhouse
     rbac.deckhouse.io/namespace: namespace
   name: d8:custom:subsystem-capability:deckhouse:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

Хук отслеживает ClusterRoleBinding и при создании привязки анализирует все системные и подсистемные роли, чтобы найти все объединенные в них capabilities с помощью проверки правила агрегации. Затем он берёт неймспейс из лейбла `rbac.deckhouse.io/namespace` и создает RoleBinding с namespace-ролью в этом неймспейсе.

Хук отслеживает только объекты с лейблом `rbac.deckhouse.io/scope: system` или `subsystem`. Capability без этого лейбла всё так же отдаёт свои правила роли через агрегацию, но её лейбл `rbac.deckhouse.io/namespace` не будет прочитан, и RoleBinding в неймспейсе не появится.

### Расширение существующих namespace-ролей

Если ресурс принадлежит неймспейсу, необходимо расширить namespace-роль вместо системной/подсистемной. Разница лишь в лейблах и имени:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-namespace-as: user
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: namespace
     rbac.deckhouse.io/capability: "custom.namespace-capability.mycustom.superresource_view"
   name: d8:custom:namespace-capability:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

Эта capability дополнит роль `d8:namespace:user`.

### Создание собственной namespace- или проектной роли

Иногда встроенная иерархия уровней не подходит: например, нужна роль «разработчик» — просмотр всего неймспейса плюс чтение логов, но без права менять квоты или RBAC. Такая роль собирается из готовых capabilities, без написания RBAC-правил вручную.

Правила для собственных ролей:

- имя должно начинаться с `d8:custom:` (например, `d8:custom:namespace:developer`);
- роль должна иметь лейбл `rbac.deckhouse.io/kind: custom-role`;
- namespace- или проектная роль, которую будут выдавать через RoleBinding, должна также иметь `rbac.deckhouse.io/delegatable: "true"`. Каждый пользовательский неймспейс — это проект, и RoleBinding в нём принимают только роли с этим лейблом. На системные и подсистемные роли лейбл ставить нельзя — вебхук отклонит такую роль;
- роль **не может содержать собственных правил** (`rules`) — только агрегировать capabilities через `aggregationRule`. Права описываются в отдельных capabilities — так состав роли всегда прозрачен;
- нельзя в одной роли агрегировать capabilities пользовательских областей (`namespace`, `project`) вместе с административными (`system`, подсистемы) — такая роль будет отклонена.

Пример: роль, включающая всё, что разрешено `d8:namespace:viewer`, плюс одну конкретную capability (подключение к подам), выбранную адресно по её уникальному лейблу `rbac.deckhouse.io/capability`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:developer
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"   # Нужен, чтобы роль можно было указать в RoleBinding внутри проекта.
  annotations:
    custom.meta.deckhouse.io/title: "Разработчик"
    custom.meta.deckhouse.io/description: "Просмотр ресурсов и подключение к подам, без управления квотами и RBAC"
aggregationRule:
  clusterRoleSelectors:
    # Всё, что входит в уровень viewer namespace-линейки.
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-namespace-as: viewer
    # Плюс одна конкретная capability, выбранная по её уникальному имени.
    - matchLabels:
        rbac.deckhouse.io/capability: "namespace-capability.kubernetes.access_terminal"
rules: []
```

Если готовой capability с нужными правами нет, создайте собственную (`custom-capability` может содержать правила) и добавьте в `aggregationRule` роли селектор по её лейблу `rbac.deckhouse.io/capability` (в примере ниже — `matchLabels: {rbac.deckhouse.io/capability: "custom.logs-reader"}`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace-capability:logs-reader
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/capability: "custom.logs-reader"
rules:
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get", "list"]
```

Чтобы получить список всех доступных capabilities и их уникальных имён, используйте команду:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability \
  -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

Созданная роль назначается так же, как и встроенная: через RoleBinding в неймспейсе или через [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) на весь проект (для проектных ролей используйте `rbac.deckhouse.io/scope: project` и агрегируйте `aggregate-to-project-as`). Для RoleBinding в неймспейсе проекта на роли обязателен лейбл `delegatable`; ProjectRoleBinding его не требует (у него свой список допустимых префиксов). Назначить роль через ClusterRoleBinding нельзя — как и встроенные роли этих областей.

> Собрать такую роль можно и без YAML — мастером выдачи доступа в веб-интерфейсе Deckhouse Console: он показывает доступные capabilities, собирает из них роль и сразу создаёт нужную привязку.

## Как перевести кастомные роли на новую схему в DP 1.78?

{% alert level="warning" %}
Кастомные роли и capabilities, в отличие от [встроенных ролей](./#устаревшие-имена-ролей), псевдонимов совместимости не получают: в DP 1.78 они перестают агрегировать права, а алерт `D8UserAuthzLegacyRBACv2CustomRoleFound` их перечисляет. Обновление на DP 1.78 не блокируется; обновление на следующий за ним релиз удерживается требованием релиза `legacyRBACv2CustomRolesCount`, пока не мигрирован каждый такой объект.
{% endalert %}

Вместе с переименованием ролей ([соответствие имён](./#устаревшие-имена-ролей)) изменилась и схема лейблов, используемых для агрегации прав.

Кастомные роли, созданные по старой схеме, после обновления **перестают собирать права**: встроенные capabilities получили новые лейблы, и старые селекторы агрегации (например, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as`) больше их не находят. Псевдонимы совместимости для кастомных ролей и capabilities не создаются — их нужно обновить вручную.

Соответствие старой и новой схем:

| Было (старая схема) | Стало (новая схема) |
|---------------------|---------------------|
| Произвольное имя роли (например, `custom:manage:mycustom:manager`) | Обязательный префикс `d8:custom:` (например, `d8:custom:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной роли | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной capability | `rbac.deckhouse.io/kind: custom-capability`, имя с префиксом `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-system-as: <уровень>` |
| Селектор агрегации: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` | Только `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` |
| Селектор для use-прав: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>` |
| Селектор по модулю: `rbac.deckhouse.io/kind: manage` + `module: <модуль>` | `rbac.deckhouse.io/scope: system` + `module: <модуль>` |

Имена встроенных capabilities также изменились (без псевдонимов):

* `d8:manage:permission:module:<модуль>:view|edit` → `d8:system-capability:<модуль>:view|edit`;
* `d8:use:capability:module:<модуль>:view|edit` → `d8:namespace-capability:<модуль>:view|edit`.

Селекторы агрегации работают по лейблам, а не по именам ролей и capabilities. Поэтому после переименования объектов достаточно обновить селекторы агрегации, чтобы они соответствовали новой схеме. Прямые привязки к capabilities использовать не следует.

Чтобы получить список всего, что ещё предстоит мигрировать при переходе на новую схему, используйте команду:

```shell
d8 k get clusterroles -o json | jq -r '.items[] | select((.metadata.name | startswith("custom:")) and ((.metadata.labels["rbac.deckhouse.io/kind"] // "" | IN("manage", "use")) or ([.aggregationRule.clusterRoleSelectors[]?.matchLabels["rbac.deckhouse.io/kind"] // ""] | any(IN("manage", "use"))))) | .metadata.name'
```

### Порядок миграции

Для миграции выполните следующие действия:

1. Создайте новую версию кастомной роли с префиксом `d8:custom:`, лейблом `rbac.deckhouse.io/kind: custom-role` и новыми селекторами агрегации. Если это namespace- или проектная роль, которую будете выдавать через RoleBinding, добавьте `rbac.deckhouse.io/delegatable: "true"`. Руководствуйтесь примерами «до и после» ниже.
1. Пересоздайте кастомные capabilities с лейблом `rbac.deckhouse.io/kind: custom-capability` и префиксом имени `d8:custom:`.
1. Пересоздайте объекты RoleBinding и ClusterRoleBinding, указывающие на старую роль, указав новые имена ролей в поле `roleRef`. Это поле является неизменяемым, поэтому существующие привязки необходимо удалить и создать заново.
1. После проверки корректности новых привязок удалите старые роли и capabilities.

### Примеры

#### Кастомная роль до и после

Пример конфигурации роли, объединяющей права подсистем `deckhouse` и `kubernetes` и модуля `user-authn`.

* Было (старая схема):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: custom:manage:mycustom:manager
    labels:
      rbac.deckhouse.io/use-role: admin
      rbac.deckhouse.io/kind: manage
      rbac.deckhouse.io/level: subsystem
      rbac.deckhouse.io/subsystem: custom
      rbac.deckhouse.io/aggregate-to-all-as: manager
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels:
          rbac.deckhouse.io/kind: manage
          rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
      - matchLabels:
          rbac.deckhouse.io/kind: manage
          rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
      - matchLabels:
          rbac.deckhouse.io/kind: manage
          module: user-authn
  rules: []
  ```

* Стало (новая схема):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: d8:custom:mycustom:manager
    labels:
      rbac.deckhouse.io/use-role: admin
      rbac.deckhouse.io/kind: custom-role
      rbac.deckhouse.io/scope: subsystem
      rbac.deckhouse.io/subsystem: mycustom
      rbac.deckhouse.io/aggregate-to-system-as: manager
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels:
          rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
      - matchLabels:
          rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
      - matchLabels:
          rbac.deckhouse.io/scope: system
          module: user-authn
  rules: []
  ```

Что изменилось:

- имя получило обязательный префикс `d8:custom:`;
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`;
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`;
- `rbac.deckhouse.io/aggregate-to-all-as` → `rbac.deckhouse.io/aggregate-to-system-as`;
- из селекторов агрегации убран лейбл `rbac.deckhouse.io/kind: manage`;
- выборка всех системных прав модуля теперь выполняется по `rbac.deckhouse.io/scope: system` + `module: <модуль>`.

#### Кастомная capability до и после

Пример конфигурации capability, которая даёт права на просмотр ресурса MySuperResource и агрегируется в роль из примера выше (в её поле `aggregationRule` должен быть селектор `rbac.deckhouse.io/aggregate-to-mycustom-as: manager`).

* Было (старая схема):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: custom:manage:permission:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: manage
      rbac.deckhouse.io/aggregate-to-custom-as: manager
  rules:
    - apiGroups:
        - mygroup.io
      resources:
        - mysuperresources
      verbs:
        - get
        - list
        - watch
  ```

* Стало (новая схема):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: d8:custom:capability:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: custom-capability
      rbac.deckhouse.io/aggregate-to-mycustom-as: manager
  rules:
    - apiGroups:
        - mygroup.io
      resources:
        - mysuperresources
      verbs:
        - get
        - list
        - watch
  ```

### Лейблы и аннотации: было и стало

Лейблы на объектах ClusterRole:

| Лейбл | Было | Стало | Назначение |
|-------|------|-------|------------|
| `rbac.deckhouse.io/kind` | `manage` или `use` | `custom-role` / `custom-capability` — для кастомных объектов; `role` / `capability` — у встроенных (зарезервированы) | Тип объекта ролевой модели. Обязателен: объекты без него не обрабатываются |
| `rbac.deckhouse.io/level` | `all` \| `subsystem` \| `module` | Удалён | Старый уровень роли; заменён лейблом `scope` |
| `rbac.deckhouse.io/scope` | — | `system` \| `subsystem` \| `namespace` | Область действия роли или capability |
| `rbac.deckhouse.io/subsystem` | Имя подсистемы | Без изменений | Подсистема роли; используется при `scope: subsystem` |
| `rbac.deckhouse.io/use-role` | Уровень use-роли | Уровень namespace-роли | Какая namespace-роль автоматически выдаётся обладателю системной/подсистемной роли в системных неймспейсах её модулей (через автоматически создаваемые объекты RoleBinding) |
| `rbac.deckhouse.io/aggregate-to-all-as` | `<уровень>` | Переименован в `rbac.deckhouse.io/aggregate-to-system-as` | Агрегация объекта в общесистемную роль (`d8:system:<уровень>`) |
| `rbac.deckhouse.io/aggregate-to-<подсистема>-as` | Использовался в селекторах вместе с `rbac.deckhouse.io/kind: manage` | Используется в селекторах сам по себе | Агрегация объекта в подсистемную роль указанного уровня |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<уровень>` (для use-прав) | По-прежнему для **подсистемы** kubernetes (`d8:subsystem:kubernetes:*`). Старый use-смысл переехал на `aggregate-to-namespace-as` | Агрегация в `d8:subsystem:kubernetes:<уровень>` |
| `rbac.deckhouse.io/namespace` | Неймспейс | Без изменений | Дополнительный неймспейс, в котором обладателям роли автоматически создаётся RoleBinding |
| `rbac.deckhouse.io/capability` | — | Уникальное имя capability (например, `system-capability.deckhouse.view`) | Машиночитаемый идентификатор встроенной capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` на ролях-псевдонимах | Роль устарела и будет удалена; переведите привязки на новую роль |
| `module` | Имя модуля | Без изменений | Принадлежность встроенного объекта модулю DP; удобен в селекторах агрегации вместе со `scope` |
| `heritage: deckhouse` | Признак объекта платформы | Без изменений | Устанавливать на кастомные объекты нельзя |

Аннотации на объектах ClusterRole (в старой схеме аннотации не использовались):

| Аннотация | Назначение |
|-----------|------------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | Отображаемые название и описание роли или capability на русском языке (платформа ставит их на встроенные объекты; на кастомных можно указать свои) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | То же на английском языке |
| `rbac.deckhouse.io/deprecated-replaced-by` | Введена в DP 1.78 вместе с новой схемой. Роли-псевдонимы один релиз агрегируют capabilities **новой** роли — существующие привязки продолжают авторизовывать, затем псевдонимы удаляются. Это не те же права, что до апгрейда: `d8:use:role:admin` больше не даёт выпуск токена ServiceAccount и impersonate. Аннотация на каждой прежней роли указывает имя новой роли, на которую нужно мигрировать |

### Добавление кастомной capability (в новой схеме)

Capability — это обычный объект ClusterRole с правилами, который через лейбл агрегации автоматически включается в выбранную роль. В новой схеме кастомная capability создаётся следующим образом:

1. Определите, какую роль нужно расширить: namespace-роль, подсистемную, системную или кастомную.
1. Создайте ClusterRole с префиксом имени `d8:custom:` (для читаемости — `d8:custom:capability:<имя>:<ресурс>:<действие>`), лейблом `rbac.deckhouse.io/kind: custom-capability` и лейблом агрегации целевой роли:
   - `rbac.deckhouse.io/aggregate-to-namespace-as: <viewer|user|manager|admin|superadmin>` — в namespace-роль `d8:namespace:<уровень>`;
   - `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <viewer|manager|superadmin>` — в подсистемную роль `d8:subsystem:<подсистема>:<уровень>`;
   - `rbac.deckhouse.io/aggregate-to-system-as: <viewer|manager|superadmin>` — в системную роль `d8:system:<уровень>`;
   - `rbac.deckhouse.io/aggregate-to-<имя своей подсистемы>-as: <уровень>` — в кастомную роль (такой селектор должен присутствовать в её поле `aggregationRule`).
1. Опишите права в `rules`.

Kubernetes агрегирует правила автоматически: сразу после создания capability её права появятся у всех обладателей целевой роли. Проверить результат можно командой `d8 k auth can-i --as <пользователь>` или посмотрев итоговые правила роли: `d8 k get clusterrole <роль> -o yaml`.

Примеры конфигурации доступны выше в подразделах «[Кастомная роль до и после](#кастомная-роль-до-и-после)» и «[Кастомная capability до и после](#кастомная-capability-до-и-после)».

## Как переименовать встроенную роль?

Изменять права встроенных ролей нельзя, но можно изменить их отображаемое название и описание — например, чтобы в интерфейсе они назывались в принятых в компании терминах. Для этого добавьте на роль аннотации `custom.meta.deckhouse.io/title` и `custom.meta.deckhouse.io/description`:

```shell
d8 k annotate clusterrole d8:namespace:admin \
  custom.meta.deckhouse.io/title='Администратор команды' \
  custom.meta.deckhouse.io/description='Управление ресурсами и доступом в пространстве имён команды'
```

Это единственное изменение, которое разрешено вносить в объекты с префиксом `d8:` (кроме `d8:custom:*`): попытка изменить правила, агрегацию или лейблы встроенной роли будет отклонена.

## Как узнать, у кого есть доступ к ресурсу?

При включённом режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) доступен обратный запрос к авторизации — ресурс WhoCan. Он отвечает на вопрос «кто может выполнить действие X над ресурсом Y?» и возвращает список пользователей, групп и ServiceAccount'ов:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: WhoCan
metadata:
  name: who-can-create-networkpolicies
spec:
  resourceAttributes:
    namespace: my-namespace
    verb: create
    group: networking.k8s.io
    resource: networkpolicies
EOF
```

Ответ возвращается в поле `status` (`users`, `groups`, `serviceAccounts`) сразу в выводе команды; объект нигде не сохраняется.

Право создавать WhoCan-запросы даёт кластерная роль `d8:user-authz:who-can-checker`. Она никому не выдана по умолчанию: результат запроса раскрывает субъекты доступа во всех неймспейсах, поэтому выдавайте её только доверенным администраторам через ClusterRoleBinding.

## Как узнать, что разрешено конкретному пользователю, группе или ServiceAccount'у?

При включённом режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) доступен ресурс SubjectAccessReport — обратная сторона WhoCan. Он отвечает на вопрос «что разрешено этому субъекту» и сразу возвращает отчёт: какие роли и через какие привязки выданы, какие действия и над какими ресурсами разрешены в кластере и в каждом неймспейсе, и откуда взялось каждое право.

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SubjectAccessReport
metadata:
  name: what-can-jane-do
spec:
  subject:
    kind: User
    name: jane@example.com
EOF
```

Поле `spec.subject.kind` принимает значения `User`, `Group` и `ServiceAccount` (для последнего обязательно укажите `spec.subject.namespace`). Если `spec.subject` не указан, отчёт строится для того, кто выполняет запрос.

Особенности отчёта:

- Группы пользователя определяются автоматически по каталогу [Group](../user-authn/cr.html#group), включая вложенные: если пользователь состоит в группе `B`, а `B` входит в `A`, права `A` тоже попадут в отчёт. Учтённые группы возвращаются в `status.subject.groups`. Группы, которых нет в каталоге (например, приходящие из внешнего провайдера аутентификации), можно передать в `spec.groups`.
- В каждом источнике права (`status.scopes[].resources[].sources[]`) есть поле `matchedBy`: по нему видно, выдано право лично субъекту или через группу — и можно посмотреть картину прав без учёта групп.
- Неймспейсы с одинаковым доступом объединяются в одну секцию (`status.scopes[].namespaces`), поэтому проект с десятком неймспейсов не превращается в десяток одинаковых таблиц.
- Права, выданные ClusterRoleBinding, показываются один раз в кластерной области (`status.scopes[].cluster: true`) — они действуют во всех неймспейсах.
- Ограничить отчёт конкретными неймспейсами можно через `spec.namespaces`.

Отчёт строится по данным RBAC. Ограничения admission-вебхуков не сворачиваются в строки RBAC — они помечаются в `status.scopes[].caveat`. Например, изменение и удаление системных ресурсов запрещено ниже уровня `superadmin` — кроме администраторов кластера (`cluster-admin`, `user-authz:super-admin` и bypass-групп). Если права субъекта ограничены ресурсом ClusterAuthorizationRule, об этом сообщается в `status.notes`.

Право строить отчёт о **другом** субъекте даёт кластерная роль `d8:user-authz:subject-access-checker`. Как и `who-can-checker`, она намеренно никому не выдана по умолчанию: отчёт раскрывает полную карту прав субъекта, включая чужие неймспейсы. Отчёт о самом себе доступен любому аутентифицированному пользователю и не требует дополнительных прав.

## Как пользователю увидеть список доступных ему неймспейсов?

При включённом режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) список неймспейсов фильтруется автоматически: команда `d8 k get namespaces` возвращает пользователю только те неймспейсы, к которым у него есть доступ — независимо от механизма предоставления доступа (привязки ролей, ProjectRoleBinding/ClusterProjectRoleBinding, ClusterAuthorizationRule/AuthorizationRule). Пользователь не видит чужих неймспейсов и не может по списку узнать об их существовании.

Тот же список отдаёт read-only ресурс `accessiblenamespaces` — его может запросить любой аутентифицированный пользователь **для самого себя**:

```shell
d8 k get accessiblenamespaces
```

Это удобно для скриптов и интерфейсов: не нужно перебирать неймспейсы и проверять доступ к каждому.
