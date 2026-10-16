---
title: "Модуль user-authz: FAQ"
description: "Ответы на частые вопросы о модуле user-authz: ограничение доступа, совместное использование ролевых моделей, собственные роли, переход на новую схему лейблов, диагностика контроллера и вебхука авторизации."
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

<div style="height: 0;" id="как-ограничить-права-пользователю-конкретными-пространствами-имён-устаревшая-ролевая-модель"></div>

## Как ограничить права пользователю конкретными неймспейсами?

Чтобы ограничить права пользователя конкретными неймспейсами в гранулярной ролевой модели, используйте в RoleBinding [namespace-роль](./#namespace-роли) с соответствующим уровнем доступа. См. [пример назначения административных прав пользователю в рамках неймспейса](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-неймспейса).

В упрощённой ролевой модели используйте параметры `namespaceSelector` или `limitNamespaces` (устарел) в кастомном ресурсе [ClusterAuthorizationRule](cr.html#clusterauthorizationrule). Эти параметры требуют параметра [`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy): пока он выключен, ClusterAuthorizationRule с ними отклоняется. Правило, в котором они уже заданы, применяется без ограничений, см. [«Почему правило требует мультитенантности?»](#почему-правило-требует-мультитенантности)

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

1. `jane.doe@example.com` имеет право запрашивать и просматривать объекты во всех неймспейсах с лейблом `env=review`.
2. `Administrators` могут запрашивать, редактировать, получать и удалять объекты на уровне кластера и в неймспейсах с лейблами `env=prod` и `env=stage`.

Так как для `Jane Doe` подходят два правила, итоговые права определяются следующим образом:

* `Jane Doe` будет иметь максимальный accessLevel среди всех подходящих правил — `ClusterAdmin`.
* Опции `namespaceSelector` будут объединены так, что `Jane Doe` будет иметь доступ в неймспейсы с лейблом `env` со значением `review`, `stage` или `prod`.

{% alert level="warning" %}
Если есть правило без опции `namespaceSelector` и без опции `limitNamespaces` (устаревшая), это значит, что доступ разрешён во все неймспейсы, кроме системных. Это влияет на итоговый список неймспейсов, доступных пользователю.
{% endalert %}

## Можно ли использовать упрощённую и гранулярную ролевые модели одновременно?

Да. Обе модели в итоге сводятся к стандартному механизму RBAC Kubernetes, а RBAC — разрешающая модель: права из всех источников **суммируются**. Если действие разрешено хотя бы одним источником — ClusterAuthorizationRule, AuthorizationRule, RoleBinding на роль гранулярной модели или ProjectRoleBinding, — оно будет разрешено. Ничего специально «переключать» не нужно: можно оставить существующие ClusterAuthorizationRule и постепенно добавлять привязки ролей гранулярной модели.

Это верно и в режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), с одним правилом об уровнях. Ограничение неймспейсов в ClusterAuthorizationRule (`limitNamespaces` или `namespaceSelector`) ограничивает права только этого правила: в неймспейсе вне списка пользователь сохраняет то, что ему дают RoleBinding, AuthorizationRule или ProjectRoleBinding, на уровне этой привязки — `accessLevel` из ClusterAuthorizationRule туда не распространяется. Неймспейс, в котором у пользователя нет ни одного источника прав, остаётся недоступным. Полный список источников и пример — [в описании модуля](./#совместное-использование-clusterauthorizationrule-authorizationrule-и-rbac).

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

- При включённом модуле `multitenancy-manager` [каждый пользовательский неймспейс является проектом](/modules/multitenancy-manager/#автоматическое-создание-проектов-для-неймспейсов), поэтому пара «системная роль + ClusterProjectRoleBinding» покрывает и платформу, и все пользовательские неймспейсы. Не покрывается только неймспейс `default` — он не относится ни к проектам, ни к системным.
- Создать собственную роль «со всеми правами» (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`) не получится: такая роль даёт, в том числе права на управление проектами и будет отклонена [встроенной защитой](./#встроенные-механизмы-защиты-ролевой-модели). Если нужен именно неограниченный доступ ко всему API (вне ролевой модели платформы), используйте ClusterRoleBinding на встроенную роль Kubernetes `cluster-admin` — назначить её может только тот, у кого такие права уже есть.

## Как дать пользователю доступ только к ресурсам одного модуля?

Типовой запрос: пользователь в неймспейсе должен работать только с ресурсами одного модуля (например, только с виртуальными машинами), не видя остальных ресурсов (Pod, Deployment и т. п.).

Каждый модуль DP поставляет отдельные capabilities на свои ресурсы, поэтому такой доступ выдаётся без написания RBAC-правил. Соберите [собственную роль](#создание-собственной-namespace--или-проектной-роли), агрегирующую только capabilities нужного модуля. Каждую capability выбирайте по её лейблу `rbac.deckhouse.io/capability`: роль с лейблом `rbac.deckhouse.io/delegatable: "true"` может выбирать capabilities только по этому лейблу или по лейблам `rbac.deckhouse.io/aggregate-to-namespace-as` и `rbac.deckhouse.io/aggregate-to-project-as`, а селектор по `module` или по любому другому лейблу отклоняется. Чтобы получить список capabilities модуля, выполните команду:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability,module=virtualization,rbac.deckhouse.io/scope=namespace -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

Пример роли, в которой `<ACTION>` нужно заменить значениями из списка:

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
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
rules: []
```

Значения без префикса `custom.` зарезервированы за capabilities, которые поставляет Deckhouse, и ClusterRole пользователя с таким значением отклоняется, поэтому эти селекторы выбирают только capabilities модуля.

Выдайте роль через RoleBinding в нужном неймспейсе или через [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) на весь проект. Пользователь получит доступ только к ресурсам модуля — стандартные ресурсы Kubernetes ему видны не будут.

{% alert level="info" %}
Не привязывайте capability модуля напрямую. Внутри неймспейсов **проектов** обычный RoleBinding может ссылаться только на роли, [доступные проекту](/modules/multitenancy-manager/usage.html#роли-доступные-в-rolebinding-внутри-проекта), а capabilities по умолчанию в их число не входят. У собственной роли из примера выше есть лейбл `rbac.deckhouse.io/delegatable: "true"`, который делает её доступной в проектах, поэтому одна и та же роль работает в любом неймспейсе.
{% endalert %}

## Как обновление на релиз с контроллером влияет на биндинги?

Ничего не создаётся заново, доступ не прерывается. В DP 1.78 компонент `user-authz-controller` принимает под управление биндинги, которые раньше создавал Helm-чарт: те же объекты с теми же именами, к ним добавляется `ownerReference` на правило.

Спланируйте релиз, в котором выполняется миграция: он один раз обрабатывает все существующие биндинги, поэтому идёт дольше обычного. На последующие релизы это не влияет.

| Биндинги в релизе | Чего ожидать |
|---|---|
| До 5000 | Релиз идёт дольше обычного и завершается сам |
| Больше 5000 | Релиз может не уложиться в 20-минутный тайм-аут модуля и завершиться с ошибкой |

Посчитайте биндинги перед обновлением. Для получения количества кластерных биндингов используйте команду:

```bash
d8 k get clusterrolebindings -l heritage=deckhouse,module=user-authz --no-headers | wc -l
```

Для получения количества namespaced-биндингов используйте команду:

```bash
d8 k get rolebindings -A -l heritage=deckhouse,module=user-authz --no-headers | wc -l
```

Если их больше 5000, на время обновления увеличьте тайм-аут релиза или переключитесь на предыдущий движок релизов, а после обновления верните настройку:

Чтобы увеличить тайм-аут релиза, используйте команду:

```bash
d8 k -n d8-system set env deploy/deckhouse HELM_TIMEOUT=60m
```

Чтобы переключиться на предыдущий движок, используйте команду:

```bash
d8 k -n d8-system set env deploy/deckhouse USE_NELM=false
```

Если миграция не смогла подготовить все биндинги, релиз не стартует, а биндинги остаются в прежнем состоянии.

После миграции время релиза больше не зависит от числа правил. Алерт `D8UserAuthzReleaseSlow` срабатывает, если применение релиза модуля повторно занимает больше 5 минут, а `D8UserAuthzRenderSlow` — если рендеринг его шаблонов повторно занимает больше 30 секунд. Оба указывают на API-сервер или на нехватку CPU у пода Deckhouse, а не на правила.

## Как проверить, что контроллер поддерживает биндинги в актуальном состоянии?

Компонент `user-authz-controller` синхронизирует ClusterRoleBinding и RoleBinding каждого ClusterAuthorizationRule и AuthorizationRule, ClusterRoleBinding на `d8:dict` гранулярной модели и объекты RoleBinding, которые дают субъектам системных и подсистемных ролей namespace-роль из лейбла `rbac.deckhouse.io/use-role` в неймспейсах модулей. О его состоянии говорят статус объекта, метрики и алерты.

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
| `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds` | Стандартные метрики controller-runtime по каждому контроллеру (лейбл `controller`: `clusterauthorizationrule-bindings`, `authorizationrule-bindings`, `dict-bindings`, `manage-bindings`) |

Deckhouse экспортирует следующие метрики хуков модуля:

| Метрика | Описание |
|---|---|
| `d8_user_authz_custom_cluster_roles{level}` | Кастомные ClusterRole (с аннотацией `user-authz.deckhouse.io/access-level`) по уровням доступа                                                                                            |
| `d8_user_authz_custom_aggregation_missing{level}` | `1`, если у агрегированной ClusterRole `user-authz:<level>:custom` нет правил, хотя кастомные роли для неё существуют                                                                     |
| `d8_user_authz_bindings_keep_stamped_total`, `d8_user_authz_keep_stamp_duration_seconds` | Работа миграционного хука, защищающего созданные чартом биндинги от удаления релизом: сколько объектов помечено и длительность последнего прогона                                         |

**Алерты.** Для контроллера модуль поставляет следующие алерты:

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

Строка `rules source: directory rebuilt from N rules` показывает, что informer получил список правил, и сколько их в каталоге; `quarantined` — число правил, которые экземпляр не смог использовать полностью: не компилируется шаблон `limitNamespaces` или `namespaceSelector` либо правило не удалось прочитать.

Экземпляр, который ещё не прочитал правила, не готов и отвечает на запросы авторизации ошибкой, а не запретом. В те секунды, пока созданное правило не дошло до вебхука, тем его субъектам, которых не покрывает ни одно уже прочитанное правило, временно запрещён доступ ко всем неймспейсам. Субъект, которого покрывает другое правило, сохраняет область этого правила.

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

API-сервер кеширует ответы вебхука авторизации. В конфигурации AuthorizationConfiguration, которую создаёт `control-plane-manager`, для вебхука заданы `authorizedTTL: 5m`, `unauthorizedTTL: 30s` и `timeout: 3s`. Ключ кеша — весь SubjectAccessReview, поэтому ответ на повторный идентичный запрос приходит из кеша.

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

Пока мультитенантность выключена, admission-вебхук `d8-user-authz-car-multitenancy-related-options.deckhouse.io` отклоняет ClusterAuthorizationRule с любым из этих параметров, даже с `allowAccessToSystemNamespaces: false`. Он также не даёт выключить `enableMultiTenancy`, пока параметры используются в правилах. Поэтому правило попадает в такое состояние, только если оно было записано до того, как эти проверки начали действовать, например до обновления. Любое изменение такого правила должно также удалять эти параметры.

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

## Как расширить роль или создать новую?

[Гранулярная ролевая модель](./#гранулярная-ролевая-модель) построена на принципе агрегации, она собирает более мелкие роли в более обширные,
тем самым предоставляя лёгкие способы расширения модели собственными ролями.

### Создание новой роли подсистемы

Предположим, что текущие подсистемы не подходят под ролевое распределение в компании и требуется создать новую [подсистему](./#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `cluster`, подсистемы `network` и модуля user-authn.

Для решения этой задачи создайте следующую роль:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: viewer
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-cluster-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-network-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
```

В начале указаны лейблы для новой роли:

- показывает, какую namespace-роль `user-authz-controller` выдаёт субъектам роли в неймспейсах модулей; собственная роль проецируется не выше `viewer`:

  ```yaml
  rbac.deckhouse.io/use-role: viewer
  ```

- показывает, что роль является кастомной (кастомные роли не определяют собственных правил, а только агрегируют capabilities):

  ```yaml
  rbac.deckhouse.io/kind: custom-role
  ```

  {% alert level="info" %}
  Этот лейбл обязателен.
  {% endalert %}

- показывает, что роль является ролью подсистемы, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/scope: subsystem
  ```

- указывает подсистему, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/subsystem: mycustom
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль менеджера из подсистемы `cluster`:

  ```yaml
  rbac.deckhouse.io/aggregate-to-cluster-as: manager
  ```

- агрегирует все системные (scope `system`) capabilities модуля `user-authn`:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

Таким образом роль получает права от подсистем `cluster`, `network` и от модуля `user-authn`.

Особенности:

* кастомные роли и capabilities должны иметь префикс имени `d8:custom:` (остальное пространство имён `d8:` зарезервировано за встроенными объектами DP). Имя должно согласовываться с объявленной областью: подсистемная роль — `d8:custom:<подсистема>:<имя>` (сегмент — сама подсистема, как в примере выше), namespace- или проектная роль — `d8:custom:namespace:<имя>` и `d8:custom:project:<имя>`, capability — `d8:custom:<область>-capability:<имя>`. Имя, расходящееся с лейблом `rbac.deckhouse.io/scope`, будет отклонено;
* RoleBinding с namespace-ролью (`d8:namespace:<уровень>`) будут созданы в неймспейсах модулей агрегированных подсистем, уровень задаётся лейблом `rbac.deckhouse.io/use-role`, для собственной роли — не выше `viewer`. Без лейбла такие RoleBinding не создаются;
* кастомную роль нельзя агрегировать в `d8:system:<уровень>`, потому что лейбл `rbac.deckhouse.io/aggregate-to-system-as` может нести только кастомная capability. Чтобы выдать роль обладателям `d8:system:manager`, привяжите её к ним через ClusterRoleBinding.

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
     rbac.deckhouse.io/use-role: viewer
     rbac.deckhouse.io/kind: custom-role
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
 aggregationRule:
   clusterRoleSelectors:
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-cluster-as: manager
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-network-as: manager
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

* Называйте собственную capability `d8:custom:<ОБЛАСТЬ>-capability:<ИМЯ>`, где `<ОБЛАСТЬ>` — значение её лейбла `rbac.deckhouse.io/scope`. Имя, которое не соответствует лейблу, отклоняется.

### Расширение существующих подсистемных ролей

Если необходимо расширить существующую роль, нужно выполнить те же шаги, что и в пункте выше, но изменив лейблы и название роли.

Пример для расширения роли менеджера из подсистемы `cluster` (`d8:subsystem:cluster:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-cluster-as: manager
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: cluster
    rbac.deckhouse.io/capability: "custom.subsystem-capability.cluster.superresource_view"
  name: d8:custom:subsystem-capability:cluster:superresource:view
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

Таким образом новая capability расширит роль `d8:subsystem:cluster:manager`.

### Расширение подсистемных ролей с добавлением нового неймспейса

Неймспейсы, в которых обладатели системной или подсистемной роли получают RoleBinding с namespace-ролью, — это неймспейсы её модулей: `user-authz-controller` берёт их из лейбла `rbac.deckhouse.io/namespace` системных capabilities, которые поставляет Deckhouse, с именем на `d8:` и лейблом `heritage: deckhouse`. На собственной capability лейбл игнорируется: она добавляет свои правила ролям, которые её агрегируют, но не добавляет неймспейс.

Чтобы дать субъектам такой роли доступ к другому неймспейсу, привяжите их там к namespace-роли через RoleBinding:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cluster-managers-viewer
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:namespace:viewer
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: cluster-managers
```

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

Иногда встроенная иерархия уровней не подходит: например, нужна роль «разработчик» — просмотр всего неймспейса плюс доступ к терминалу подов (`exec`, `attach`), но без права менять квоты или RBAC. Такая роль собирается из готовых capabilities, без написания RBAC-правил вручную.

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

Если готовой capability с нужными правами нет, создайте собственную (`custom-capability` может содержать правила) и добавьте в `aggregationRule` роли селектор по её лейблу `rbac.deckhouse.io/capability` (в примере ниже — `matchLabels: {rbac.deckhouse.io/capability: "custom.namespace-capability.port-forward"}`). Значение этого лейбла у собственной capability — `custom.<ОБЛАСТЬ>-capability.<ИМЯ>`, а роль, которую выдают в проекте, может выбирать только области `namespace-capability` и `project-capability`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace-capability:port-forward
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/capability: "custom.namespace-capability.port-forward"
rules:
  - apiGroups: [""]
    resources: ["pods/portforward"]
    verbs: ["create"]
```

Чтобы получить список всех доступных capabilities и их уникальных имён, используйте команду:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

Созданная роль назначается так же, как и встроенная: через RoleBinding в неймспейсе или через [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) на весь проект (для проектных ролей используйте `rbac.deckhouse.io/scope: project` и агрегируйте `aggregate-to-project-as`). Для RoleBinding в неймспейсе проекта на роли обязателен лейбл `delegatable`; ProjectRoleBinding его не требует (у него свой список допустимых префиксов). Назначить роль через ClusterRoleBinding нельзя — как и встроенные роли этих областей.

{% alert level="info" %}
Собрать такую роль можно и без YAML — мастером выдачи доступа в веб-интерфейсе Deckhouse: он показывает доступные capabilities, собирает из них роль и сразу создаёт нужный биндинг.
{% endalert %}

<div style="height: 0;" id="как-перевести-кастомные-роли-на-новую-схему-в-dp-178"></div>

## Как перевести кастомные роли на новую схему?

Вместе с [переименованием ролей](./#устаревшие-имена-ролей) изменилась и схема лейблов, по которым агрегируются права.

{% alert level="warning" %}
У встроенных capabilities больше нет лейблов `rbac.deckhouse.io/kind: manage` и `rbac.deckhouse.io/kind: use`. Кастомная роль старой схемы, которая выбирает capabilities по этим лейблам, потеряла их права, а вместе с ней и её субъекты. Псевдонимы совместимости для кастомных ролей не создаются, поэтому почините такие роли, как описано ниже.
{% endalert %}

Селектор агрегации кастомной роли, который требует один из этих лейблов (например, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<SUBSYSTEM>-as`), не находит ни одной встроенной capability.
По такому селектору контроллер агрегации Kubernetes берёт только правила кастомных capabilities, которые ещё несут старые лейблы.
Deckhouse создаёт объекты RoleBinding для субъектов роли с лейблом `rbac.deckhouse.io/use-role` в неймспейсах модулей, только если у роли есть лейбл `rbac.deckhouse.io/scope: system` или `subsystem`. Кастомная роль старой схемы с `use-role` и без такого лейбла потеряла эти объекты, даже если её селекторы уже переписаны.
Селектор, который требует лейбл агрегации линейки, которую не собирает ни одна встроенная роль, тоже не находит ни одной встроенной capability, потому что модули ставят на свои capabilities только лейблы линеек, которые собирают встроенные роли.
Это, например, лейбл `rbac.deckhouse.io/aggregate-to-all-as` старой схемы, лейбл подсистемы, которой больше нет в ролевой модели, или лейбл подсистемы, из которой ушёл модуль.
Исключение составляет линейка, которую несёт кастомная capability, потому что селектор по-прежнему получает правила этой capability. Кастомная capability здесь означает объект ClusterRole без лейбла `heritage: deckhouse` и без поля `aggregationRule`.
Исключение составляет и линейка собственной подсистемы роли из её лейбла `rbac.deckhouse.io/subsystem`, потому что её несут кастомные capabilities этой подсистемы, см. [Расширение пользовательской роли](#расширение-пользовательской-роли).
Ни одно из исключений не относится к линейкам `all`, `deckhouse`, `infrastructure`, `kubernetes` и `networking`, которые не считаются кастомными.
Устаревшая роль-псевдоним, то есть ClusterRole с лейблом `rbac.deckhouse.io/deprecated: "true"`, не считается встроенной ролью, которая собирает линейку, потому что линейку старой подсистемы она собирает только для привязок к старому имени.
Кастомная capability старой схемы тоже могла перейти в другие роли, см. [Замена capability](#замена-capability).

Такие роли перечисляет алерт `D8UserAuthzLegacyRBACv2CustomRoleFound`.
Он срабатывает на каждый объект ClusterRole без лейбла `heritage: deckhouse` и с полем `aggregationRule`, у которого есть селектор, требующий лейбл `rbac.deckhouse.io/kind: manage` или `use`, есть селектор, требующий такой лейбл агрегации, или есть лейбл `rbac.deckhouse.io/use-role` без `rbac.deckhouse.io/scope: system` или `subsystem`, независимо от имени роли.
Роль без поля `aggregationRule`, например [замороженная ранее](#замороженные-роли), в него не попадает.
Чтобы получить список таких ролей, используйте команду:

```shell
d8 k get clusterroles -o json | jq -r '
  def lineages: [((.matchLabels // {}) | keys[]), (.matchExpressions[]? | select(.operator == "In" or .operator == "Exists") | .key)]
    | map(capture("^rbac\\.deckhouse\\.io/aggregate-to-(?<lineage>.+)-as$").lineage);
  ([.items[] | select(.metadata.labels.heritage == "deckhouse" and .metadata.labels["rbac.deckhouse.io/deprecated"] != "true")
        | .aggregationRule.clusterRoleSelectors[]? | lineages[]] + ["system", "namespace", "project"]) as $collected
  | ([.items[] | select(.metadata.labels.heritage != "deckhouse" and .aggregationRule == null) | (.metadata.labels // {}) | keys[]
        | capture("^rbac\\.deckhouse\\.io/aggregate-to-(?<lineage>.+)-as$").lineage]) as $carried
  | .items[]
  | select(.metadata.labels.heritage != "deckhouse" and .aggregationRule != null)
  | (.metadata.labels["rbac.deckhouse.io/subsystem"] // "") as $own
  | select(
      (
        ((.metadata.labels["rbac.deckhouse.io/use-role"] // "") != "")
        and (((.metadata.labels["rbac.deckhouse.io/scope"] // "") | IN("system", "subsystem")) | not)
      )
      or (
        [
          .aggregationRule.clusterRoleSelectors[]?
          | (.matchLabels["rbac.deckhouse.io/kind"] // empty),
            (.matchExpressions[]? | select(.key == "rbac.deckhouse.io/kind" and .operator == "In") | .values[])
        ]
        | any(IN("manage", "use"))
      )
      or (
        [
          .aggregationRule.clusterRoleSelectors[]? | lineages[]
          | select((IN($collected[]) | not)
              and (IN("all", "deckhouse", "infrastructure", "kubernetes", "networking") or (. != $own and (IN($carried[]) | not))))
        ]
        | length > 0
      )
    )
  | .metadata.name'
```

Соответствие старой и новой схем:

| Было (старая схема) | Стало (новая схема) |
|---------------------|---------------------|
| Произвольное имя роли (например, `custom:manage:mycustom:manager`) | Обязательный префикс `d8:custom:` (например, `d8:custom:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной роли | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной capability | `rbac.deckhouse.io/kind: custom-capability`, имя с префиксом `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` на кастомной capability | `rbac.deckhouse.io/aggregate-to-system-as: <уровень>` |
| `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` на кастомной capability | `rbac.deckhouse.io/kind: custom-capability` + `rbac.deckhouse.io/scope: namespace` + `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>`, имя `d8:custom:namespace-capability:<имя>` |
| `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` на кастомной роли | ClusterRoleBinding роли на субъектов `d8:manage:all:<уровень>` и `d8:system:<уровень>`, потому что кастомная роль не может нести `rbac.deckhouse.io/aggregate-to-system-as` |
| `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` на кастомной capability | `rbac.deckhouse.io/kind: custom-capability` + `rbac.deckhouse.io/scope: namespace` + `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>`, имя `d8:custom:namespace-capability:<имя>` |
| Селектор агрегации: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` | Только `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` |
| Селектор для use-прав: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>` |
| Селектор по модулю: `rbac.deckhouse.io/kind: manage` + `module: <модуль>` | `rbac.deckhouse.io/scope: system` + `module: <модуль>` |
| Лейблы подсистем `deckhouse`, `infrastructure` и `kubernetes` (`rbac.deckhouse.io/aggregate-to-deckhouse-as` и другие) | Один лейбл подсистемы `cluster`, `rbac.deckhouse.io/aggregate-to-cluster-as` |
| Лейбл подсистемы `networking`, `rbac.deckhouse.io/aggregate-to-networking-as` | `rbac.deckhouse.io/aggregate-to-network-as` |
| Модули `user-authz`, `user-authn` и `multitenancy-manager` с лейблами подсистем `security` и `deckhouse` | Лейбл подсистемы `iam`, `rbac.deckhouse.io/aggregate-to-iam-as` |

Имена встроенных capabilities также изменились (без псевдонимов):

* `d8:manage:permission:module:<модуль>:view|edit` → `d8:system-capability:<модуль>:view|edit`;
* `d8:use:capability:module:<модуль>:view|edit` → `d8:namespace-capability:<модуль>:view|edit`.

Селекторы агрегации работают по лейблам, а не по именам. Привязывайте роли, а не capabilities.

### Починка роли

Почините каждую роль, которую перечисляет алерт `D8UserAuthzLegacyRBACv2CustomRoleFound`.
Соблюдайте порядок шагов, чтобы старая роль и её привязки оставались на месте, пока доступ не дадут новые.

1. Создайте рядом со старой ролью роль новой схемы с префиксом имени `d8:custom:`, лейблом `rbac.deckhouse.io/kind: custom-role` и селекторами агрегации новой схемы, без `rbac.deckhouse.io/kind`. См. [пример «до и после»](#кастомная-роль-до-и-после). Селектор по лейблу агрегации линейки, которую не собирает ни одна встроенная роль, замените, как описано в разделе [Селекторы по линейке, которую не собирает ни одна встроенная роль](#селекторы-по-линейке-которую-не-собирает-ни-одна-встроенная-роль). Выберите область действия роли и назовите её соответственно:

   - Роль с лейблом `rbac.deckhouse.io/use-role` или с лейблом `rbac.deckhouse.io/level: subsystem` или `all` получает лейблы `rbac.deckhouse.io/scope: subsystem` и `rbac.deckhouse.io/subsystem: <SUBSYSTEM>` с именем `d8:custom:<SUBSYSTEM>:<NAME>` или лейбл `rbac.deckhouse.io/scope: system` с именем `d8:custom:system:<NAME>`.
     Поставьте на ней лейбл `rbac.deckhouse.io/use-role: viewer`. Тогда для субъектов её объектов ClusterRoleBinding Deckhouse создаёт объекты RoleBinding на `d8:namespace:viewer` в неймспейсах модулей, системные capabilities которых агрегирует роль: собственная роль проецируется не выше `viewer`, большее значение вебхук отклоняет. Без лейбла роль в эти неймспейсы не проецируется совсем.
     Вебхук `rbacv2-cluster-roles.deckhouse.io` отклоняет кастомную роль без лейбла `rbac.deckhouse.io/scope`, селекторы которой называют лейбл агрегации подсистемы или системной области, например `rbac.deckhouse.io/aggregate-to-deckhouse-as`.
   - Namespace- или проектная роль, которую вы выдаёте через RoleBinding, получает лейблы `rbac.deckhouse.io/scope: namespace` и `rbac.deckhouse.io/delegatable: "true"` с именем `d8:custom:namespace:<NAME>`. См. [Создание собственной namespace- или проектной роли](#создание-собственной-namespace--или-проектной-роли).

1. Создайте capabilities новой схемы вместо кастомных capabilities старой схемы, которые агрегирует роль, как описано в разделе [Замена capability](#замена-capability). Старые capabilities пока не удаляйте.
1. Для каждого объекта RoleBinding и ClusterRoleBinding, указывающего на старую роль, создайте объект с теми же субъектами, указывающий на новую роль. Поле `roleRef` неизменяемо, поэтому создайте новый объект, а не редактируйте старый.
   Если у старой роли есть лейблы `rbac.deckhouse.io/kind: manage` и `rbac.deckhouse.io/aggregate-to-all-as`, привяжите новую роль ещё и так, как описано в разделе [Роли в составе системных ролей](#роли-в-составе-системных-ролей). С `rbac.deckhouse.io/kind: use` она расширяла роли неймспейса, см. тот же раздел.
1. Проверьте права субъектов:

   ```shell
   d8 k auth can-i <VERB> <RESOURCE> --as=<USER_NAME>
   d8 k auth can-i <VERB> <RESOURCE> -n <NAMESPACE> --as=<USER_NAME>
   ```

   Чтобы проверить права группы, добавьте к `--as` флаг `--as-group=<GROUP_NAME>`.

1. Удалите старые объекты RoleBinding и ClusterRoleBinding, старую роль и старые capabilities.

Если объекты применяются из Git-репозитория (например, с помощью Argo CD или Flux), внесите те же изменения в репозиторий.
Иначе следующее применение манифестов снова создаст старые объекты.

#### Роли в составе системных ролей

В старой схеме лейбл `rbac.deckhouse.io/aggregate-to-all-as: <LEVEL>` означал одно из двух, в зависимости от лейбла вида рядом с ним.
С `rbac.deckhouse.io/kind: use` он расширял роли `d8:use:role:<LEVEL>` во всех неймспейсах. Им соответствует `d8:namespace:<LEVEL>`, которая выбирает `rbac.deckhouse.io/aggregate-to-namespace-as: <LEVEL>`, поэтому перенесите такие права в capability неймспейса, как описано в разделе [Замена capability](#замена-capability).
С `rbac.deckhouse.io/kind: manage` роль с этим лейблом расширяла `d8:manage:all:<LEVEL>`, и дальше этот раздел о ней.
Ни одна роль новой модели этот лейбл не выбирает. Его выбирают только устаревшие псевдонимы `d8:manage:all:<LEVEL>` и `d8:use:role:<LEVEL>`, в зависимости от лейбла вида рядом с ним, пока их не удалят.
Псевдоним `d8:manage:all:<LEVEL>` и роль `d8:system:<LEVEL>` выбирают лейбл `rbac.deckhouse.io/aggregate-to-system-as: <LEVEL>`, а вебхук `rbacv2-cluster-roles.deckhouse.io` допускает этот лейбл только на кастомной capability.
Поэтому кастомная роль не может расширить эти роли, и субъекты, которые получали старую роль через них, получают новую роль только через собственный объект ClusterRoleBinding.

Следующие команды создают такой объект ClusterRoleBinding для каждого объекта ClusterRoleBinding на `d8:manage:all:<LEVEL>` и `d8:system:<LEVEL>`:

```shell
NEW_ROLE=<NEW_ROLE_NAME>
LEVEL=<LEVEL>
d8 k get clusterrolebindings -o json > clusterrolebindings.json
jq --arg role "$NEW_ROLE" --arg level "$LEVEL" '
  {apiVersion: "v1", kind: "List", items: [.items[]
    | select(.roleRef.kind == "ClusterRole" and (.roleRef.name | IN("d8:manage:all:" + $level, "d8:system:" + $level)))
    | {apiVersion, kind, metadata: {name: ($role + ":" + .metadata.name)},
       roleRef: {apiGroup: "rbac.authorization.k8s.io", kind: "ClusterRole", name: $role}, subjects}]}' \
  clusterrolebindings.json > new-clusterrolebindings.json
d8 k apply -f new-clusterrolebindings.json
```

Перед созданием объектов просмотрите файл `new-clusterrolebindings.json` и удалите из него субъектов, которые не должны получить роль.
Deckhouse не синхронизирует эти объекты с объектами ClusterRoleBinding, из которых они скопированы.
Когда вы отзываете доступ субъекта, удалите оба. Имя копии заканчивается именем исходного объекта.

#### Селекторы по линейке, которую не собирает ни одна встроенная роль

Селектор по лейблу агрегации линейки выбирал capabilities, которые несли этот лейбл.
Например, `rbac.deckhouse.io/aggregate-to-virtualization-as: <LEVEL>` выбирал capabilities модуля `virtualization`.
Когда модуль ставит на них лейбл линейки подсистемы, в которой он теперь находится, селектор по этой линейке выбирает capabilities всех модулей подсистемы, а это может быть намного больше, чем давала роль раньше.
Чтобы новая роль осталась такой же узкой, как старая, выбирайте сами capabilities по лейблу `rbac.deckhouse.io/capability`.
Его значение не зависит от подсистемы. Для системной capability это `system-capability.<MODULE_NAME>.<ACTION>`, для capability неймспейса это `namespace-capability.<MODULE_NAME>.<ACTION>`.

1. Получите список capabilities модуля с уровнями ролей, которые их собирают:

   ```shell
   d8 k get clusterroles -l rbac.deckhouse.io/kind=capability,module=<MODULE_NAME> -o json | jq -r '
     .items[]
     | [.metadata.labels["rbac.deckhouse.io/capability"],
        (.metadata.labels | to_entries
          | map(select(.key | test("^rbac\\.deckhouse\\.io/aggregate-to-.+-as$")) | (.key | ltrimstr("rbac.deckhouse.io/aggregate-to-") | rtrimstr("-as")) + "=" + .value)
          | join(","))]
     | @tsv'
   ```

1. Оставьте capabilities, которые находил старый селектор. Селектор со значением `<LEVEL>` находил capabilities этого уровня и, через встроенные роли нижних уровней, capabilities этих уровней.
1. В новой роли укажите по одному селектору по лейблу `rbac.deckhouse.io/capability` для каждой из них. Чтобы выдать все системные capabilities модуля независимо от уровня, достаточно одного селектора по `rbac.deckhouse.io/scope: system` и `module: <MODULE_NAME>`.

Например, следующая роль неймспейса даёт в неймспейсе то же, что давал селектор по `rbac.deckhouse.io/aggregate-to-virtualization-as: user`, без других модулей подсистемы. Замените `<ACTION>` значениями из списка:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:virtualization-user
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
    rbac.deckhouse.io/delegatable: "true"
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
    - matchLabels:
        rbac.deckhouse.io/capability: namespace-capability.virtualization.<ACTION>
rules: []
```

Роль системной области или подсистемы выбирает значения `system-capability.<MODULE_NAME>.<ACTION>` так же.

### Замена capability

Кастомная capability старой схемы несёт лейбл `rbac.deckhouse.io/kind: manage` или `use` и лейбл агрегации, и её называет алерт `D8UserAuthzForeignAggregationLabel`.
Пока на ней есть лейбл агрегации системной области, области неймспейса или проекта либо встроенной подсистемы, вебхук `rbacv2-cluster-roles.deckhouse.io` отклоняет любое её изменение.
Вебхук допускает такой лейбл только вместе с `rbac.deckhouse.io/kind: custom-capability`, а этот лейбл требует префикса имени `d8:custom:`, который существующий объект получить не может.
Создайте на её месте capability новой схемы с теми же правилами, а старую удалите, когда новая будет давать доступ.

Лейблы старой capability определяют новую:

| Старая capability | Новая capability |
|-------------------|------------------|
| `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <LEVEL>` | `d8:custom:namespace-capability:<NAME>` с лейблами `rbac.deckhouse.io/scope: namespace` и `rbac.deckhouse.io/aggregate-to-namespace-as: <LEVEL>` |
| `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<SUBSYSTEM>-as: <LEVEL>` для встроенной или собственной подсистемы | `d8:custom:subsystem-capability:<NAME>` с лейблами `rbac.deckhouse.io/scope: subsystem`, `rbac.deckhouse.io/subsystem: <SUBSYSTEM>` и тем же лейблом агрегации |
| `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-all-as: <LEVEL>` | `d8:custom:namespace-capability:<NAME>` с лейблами `rbac.deckhouse.io/scope: namespace` и `rbac.deckhouse.io/aggregate-to-namespace-as: <LEVEL>` |
| `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-all-as: <LEVEL>` | `d8:custom:system-capability:<NAME>` с лейблами `rbac.deckhouse.io/scope: system` и `rbac.deckhouse.io/aggregate-to-system-as: <LEVEL>` |

Каждая новая capability также получает лейбл `rbac.deckhouse.io/kind: custom-capability`.

Старая capability с лейблом `rbac.deckhouse.io/namespace: <NAMESPACE>` давала субъектам агрегирующих её ролей объекты RoleBinding в этом неймспейсе, неймспейсе модуля. Deckhouse читает этот лейбл только на ClusterRole, которые поставляет сам, поэтому ни старая capability, ни её собственная замена таких объектов RoleBinding не дают. Чтобы сохранить доступ в этом неймспейсе, привяжите субъектов там к namespace-роли через RoleBinding, см. [Расширение подсистемных ролей с добавлением нового неймспейса](#расширение-подсистемных-ролей-с-добавлением-нового-неймспейса).

Capability с лейблами `rbac.deckhouse.io/kind: use` и `rbac.deckhouse.io/aggregate-to-kubernetes-as: <LEVEL>` расширяла роли `d8:use:role:<LEVEL>`.
Её выбирают только устаревшие псевдонимы `d8:use:role:<LEVEL>`, пока их не удалят. `d8:namespace:<LEVEL>` и `d8:project:<LEVEL>` выбирают лейбл `rbac.deckhouse.io/aggregate-to-namespace-as`, поэтому RoleBinding, переведённый на новую роль, не получает правил этой capability. Такую capability называет алерт `D8UserAuthzLegacyUseCapability`.
Замените её до того, как переводить объекты RoleBinding на новые роли. Например, следующие команды создают её аналог для namespace-ролей:

```shell
OLD_CAPABILITY=<OLD_CAPABILITY_NAME>
NEW_CAPABILITY=d8:custom:namespace-capability:<NAME>
LEVEL=<LEVEL>
d8 k get clusterrole "$OLD_CAPABILITY" -o json | jq --arg name "$NEW_CAPABILITY" --arg level "$LEVEL" '
  {apiVersion, kind, metadata: {name: $name, labels: {
     "rbac.deckhouse.io/kind": "custom-capability",
     "rbac.deckhouse.io/scope": "namespace",
     "rbac.deckhouse.io/aggregate-to-namespace-as": $level}},
   rules}' > new-capability.json
d8 k apply -f new-capability.json
```

Проверьте права субъектов namespace-ролей, а затем удалите старую capability:

```shell
d8 k delete clusterrole "$OLD_CAPABILITY"
```

Псевдонимы `d8:use:role:<LEVEL>` собирают и новую capability, поэтому после удаления старой их субъекты ничего не теряют.

### Замороженные роли

Роль, которую заморозили до того, как встроенные capabilities получили новые лейблы (удалили поле `aggregationRule` и сохранили правила), сохраняет свои права, и алерт `D8UserAuthzLegacyRBACv2CustomRoleFound` её не перечисляет.

* Явные объекты RoleBinding, которые заменили её автоматические, указывают на `d8:use:role:<LEVEL>`. Новая схема сохраняет эти роли только как устаревшие псевдонимы `d8:namespace:<LEVEL>`. Такие объекты называет алерт `D8UserAuthzDeprecatedRBACv2RoleInUse`.
  Переведите их на `d8:namespace:<LEVEL>`, потому что привязка к псевдониму перестаёт давать доступ, когда псевдоним удаляется.
  Следующие команды делают это для объектов RoleBinding с префиксом имени `explicit:`:

  ```shell
  d8 k get rolebindings -A -o json | jq '{apiVersion: "v1", kind: "List", items: [.items[]
    | select(.roleRef.kind == "ClusterRole" and (.roleRef.name | startswith("d8:use:role:")) and (.metadata.name | startswith("explicit:")))]}' \
    > use-role-rolebindings.json
  jq '.items |= map({apiVersion, kind,
      metadata: {name: ("namespace:" + .metadata.name), namespace: .metadata.namespace,
        annotations: ((.metadata.annotations // {}) | with_entries(select(.key == "rbac.deckhouse.io/related-with")))},
      roleRef: (.roleRef | .name |= (sub("^d8:use:role:"; "d8:namespace:") | sub(":kubernetes$"; ""))), subjects})' \
    use-role-rolebindings.json > namespace-rolebindings.json
  d8 k apply -f namespace-rolebindings.json && d8 k get -f namespace-rolebindings.json > /dev/null && d8 k delete -f use-role-rolebindings.json
  ```

  Старые объекты удаляются, только если все новые уже есть, и после сбоя команды можно запустить снова.

* Если на роли остался лейбл `rbac.deckhouse.io/aggregate-to-all-as`, её называет алерт `D8UserAuthzForeignAggregationLabel`, а вебхук `rbacv2-cluster-roles.deckhouse.io` отклоняет любое изменение роли, которое сохраняет лейбл, в том числе повторное применение её манифеста.
  Ни одна роль новой модели этот лейбл не выбирает. Устаревший псевдоним `d8:manage:all:<LEVEL>` ещё выбирает его на роли с `rbac.deckhouse.io/kind: manage`, а устаревший псевдоним `d8:use:role:<LEVEL>` на роли с `rbac.deckhouse.io/kind: use`, пока псевдонимы не удалят.

  С `rbac.deckhouse.io/kind: manage` субъекты `d8:manage:all:<LEVEL>` сохраняют роль через объекты ClusterRoleBinding, которые для них создала заморозка, поэтому удалите лейбл:

  ```shell
  d8 k label clusterrole <ROLE_NAME> rbac.deckhouse.io/aggregate-to-all-as-
  ```

  С `rbac.deckhouse.io/kind: use` роль даёт свои правила субъектам каждого объекта RoleBinding на `d8:use:role:<LEVEL>` через этот лейбл, а заморозка не создала для них привязок. Если удалить лейбл, они потеряют эти правила.
  Сначала создайте capability неймспейса с правилами роли, как описано в разделе [Замена capability](#замена-capability). Её собирают и `d8:namespace:<LEVEL>`, и псевдоним `d8:use:role:<LEVEL>`. Проверьте права субъектов и только потом удалите лейбл с роли или удалите роль.

* Замороженная роль не получает права на ресурсы, добавленные позже, например на ресурсы нового модуля. Чтобы они появились, замените её ролью новой схемы, как описано в разделе [Починка роли](#починка-роли).

RoleBinding, которые прошлый релиз создал в неймспейсах модулей для привязки к кастомной роли с лейблами `rbac.deckhouse.io/kind: manage` и `rbac.deckhouse.io/use-role`, при обновлении удаляются. Когда у новой роли есть лейблы `rbac.deckhouse.io/scope: system` или `subsystem` и `rbac.deckhouse.io/use-role`, а привязка ссылается на неё, они появляются снова как `d8:namespace:<уровень>:binding:<имя ClusterRoleBinding>`, но только в неймспейсах модулей, которые уже несут новые лейблы и системные capabilities которых агрегирует новая роль. Модуль, который ещё несёт старые лейблы, например `rbac.deckhouse.io/kind: manage`, не получает такой RoleBinding, пока не перейдёт на новые.
До этого создайте в неймспейсе этого модуля для субъектов привязки RoleBinding на `d8:namespace:<уровень>` и на каждую ClusterRole этого модуля, которую называет алерт `D8UserAuthzLegacyUseCapability`. Ни одна из этих ролей не является псевдонимом, поэтому такие RoleBinding следующий релиз не держат, а вебхук, который проверяет RoleBinding в неймспейсах проектов, неймспейс модуля не проверяет. Удалите их, когда у модуля появятся новые лейблы и контроллер создаст там свою RoleBinding.
RoleBinding на `d8:use:role:<уровень>` тоже даёт эти права, но о ней сообщает алерт `D8UserAuthzDeprecatedRBACv2RoleInUse`, и она держит релиз, который удаляет псевдонимы.

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
      rbac.deckhouse.io/use-role: viewer
      rbac.deckhouse.io/kind: custom-role
      rbac.deckhouse.io/scope: subsystem
      rbac.deckhouse.io/subsystem: mycustom
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels:
          rbac.deckhouse.io/aggregate-to-cluster-as: manager
      - matchLabels:
          rbac.deckhouse.io/scope: system
          module: user-authn
  rules: []
  ```

Что изменилось:

- имя получило обязательный префикс `d8:custom:` и называет подсистему роли;
- `rbac.deckhouse.io/kind: manage` → `rbac.deckhouse.io/kind: custom-role`;
- `rbac.deckhouse.io/level: subsystem` → `rbac.deckhouse.io/scope: subsystem`;
- лейбл `rbac.deckhouse.io/aggregate-to-all-as` удалён, субъекты `d8:system:manager` получают роль через ClusterRoleBinding, см. [Роли в составе системных ролей](#роли-в-составе-системных-ролей);
- из селекторов агрегации убран лейбл `rbac.deckhouse.io/kind: manage`;
- выборка всех системных прав модуля теперь выполняется по `rbac.deckhouse.io/scope: system` + `module: <модуль>`;
- подсистемы `deckhouse` и `kubernetes` теперь входят в подсистему `cluster`, поэтому два их селектора стали одним. Он выбирает всю подсистему `cluster`, а она шире двух прежних. Чтобы роль осталась такой же узкой, как раньше, выбирайте вместо него системные capabilities нужных модулей по `module: <модуль>` и `rbac.deckhouse.io/scope: system`.

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

* Стало (новая схема):

  ```yaml
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: d8:custom:subsystem-capability:mycustom:superresource:view
    labels:
      rbac.deckhouse.io/kind: custom-capability
      rbac.deckhouse.io/scope: subsystem
      rbac.deckhouse.io/subsystem: mycustom
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
| `rbac.deckhouse.io/kind` | `manage` или `use` | `custom-role` / `custom-capability` для кастомных объектов; `role` / `capability` у встроенных (зарезервированы) | Тип объекта ролевой модели. Обязателен, объекты без него не обрабатываются |
| `rbac.deckhouse.io/level` | `all` \| `subsystem` \| `module` | Удалён | Старый уровень роли; заменён лейблом `scope` |
| `rbac.deckhouse.io/scope` | — | `system` \| `subsystem` \| `namespace` | Область действия роли или capability |
| `rbac.deckhouse.io/subsystem` | Имя подсистемы | Без изменений | Подсистема роли; используется при `scope: subsystem` |
| `rbac.deckhouse.io/use-role` | Уровень use-роли | Уровень namespace-роли | Какая namespace-роль автоматически выдаётся обладателю системной/подсистемной роли в системных неймспейсах её модулей (через автоматически создаваемые объекты RoleBinding); для собственной роли — не выше `viewer` |
| `rbac.deckhouse.io/aggregate-to-all-as` | `<уровень>` | С `rbac.deckhouse.io/kind: manage` заменён лейблом `rbac.deckhouse.io/aggregate-to-system-as` на кастомной capability, который кастомная роль нести не может. С `rbac.deckhouse.io/kind: use` заменён лейблом `rbac.deckhouse.io/aggregate-to-namespace-as` на кастомной capability | Агрегация объекта в общесистемную роль (`d8:system:<уровень>`) |
| `rbac.deckhouse.io/aggregate-to-<подсистема>-as` | Использовался в селекторах вместе с `rbac.deckhouse.io/kind: manage` | Используется в селекторах сам по себе | Агрегация объекта в подсистемную роль указанного уровня |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<уровень>` (для use- и manage-прав) | Не используется. Подсистема kubernetes входит в подсистему `cluster` (`aggregate-to-cluster-as`), а старый use-смысл переехал на `aggregate-to-namespace-as` | Агрегация в старые роли `d8:manage:kubernetes:<уровень>` и `d8:use:role:<уровень>:kubernetes` |
| `rbac.deckhouse.io/namespace` | Неймспейс | Без изменений | Дополнительный неймспейс, в котором обладателям роли автоматически создаётся RoleBinding; читается только на capabilities, которые поставляет Deckhouse, на собственной игнорируется |
| `rbac.deckhouse.io/capability` | — | Уникальное имя capability (например, `system-capability.deckhouse.view`) | Машиночитаемый идентификатор встроенной capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` на ролях-псевдонимах | Роль устарела и будет удалена; переведите привязки на новую роль |
| `module` | Имя модуля | Без изменений | Принадлежность встроенного объекта модулю DP; удобен в селекторах агрегации вместе со `scope` |
| `heritage: deckhouse` | Признак объекта платформы | Без изменений | Устанавливать на кастомные объекты нельзя |

Аннотации на объектах ClusterRole (в старой схеме аннотации не использовались):

| Аннотация | Назначение |
|-----------|------------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | Отображаемые название и описание роли или capability на русском языке (платформа ставит их на встроенные объекты; на кастомных можно указать свои) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | То же на английском языке |
| `rbac.deckhouse.io/deprecated-replaced-by` | Стоит на ролях-псевдонимах прежних имён и называет новую роль, на которую нужно перевести привязки. Псевдоним агрегирует capabilities **новой** роли, поэтому существующие привязки к прежнему имени дают доступ, пока псевдонимы не удалены. Это не те же права, что прежде, потому что `d8:use:role:admin` больше не даёт выпуск токена ServiceAccount и impersonate |

### Добавление кастомной capability (в новой схеме)

Capability — это обычный объект ClusterRole с правилами, который через лейбл агрегации автоматически включается в выбранную роль. В новой схеме кастомная capability создаётся следующим образом:

1. Определите, какую роль нужно расширить: namespace-роль, подсистемную, системную или кастомную.
1. Создайте ClusterRole с именем `d8:custom:<ОБЛАСТЬ>-capability:<ИМЯ>`, лейблом `rbac.deckhouse.io/kind: custom-capability`, лейблом `rbac.deckhouse.io/scope: <ОБЛАСТЬ>` и лейблом агрегации целевой роли:
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
d8 k annotate clusterrole d8:namespace:admin custom.meta.deckhouse.io/title='Администратор команды' custom.meta.deckhouse.io/description='Управление ресурсами и доступом в неймспейсе команды'
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

Это право есть и у менеджеров подсистем `iam` и `security`. Его даёт capability `d8:subsystem-capability:iam:inspect_access` ролей `d8:subsystem:iam:manager` и `d8:subsystem:security:manager`, поэтому оно есть и у ролей выше них (`d8:subsystem:iam:superadmin`, `d8:subsystem:security:superadmin`, `d8:system:manager` и `d8:system:superadmin`).

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

Это право есть и у менеджеров подсистем `iam` и `security`. Его даёт capability `d8:subsystem-capability:iam:inspect_access` ролей `d8:subsystem:iam:manager` и `d8:subsystem:security:manager`, поэтому оно есть и у ролей выше них (`d8:subsystem:iam:superadmin`, `d8:subsystem:security:superadmin`, `d8:system:manager` и `d8:system:superadmin`).

## Как пользователю увидеть список доступных ему неймспейсов?

При включённом режиме мультитенантности ([`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)) список неймспейсов фильтруется автоматически: команда `d8 k get namespaces` возвращает пользователю только те неймспейсы, к которым у него есть доступ — независимо от механизма предоставления доступа (привязки ролей, ProjectRoleBinding/ClusterProjectRoleBinding, ClusterAuthorizationRule/AuthorizationRule). Пользователь не видит чужих неймспейсов и не может по списку узнать об их существовании.

Если включён модуль `console` версии ниже 1.44.0, список не фильтруется: уровни доступа ClusterAuthorizationRule и AuthorizationRule и namespace-роли сохраняют право получать список всех неймспейсов.

Тот же список отдаёт read-only ресурс `accessiblenamespaces` — его может запросить любой аутентифицированный пользователь **для самого себя**:

```shell
d8 k get accessiblenamespaces
```

Это удобно для скриптов и интерфейсов: не нужно перебирать неймспейсы и проверять доступ к каждому.

## Как защитить объекты, которые модуль размещает в пользовательских неймспейсах?

Маркер — это **лейбл** `deckhouse.io/system-resource: "true"` на самом объекте, а не аннотация. Для изменения и удаления объекты для защиты отбираются на стороне API-сервера, в `matchConditions` admission-конфигурации, а они читают лейблы; для `exec`, `attach` и `port-forward` объектом admission является сам запрос подключения, а не под, поэтому вебхук читает под — и снова его лейблы. В обоих случаях объект, помеченный аннотацией, не будет замечен и останется открыт — без какого-либо сообщения об этом. Поставьте лейбл на каждый объект, который ваш модуль размещает в пользовательских неймспейсах (поды, сервисы, PVC, секреты, кастомные ресурсы), — и менять его смогут только `superadmin` и администраторы кластера, а `exec`, `attach` и `port-forward` в его поды будут доступны только им.

Что именно ограничивает этот лейбл, описано в разделе [«Ограничения уровня admin и права superadmin»](./#ограничения-уровня-admin-и-права-superadmin).
