---
title: "Модуль user-authz: FAQ"
---

## Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

<div style="height: 0;" id="как-ограничить-права-пользователю-конкретными-пространствами-имён-устаревшая-ролевая-модель"></div>

## Как ограничить права пользователю конкретными пространствами имён?

Чтобы ограничить права пользователя конкретными пространствами имён в экспериментальной ролевой модели, используйте в `RoleBinding` [use-роль](./#use-роли) с соответствующим уровнем доступа. [Пример...](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-пространства-имён).

В текущей ролевой модели используйте параметры `namespaceSelector` или `limitNamespaces` (устарел) в кастомном ресурсе [ClusterAuthorizationRule](cr.html#clusterauthorizationrule).

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

Так как для `Jane Doe` подходят два правила, необходимо провести вычисления:

* `Jane Doe` будет иметь самый сильный accessLevel среди всех подходящих правил — `ClusterAdmin`.
* Опции `namespaceSelector` будут объединены так, что `Jane Doe` будет иметь доступ в пространства имён, помеченные лейблом `env` со значением `review`, `stage` или `prod`.

{% alert level="warning" %}
Если есть правило без опции `namespaceSelector` и без опции `limitNamespaces` (устаревшая), это значит, что доступ разрешён во все пространства имён, кроме системных, что повлияет на результат вычисления доступных пространств имён для пользователя.
{% endalert %}

## Что произойдёт с биндингами при обновлении на релиз с контроллером?

Ничего не пересоздаётся, доступ не прерывается. Биндинги, которые раньше рендерил Helm-чарт,
`user-authz-controller` принимает под управление как есть: те же объекты, те же имена, с них
снимаются метаданные чарта и добавляется `ownerReference` на правило.

Планировать нужно одно — **сам релиз, в котором происходит миграция**. Перед запуском Helm хук
проставляет `helm.sh/resource-policy: keep` на все живые биндинги, чтобы движок раскатки не удалил
их, когда они исчезнут из рендера. После этого движок делает один проход по предыдущему манифесту,
запрашивая каждый объект из него, чтобы увидеть метку. Этот проход пропорционален числу биндингов и
случается ровно один раз:

| Биндингов в релизе | Чего ожидать |
|---|---|
| до ~5000 | релиз идёт дольше обычного и завершается сам |
| больше ~5000 | проход может не уложиться в 20-минутный таймаут релиза модуля |

Для кластера крупнее этого выполните обновление с поднятым таймаутом или на предыдущем движке
раскатки, а потом верните как было:

```bash
# что-то одно из двух, на время обновления
d8 k -n d8-system set env deploy/deckhouse HELM_TIMEOUT=60m
d8 k -n d8-system set env deploy/deckhouse USE_NELM=false
```

Посчитайте, что у вас есть, прежде чем решать:

```bash
d8 k get clusterrolebindings -l heritage=deckhouse,module=user-authz --no-headers | wc -l
d8 k get rolebindings -A -l heritage=deckhouse,module=user-authz --no-headers | wc -l
```

Если хук не смог проставить метку на все биндинги, он завершается ошибкой и релиз не стартует —
биндинги остаются ровно такими, какими были. Это сделано намеренно: релиз, прошедший без меток,
удалил бы их.

Цена разовая. Как только биндинги принадлежат контроллеру, в релизе больше нет объектов, число
которых зависит от числа правил, и его длительность перестаёт от них зависеть.

## Как проверить, что контроллер поддерживает биндинги в актуальном состоянии?

Компонент `user-authz-controller` синхронизирует ClusterRoleBinding и RoleBinding каждого ClusterAuthorizationRule и AuthorizationRule, выдачу `d8:use:dict` в экспериментальной ролевой модели и проекции биндингов manage-ролей в use-RoleBinding'и неймспейсов. О его состоянии говорят три источника: статус объекта, метрики и алерты.

**Статус объекта.** У каждого правила есть условие `Ready` и число его биндингов:

Чтобы получить список всех ClusterAuthorizationRule в кластере, выполните команду:

```bash
d8 k get clusterauthorizationrules
```

Пример вывода:

```console
NAME        ACCESS LEVEL   READY   BINDINGS   AGE
my-rule     Admin          True    3          5d
```

Чтобы получить список всех AuthorizationRule во всех пространствах имён кластера, выполните команду:

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

`Ready=True` с причиной `BindingsApplied` означает, что биндинги соответствуют правилу. `Ready=False` называет проблему: `InvalidSpec` (правило нельзя превратить в биндинги, в сообщении сказано почему) или `ApplyError` (API-сервер отклонил запись; в сообщении указаны биндинг и ошибка). На правиле записываются события с теми же причинами.

**Метрики.** Контроллер отдаёт их на порту `metrics` своего пода, собирает их `PodMonitor` `user-authz-controller` (нужен включённый модуль `operator-prometheus`). Лейбл `kind` принимает значения `ClusterAuthorizationRule`, `AuthorizationRule`, `dict` и `manage`.

| Метрика | Описание |
|---|---|
| `d8_user_authz_authorization_rules{kind}` | Известные контроллеру объекты данного вида |
| `d8_user_authz_bindings_desired{kind}` | Биндинги, которые должны быть у объектов данного вида |
| `d8_user_authz_bindings_actual{kind}` | Существующие биндинги данного вида |
| `d8_user_authz_bindings_drift{kind,reason}` | Биндинги, не приведённые к желаемому состоянию на последнем reconcile; `reason` — `missing`, `extra` или `changed`. Больше нуля только пока контроллеру не удаётся сойтись |
| `d8_user_authz_bindings_apply_total{kind,op,result}` | Операции записи контроллера (`op`: `create`, `update`, `delete`; `result`: `success`, `error`) |
| `d8_user_authz_authorization_rules_invalid{kind,reason}` | Правила с условием `Ready=False` по виду и причине |
| `d8_user_authz_authorization_rule_invalid{kind,name,rule_namespace,reason}` | `1` для каждого такого правила (по имени экспортируется не более 50 правил; агрегат выше всегда полный) |
| `d8_user_authz_custom_cluster_roles{level}` | Кастомные ClusterRole (с аннотацией `user-authz.deckhouse.io/access-level`) по уровням доступа |
| `d8_user_authz_custom_aggregation_missing{level}` | `1`, если у агрегированной ClusterRole `user-authz:<level>:custom` нет правил, хотя кастомные роли для неё существуют |
| `d8_user_authz_bindings_keep_stamped_total`, `d8_user_authz_keep_stamp_duration_seconds` | Работа миграционного хука, защищающего отрендеренные чартом биндинги от удаления релизом: сколько объектов помечено и длительность последнего прогона |
| `controller_runtime_reconcile_total`, `controller_runtime_reconcile_errors_total`, `controller_runtime_reconcile_time_seconds` | Стандартные метрики controller-runtime по реконсилерам (лейбл `controller`: `clusterauthorizationrule-bindings`, `authorizationrule-bindings`, `dict-bindings`, `manage-bindings`) |

**Алерты** (в правилах Prometheus `d8_user_authz`):

- `D8UserAuthzControllerUnavailable` и `D8UserAuthzControllerTargetDown` — у контроллера недоступные реплики или он не скрейпится 5 минут;
- `D8UserAuthzControllerReconcileErrorsHigh` — устойчивый поток ошибок reconcile;
- `D8UserAuthzBindingsDrift` — биндинги какого-то вида 15 минут не приводятся к желаемому состоянию;
- `D8UserAuthzAuthorizationRulesInvalid` и `D8UserAuthzAuthorizationRuleInvalid` — число и имена правил, биндинги которых не применяются 10 минут;
- `D8UserAuthzCustomRolesNotAggregated` — агрегированная ClusterRole уровня доступа остаётся пустой, хотя кастомные роли для неё есть.

Если правило остаётся в `Ready=False` с `ApplyError` из-за того, что у биндинга вручную изменили `roleRef`, удалите этот биндинг: `roleRef` неизменяем, контроллер пересоздаст биндинг с правильным.

## Как проверить, что вебхук авторизации знает актуальные правила?

Вебхук авторизации и Permission Browser читают параметры мультитенантности из объектов ClusterAuthorizationRule (параметры `limitNamespaces`, `namespaceSelector`, `allowAccessToSystemNamespaces`) напрямую из API через общий informer, поэтому изменение доходит до них за время окна склейки informer'а, а не после рендера Helm.

**Порядок применения.** `user-authz-controller` пишет ClusterRoleBinding'и правила примерно одновременно с самим правилом, и до вебхука они идут разными watch-потоками, поэтому биндинги могут прийти первыми. Субъект, которого управляемый контроллером биндинг связывает с правилом, где вебхук его ещё не видел, получает отказ во всех неймспейсах до прихода правила: кластерный биндинг никогда не даёт больше, чем разрешает его правило. Тот же механизм работает в Permission Browser, поэтому он не может показать доступ шире того, что применяет API-сервер.

**Состояние.** Вебхук сообщает состояние своего informer'а правил.

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

**Метрики.** Webhook отдаёт их по адресу `127.0.0.1` внутри своего пода; сайдкар `kube-rbac-proxy` публикует их на узле, а собирает `PodMonitor` `user-authz-webhook` (должен быть включён модуль `operator-prometheus`). На каждый master приходится один набор серий.

| Метрика | Описание |
|---|---|
| `user_authz_webhook_rules_informer_synced` | `1`, если webhook хотя бы раз получил список `ClusterAuthorizationRules`. Пока `0`, экземпляр неготов и отвечает на запросы авторизации ошибкой, а не запретом, — то есть apiserver этот ответ не кеширует |
| `user_authz_webhook_rules_observed` | Количество правил, из которых собран текущий каталог |
| `user_authz_webhook_rules_subjects` | Количество различных субъектов в текущем каталоге |
| `user_authz_webhook_rules_max_resource_version` | Наибольший `resourceVersion` среди наблюдаемых правил — отметка для сравнения с кластером при измерении отставания |
| `user_authz_webhook_rules_quarantined` | Количество правил, у которых не компилируется паттерн `limitNamespaces` или `namespaceSelector`. Сломанный фильтр отбрасывается, поэтому субъекты получают более узкую область, чем записано |
| `user_authz_webhook_rules_directory_updated_timestamp_seconds` | Unix-время последней пересборки |
| `user_authz_webhook_rules_directory_rebuilds_total`, `user_authz_webhook_rules_directory_rebuild_duration_seconds` | Количество пересборок и время, которое они занимают |
| `user_authz_webhook_rules_watch_errors_total` | Ошибки list/watch informer'а правил |

Permission Browser отдаёт тот же набор с префиксом `user_authz_permission_browser`. Отдаёт он их так же — на loopback-адресе за sidecar'ом `kube-rbac-proxy`, собирает `PodMonitor` `permission-browser-apiserver`; порт самого API-сервера остаётся за слоем агрегации, который Prometheus скрейпить не может. Собственный алерт у него один — `D8UserAuthzPermissionBrowserUnavailable`, по числу доступных реплик. Алерта про правила у него намеренно нет: непрочитанные или устаревшие правила делают его неготовым, неготовый выпадает из Service, и об этом уже сообщает `Unavailable`. Некомпилируемое правило и постоянные ошибки watch — свойство кластера, а не одного потребителя, поэтому о них сообщают алерты webhook'а выше.

**Алерты** (в правилах Prometheus `d8_user_authz`, группа `D8UserAuthzWebhookMalfunctioning`):

- `D8UserAuthzWebhookTargetDown` — какой-либо экземпляр не скрейпится 5 минут (именно любой, а не все сразу: на нескольких мастерах один замолчавший экземпляр и есть тот случай, о котором нужно знать; алерт при этом один — выражение считает замолчавшие экземпляры, а не перечисляет их);
- `D8UserAuthzWebhookRulesQuarantined` — правило 10 минут не компилируется;
- `D8UserAuthzWebhookRulesWatchErrors` — устойчивые ошибки watch в течение 10 минут.

Алерта «экземпляр не получил список правил» намеренно нет: в этом состоянии он неготов, а неготовый экземпляр и так виден как остановленная раскатка и как `D8UserAuthzWebhookTargetDown`, если перестал отвечать. Обратите внимание, что `PodMonitor` **не** выбрасывает неготовые поды: экземпляр со сломанным watch — ровно тот, о котором эти алерты, и фильтрация сделала бы их слепыми в тот момент, когда они становятся истинными.

Ещё два следят за мастерами друг относительно друга — того, чего метрики одного инстанса показать не могут: `D8UserAuthzWebhookDirectoryDiverged`, когда инстансы 10 минут собраны из разных наборов правил (тогда один и тот же запрос отвечается по-разному в зависимости от того, на какой мастер он попал), и `D8UserAuthzRulePropagationLag`, когда самый старый инстанс час не пересобирал каталог, а самый свежий пересобирал в пределах этого часа. Второе условие и оставляет тихий кластер тихим: там, где ничего не меняется, все инстансы одинаково старые, и алерт не срабатывает. На одномастерном кластере не сработает ни тот, ни другой — и это правильно, сравнивать не с чем.

## Почему изменение ClusterAuthorizationRule применяется до 30 секунд?

Потому что API-сервер кеширует ответы вебхука. В `AuthorizationConfiguration`, который рендерит `control-plane-manager`, вебхуку заданы `authorizedTTL: 5m`, `unauthorizedTTL: 30s` и `timeout: 3s`; ключ кеша — весь SubjectAccessReview, поэтому повторный идентичный запрос отвечается из кеша, не доходя до вебхука.

Вебхук никогда не разрешает. Правило говорит, где применяется уровень доступа, а не даёт ли он глагол, — это вопрос RBAC, и ответ на него здесь исключил бы RBAC из цепочки. Поэтому любой ответ вебхука — это либо запрет, либо отсутствие мнения, и оба кешируются по `unauthorizedTTL`, а `authorizedTTL` к нему не применяется никогда.

Цена этого — случай «нет мнения». Если пользователь сделал запрос, когда его ничто не ограничивало, API-сервер помнит это отсутствие мнения 30 секунд, и всё это время на идентичный запрос отвечает один только RBAC. Поэтому создание ограничивающего правила или его сужение применяется к конкретному запросу в течение 30 секунд с момента, когда этот запрос делали в последний раз, а не в течение окна информера:

```bash
# Сразу после создания ограничивающего правила запрос, который делали в предыдущие
# 30 секунд, всё ещё отвечается из кеша.
d8 k auth can-i --as=user@example.com get pods -n other-namespace
sleep 30
d8 k auth can-i --as=user@example.com get pods -n other-namespace
```

Сбросить это окно без перезапуска `kube-apiserver` нельзя. По этой же причине вебхук на старте, пока наполняются его кеши, отвечает `503`, а не запретом: запрет запомнился бы на 30 секунд после того, как вебхук уже готов отвечать по существу.

У только что установленного CRD есть своё окно поверх этого, и они складываются, а не перекрываются.
Чтобы применить ограничение по неймспейсам, вебхуку нужно знать, неймспейсный ли это ресурс, — он
узнаёт это из discovery и запрашивает список API-группы не чаще раза в 10 секунд. Поэтому ресурс,
добавленный в группу, список которой только что получен, до 10 секунд остаётся неизвестным, а
посчитанный по этому ответ API-сервер кеширует ещё 30 — в худшем случае около 40 секунд с момента
установки CRD до того, как мультитенантность начнёт фильтровать запросы к нему. Ограничение частоты
сделано намеренно: имя API-группы и имя ресурса берутся из пути запроса, и без него любой субъект,
которого касается правило, мог бы сделать так, что каждый его запрос стоит API-серверу одного
discovery-запроса. Установка CRD требует куда больших прав, чем даёт это окно.

## Что значит, что правилу «нужна мультитенантность»?

`limitNamespaces`, `namespaceSelector` и `allowAccessToSystemNamespaces` применяет вебхук
авторизации, а он разворачивается только при включённом
[`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy). Правило, задающее что-то из
этого при выключенной настройке, применяется **не частично** — оно применяется без своих
ограничений. Перечисленные в нём субъекты получают уровень доступа правила во всех пространствах
имён кластера, включая системные.

Раньше это был `fail` в чарте, останавливавший рендер всего модуля: одно правило, которое может
написать любой, кому разрешено их создавать, замораживало все остальные изменения модуля, а сообщение
доходило только до того, кто читает логи релиза. Теперь об этом сообщается.

| Метрика | Описание |
|---|---|
| `d8_user_authz_rule_needs_multitenancy{name,options}` | По серии на каждое затронутое правило, не более пятидесяти, с перечнем опций, которые не подействуют. |
| `d8_user_authz_rules_needing_multitenancy` | Сколько правил затронуто всего, включая те, что за пределами названных пятидесяти. |

Их читают два алерта: `D8UserAuthzRuleNeedsMultiTenancy` называет конкретное правило,
`D8UserAuthzRulesNeedMultiTenancy` срабатывает, когда правил больше, чем первый успевает назвать.
Оба в группе `D8UserAuthzMisconfigured` — это утверждение о конфигурации, а не о нездоровье
компонента.

Считаются только значения, которые чего-то просят: правило, где явно написано
`allowAccessToSystemNamespaces: false` или где `limitNamespaces` — пустой список, не нуждается ни в
чём, что применял бы вебхук, и не репортится.

Либо включите мультитенантность, либо уберите опции из правил, чтобы они говорили то, что делают:

```bash
d8 k get moduleconfig user-authz -o jsonpath='{.spec.settings.enableMultiTenancy}'
d8 k get clusterauthorizationrule -o json | jq -r '.items[] | select((.spec.limitNamespaces // [] | length > 0) or (.spec.namespaceSelector != null) or (.spec.allowAccessToSystemNamespaces == true)) | .metadata.name'
```

## Как расширить роли или создать новую?

[Экспериментальная ролевая модель](./#экспериментальная-ролевая-модель) построена на принципе агрегации, она собирает более мелкие роли в более обширные,
тем самым предоставляя лёгкие способы расширения модели собственными ролями.

### Создание новой роли подсистемы

Предположим, что текущие подсистемы не подходят под ролевое распределение в компании и требуется создать новую [подсистему](./#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `deckhouse`, подсистемы `kubernetes` и модуля user-authn.

Для решения этой задачи создайте следующую роль:

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

В начале указаны лейблы для новой роли:

- показывает, какую роль хук должен использовать при создании use ролей:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- показывает, что роль должна обрабатываться как manage-роль:

  ```yaml
  rbac.deckhouse.io/kind: manage
  ```

  > Этот лейбл обязателен.

- показывает, что роль является ролью подсистемы, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/level: subsystem
  ```

- указывает подсистему, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/subsystem: custom
  ```

- позволяет `manage:all`-роли агрегировать эту роль в себя:

  ```yaml
  rbac.deckhouse.io/aggregate-to-all-as: manager
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль менеджера из подсистемы `deckhouse`:

  ```yaml
  rbac.deckhouse.io/kind: manage
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- агрегирует все правила от модуля user-authn:

  ```yaml
   rbac.deckhouse.io/kind: manage
   module: user-authn
  ```

Таким образом роль получает права от подсистем `deckhouse`, `kubernetes` и от модуля user-authn.

Особенности:

* ограничений на имя роли нет, но для читаемости лучше использовать этот стиль;
* use-роли будут созданы в пространстве имён агрегированных подсистем и модуля, тип роли выбран лейблом.

### Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для manage-роли) CRD-объект — MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом.

Первым делом нужно дополнить роль новым селектором:

```yaml
rbac.deckhouse.io/kind: manage
rbac.deckhouse.io/aggregate-to-custom-as: manager
```

Этот селектор позволит агрегировать роли к новой подсистеме через указание этого лейбла. После добавления нового селектора роль будет выглядеть так:

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
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         rbac.deckhouse.io/aggregate-to-custom-as: manager
 rules: []
 ```

 Далее нужно создать новую роль, в которой следует определить права для нового ресурса. Например, только чтение:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-custom-as: manager
     rbac.deckhouse.io/kind: manage
   name: custom:manage:permission:mycustom:superresource:view
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

Роль дополнит своими правами роль подсистемы, дав права на просмотр нового объекта.

Особенности:

* ограничений на имя роли нет, но для читаемости лучше использовать этот стиль.

### Расширение существующих manage subsystem-ролей

Если необходимо расширить существующую роль, нужно выполнить те же шаги, что и в пункте выше, но изменив лейблы и название роли.

Пример для расширения роли менеджера из подсистемы `deckhouse`(`d8:manage:deckhouse:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: manage
  name: custom:manage:permission:mycustommodule:superresource:view
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

Таким образом новая роль расширит роль `d8:manage:deckhouse`.

### Расширение manage subsystem-ролей с добавлением нового пространства имён

Если необходимо добавить новое пространство имён (для создания в нём use-роли с помощью хука), потребуется добавить лишь один лейбл:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

Этот лейбл сообщает хуку, что в этом пространстве имён нужно создать use-роль:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: manage
     rbac.deckhouse.io/namespace: namespace
   name: custom:manage:permission:mycustom:superresource:view
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

Хук мониторит `ClusterRoleBinding` и при создании биндинга ходит по всем manage-ролям, чтобы найти все объединенные в них роли с помощью проверки правила агрегации. Затем он берёт пространство имён из лейбла `rbac.deckhouse.io/namespace` и создает use-роль в этом пространстве имён.

### Расширение существующих use-ролей

Если ресурс принадлежит пространству имён, необходимо расширить use-роль вместо manage-роли. Разница лишь в лейблах и имени:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-kubernetes-as: user
     rbac.deckhouse.io/kind: use
   name: custom:use:capability:mycustom:superresource:view
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

Эта роль дополнит роль `d8:use:role:user:kubernetes`.

## Как перевести кастомные роли на новую схему в DKP 1.78?

{% alert level="warning" %}
Этот раздел описывает [переименование ролевой модели](./#миграция-на-новые-имена-ролей-в-dkp-178), которое вступит в силу в DKP 1.78. До DKP 1.78 кастомные роли и capabilities продолжают работать по старой схеме.
{% endalert %}

Вместе с переименованием ролей ([соответствие имён](./#миграция-на-новые-имена-ролей-в-dkp-178)) в DKP 1.78 изменится схема лейблов, используемых для агрегации прав.

После обновления до DKP 1.78 кастомные роли, созданные по старой схеме, **перестанут собирать права**: встроенные capabilities получат новые лейблы, и старые селекторы агрегации (например, `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as`) больше не будут их находить. Псевдонимы совместимости для кастомных ролей и capabilities не создаются — их нужно обновить вручную.

Соответствие старой и новой схем:

| Было (старая схема) | Стало (новая схема) |
|---------------------|---------------------|
| Произвольное имя роли (например, `custom:manage:mycustom:manager`) | Обязательный префикс `d8:custom:` (например, `d8:custom:subsystem:mycustom:manager`) |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной роли | `rbac.deckhouse.io/kind: custom-role` |
| `rbac.deckhouse.io/kind: manage` или `use` на кастомной capability | `rbac.deckhouse.io/kind: custom-capability`, имя с префиксом `d8:custom:` |
| `rbac.deckhouse.io/level: all \| subsystem \| module` | `rbac.deckhouse.io/scope: system \| subsystem \| namespace` |
| `rbac.deckhouse.io/aggregate-to-all-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-system-as: <уровень>` |
| Селектор агрегации: `rbac.deckhouse.io/kind: manage` + `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` | Только `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` |
| Селектор для use-прав: `rbac.deckhouse.io/kind: use` + `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` | `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>` |
| Селектор по модулю: `rbac.deckhouse.io/kind: manage` + `module: <модуль>` | `rbac.deckhouse.io/scope: system` + `module: <модуль>` |

Имена встроенных capabilities также изменятся (без псевдонимов):

* `d8:manage:permission:module:<модуль>:view|edit` → `d8:system-capability:<модуль>:view|edit`;
* `d8:use:capability:module:<модуль>:view|edit` → `d8:namespace-capability:<модуль>:view|edit`.

Селекторы агрегации работают по лейблам, а не по именам, поэтому при миграции достаточно обновить селекторы. Прямые привязки к capabilities использовать не следует.

### Порядок миграции

После обновления до DKP 1.78 выполните следующее:

1. Создайте новую версию кастомной роли с префиксом `d8:custom:`, лейблом `rbac.deckhouse.io/kind: custom-role` и новыми селекторами агрегации. Руководствуйтесь примерами «до и после» ниже.
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
    name: d8:custom:subsystem:mycustom:manager
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
| `rbac.deckhouse.io/aggregate-to-kubernetes-as` | `<уровень>` (для use-прав) | Переименован в `rbac.deckhouse.io/aggregate-to-namespace-as` | Агрегация объекта в namespace-роль (`d8:namespace:<уровень>`) |
| `rbac.deckhouse.io/namespace` | Неймспейс | Без изменений | Дополнительный неймспейс, в котором обладателям роли автоматически создаётся RoleBinding |
| `rbac.deckhouse.io/capability` | — | Уникальное имя capability (например, `system-capability.deckhouse.view`) | Машиночитаемый идентификатор встроенной capability |
| `rbac.deckhouse.io/deprecated` | — | `"true"` на ролях-псевдонимах | Роль устарела и будет удалена; переведите привязки на новую роль |
| `module` | Имя модуля | Без изменений | Принадлежность встроенного объекта модулю DKP; удобен в селекторах агрегации вместе со `scope` |
| `heritage: deckhouse` | Признак объекта платформы | Без изменений | Устанавливать на кастомные объекты нельзя |

Аннотации на объектах ClusterRole (в старой схеме аннотации не использовались):

| Аннотация | Назначение |
|-----------|------------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | Отображаемые название и описание роли или capability на русском языке (платформа ставит их на встроенные объекты; на кастомных можно указать свои) |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | То же на английском языке |
| `rbac.deckhouse.io/deprecated-replaced-by` | Появится в DKP 1.78 вместе с новой схемой. Правила агрегации прежних ролей изменятся так, что роли продолжат давать те же права, что и соответствующие им новые — существующие привязки не сломаются. Однако сохраняются прежние роли только на один релиз DKP: за это время привязки нужно перевести на новые роли. Аннотация проставляется на каждой прежней роли и содержит имя новой роли, эквивалентной ей по правам, на которую следует мигрировать |

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
