# Гранты на кластерные ресурсы — дизайн

Статус: дизайн. Заменяет прежний вариант с квотами. Квота из фичи **убрана** (делегирована
Kubernetes `ResourceQuota`); фича теперь делает только **доступность** (на какие кластерные ресурсы
проект может ссылаться) и **дефолтинг**.

## Проблема

Проект (тенант) живёт в одном или нескольких неймспейсах и ссылается из своих объектов на
**кластерные (cluster-scoped)** ресурсы: `StorageClass` (через `PersistentVolumeClaim.spec.storageClassName`),
`ClusterIssuer` (через `Certificate.spec.issuerRef` / аннотацию `Ingress`), `ClusterRole` (через
`RoleBinding.roleRef`), `LoadBalancerClass` (через `Service.spec.loadBalancerClass`), а также на
произвольные глобальные ресурсы из CRD сторонних модулей. Платформа должна управлять per-проект тем,
**какие** ресурсы доступны и **какой дефолт** — без проксирования per-user.

Две ключевые требования формируют модель:

1. **Расширяемость.** Разработчик модуля должен мочь зарегистрировать *новый путь валидации* — «в моём
   CRD поле X ссылается на глобальный ресурс Z» — **не редактируя** центральную регистрацию владельца
   ресурса.
2. **Без своей квоты.** Учёт объектов/потребления отдан Kubernetes `ResourceQuota` (per-storage-class
   storage и счёт PVC — нативно; суммарный счёт объектов — `count/<resource>.<group>`; LB-сервисы —
   `services.loadbalancers`). См. [Почему без квоты](#почему-без-квоты).

## Истории пользователя

Роли: **разработчик модуля** (владеет доменом ресурса и/или CRD, ссылающимся на глобальные ресурсы),
**администратор кластера** (управляет проектами), **тенант** (работает внутри неймспейсов проекта).

Разработчик модуля:
- **D1** — зарегистрировать кластерный ресурс как грантуемый: его идентичность и базовую доступность. → `GrantableClusterResourceDefinition`
- **D2** — зарегистрировать *новый путь валидации* для **своего** ресурса («поле X моего CRD ссылается на глобальный ресурс Z») **не редактируя** регистрацию владельца ресурса. → `GrantableClusterResourceReference` *(история, ради которой этот редизайн)*
- **D3** — объявить, как находится кластерный дефолт ресурса (аннотация на объекте). → `defaultFrom`
- **D4** — исключить некоторые объекты ресурса из грантуемых навсегда (hard deny, напр. системные `ClusterRole`). → `excluded`
- **D5** — per-путь выбрать «только валидация» vs «ещё дефолтинг» и как дефолтить. → `fieldPaths[].defaulting`
- **D6** — оставить энфорсмент в своём вебхуке; платформа только рендерит каталог. → `enforcement: External`

Администратор кластера:
- **A1** — управлять per-проект, какие имена доступны (allow-list / селектор). → `ClusterResourceGrantPolicy`
- **A2** — задать per-проект дефолтное имя. → `default` в политике
- **A3** — перевернуть базу для проекта (открыть полностью / закрыть). → `availabilityDefault` в политике
- **A4** — запретить конкретные имена для проекта (перекрыть allow-list). → `denied`/`deniedSelector` в политике

Тенант:
- **T1** — узнать, что доступно проекту и какой дефолт, обычным namespace-RBAC. → `AvailableClusterResource`
- **T2** — ссылка на недопустимый ресурс отклоняется с понятным сообщением; пропущенное поле дозаполняется, где путь это включил.

Observability:
- **O1** — как владелец ресурса, видеть какие пути на него ссылаются. → `definition.status.references`
- **O2** — как автор пути, видеть, привязался ли reference или промахнулся именем. → `reference.status.bound` / condition `Bound`

Вне области (делегировано): **квота** на потребление — отдана Kubernetes `ResourceQuota` (см. [Почему без квоты](#почему-без-квоты)).

## Модель: раскол definition и reference

Governance и пути использования — **два разных концепта**, значит два CRD:

- **`GrantableClusterResourceDefinition`** (cluster-scoped) — объявляет управляемый кластерный ресурс и
  его базовую доступность. Владеет тот, кто владеет доменом ресурса.
- **`GrantableClusterResourceReference`** (cluster-scoped) — объявляет **одно место**, где ресурс
  используется (путь валидации/дефолтинга). Шипает **любой** модуль, для своих ресурсов.

Плюс per-проект части, без изменений:

- **`ClusterResourceGrantPolicy`** (cluster-scoped) — per-проект allow-list + дефолт.
- **`AvailableClusterResource`** (namespaced, read-only) — каталог, который контроллер рендерит, чтобы
  тенант видел доступное.

## CRD

### GrantableClusterResourceDefinition

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
spec:
  grantedResource:                 # управляемый ресурс; отсутствует ⇒ value-backed
    apiGroup: storage.k8s.io       # group + kind (версия резолвится через discovery)
    kind: StorageClass
  enforcement: Managed             # Managed (наши вебхуки) | External (владелец сам, мы лишь рендерим каталог)
  defaultAvailability: All         # All (доступно, пока policy не сузит) | None (opt-in)
  excluded:                        # объекты, недоступные никогда (hard deny): имена и/или селекторы
    - matchLabels:
        storageclass.deckhouse.io/system: "true"
  defaultFrom:                     # как найти дефолтное значение ресурса
    annotationKey: storageclass.kubernetes.io/is-default-class
  catalogFields:                   # поля выданных объектов, которые копируются в каталог тенанта
    - name: provisioner
      path: $.provisioner
    - name: reclaimPolicy
      path: $.reclaimPolicy
status:
  observedGeneration: 1
  references:                      # обратный индекс: какие пути на меня смотрят
    - name: storageclasses-pvc
      resources:
        - persistentvolumeclaims
    - name: storageclasses-postgres
      resources:
        - postgresqls
  referenceCount: 2
  conditions:                      # ставит binding reconciler, см. ниже
    - type: GrantedResourceValid
      status: "True"
      reason: ClusterScoped        # ValueBacked | Namespaced (False) | KindNotServed, MappingFailed (Unknown)
      message: grantedResource StorageClass.storage.k8s.io is cluster-scoped.
    - type: CatalogFieldsValid
      status: "True"
      reason: Valid                # InvalidCatalogFields (False)
      message: All catalogFields are valid.
```

**Только cluster-scoped `grantedResource`.** Definition namespaced-типа не резолвится:
`internal/resolve.grantedGVK` — единственный путь к выданным объектам (листинг живых объектов и
`defaultFrom`) — проверяет scope REST-маппинга и роняет регистрацию с
`resolve.ErrNamespacedGrantedResource`. Каталог такой definition не рендерится, а отрендеренный
раньше удаляется. В этом отличие от неизвестного kind, каталог которого остаётся в последнем
корректном состоянии, потому что kind может появиться после установки его CRD: отказ здесь
окончательный, и старый каталог продолжал бы показывать те самые имена, которые отказ должен скрыть.

**Definition, которая не резолвится, инертна.** Namespaced `grantedResource` и kind, который apiserver
не обслуживает, — конфигурационные ошибки (`resolve.IsConfigurationError` — одно правило на вебхуки и
реконсайлер). Kind, удалённый после того, как обслуживался, тоже считается необслуживаемым: REST-маппер
его ещё знает, а его листинг отвечает 404, что у листинга без имени ничего другого значить не может
(`resolve.ErrGrantedResourceNotServed`). `/is-granted` и `/defaults` пропускают ссылки на такую definition с записью в лог (имя
definition, имя ссылки, причина) и проверяют остальные ссылки запроса как обычно — так же, как
пропускают ссылку, путь которой не вычисляется. Проблему видно в условии `GrantedResourceValid`
самой definition (см. ниже). Оба вебхука работают с `failurePolicy: Fail`, и ответ
ошибкой заблокировал бы запись ресурсов по правилу ссылки во всех проектах из-за одной неверной
регистрации. Реконсайлер каталога пишет такие ошибки в лог на уровне `V(1)` и не возвращает их, так что
проход по-прежнему перепланируется через `ResyncInterval`, а не уходит в back-off; скан нарушений
пропускает definition и сканирует остальные. Все прочие ошибки `Resolve` (сбой листинга, ошибка API)
по-прежнему роняют запрос вебхука и проход реконсайлера. Инертна — значит, не проверяется: пока CRD
выданного kind не обслуживается, тенант может записать любое значение (скажем, аннотацию
`cert-manager.io/cluster-issuer`), а когда CRD появится, UPDATE его сохранит, потому что значения,
уже бывшие в старом объекте, повторно не проверяются. Метрика нарушений покажет это позже.

Вебхук `grantableclusterresourcedefinitions` отвергает такую definition при apply (см.
[Вебхуки](#вебхуки)), а binding reconciler показывает сохранённую как
`GrantedResourceValid=False`/`Namespaced`. Оба спрашивают `resolve.GrantedResourceProblem`, который
идёт через `grantedGVK`, так что отвергают ровно то, что отвергает catalog reconciler. У kind,
которого маппер не знает, scope определить нечем: вебхук его пропускает, condition —
`Unknown`/`KindNotServed`. Reconciler переставляет в очередь каждую definition через
`ResyncInterval` при любом ответе, потому что установку или удаление CRD этого kind больше ничто не
заметит. Отдельный condition, а не часть `CatalogFieldsValid`: namespaced-тип ломает всю definition
— каталог и проверки, — а не только `catalogFields`, и `kubectl describe` тогда называет это своим
именем. Причины отказа для namespaced-типа:

- «кластерный ресурс» по определению cluster-scoped; namespaced-тип — не то, что выдаёт этот
  механизм;
- контроллер работает как `cluster-admin` и листит выданный тип по всему кластеру. Для `Secret` в
  каталог каждого проекта попали бы имена всех Secret'ов кластера, а с `catalogFields` — и их значения
  (`$.data.token`);
- каталог ключуется только именем, так что одноимённые объекты из разных неймспейсов затирали бы
  друг друга.

**REST-маппер грантов.** Резолвер, реконсайлеры каталога и политик и оба вебхука используют свой
REST-маппер поверх discovery, отдельный от маппера менеджера. REST-маппер не забывает kind, который
однажды смаппил, поэтому маппер грантов сбрасывается каждые `ResyncInterval` и при 404 на листинге
выданного kind: удалённый CRD или CRD, пересозданный с другим scope, замечается в пределах
`ResyncInterval`, а не при следующем перезапуске пода. В обратную сторону задержка та же: на CRD,
установленный после последнего заполнения, маппер грантов отвечает no-match, так что `/is-granted` и
`/defaults` пропускают ссылки на этот kind до `ResyncInterval`.

Нет `usageReferences`, нет `measure`, нет `coerceToDefault` — измерения убраны, поведение дефолтинга
переехало на reference.

### GrantableClusterResourceReference

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-pvc
spec:
  grantableClusterResourceName: storageclasses   # с каким definition валидируем
  rule:                                           # какие usage-объекты матчим
    apiGroups:
      - ""
    apiVersions:
      - v1
    resources:
      - persistentvolumeclaims
  fieldPaths:                                     # где ИМЯ, по ресурсам и версиям
    - path: $.spec.storageClassName               # запись без scope = дефолт (для всех ресурсов и версий)
      defaulting: Coerce                          # None | FillEmpty | Coerce
    # Два примера ниже — из ДРУГИХ ссылок: скоуп записи обязан укладываться в spec.rule, а правило
    # этой ссылки — только core/v1 persistentvolumeclaims.
    # версионно-зависимая запись, из ссылки с правилом на networking.k8s.io v1 и v1beta1 ingresses:
    # - apiVersions:
    #     - v1beta1
    #   path: $.metadata.annotations['kubernetes.io/ingress.class']
    #   defaulting: None
    # ресурсно-зависимая запись, из ссылки с правилом на batch/v1 jobs и cronjobs:
    # - apiGroups: [batch]
    #   apiVersions: [v1]
    #   resources: [cronjobs]
    #   path: $.spec.jobTemplate.spec.template.spec.priorityClassName
status:
  observedGeneration: 1
  bound: true                                     # grantableClusterResourceName резолвится
  conditions:
    - type: Bound
      status: "True"
      reason: Resolved                            # Resolved | UnknownResource (промахнулись именем)
    - type: FieldPathsValid
      status: "True"
      reason: Valid                               # Valid | InvalidFieldPaths (в message — все проблемы)
```

**Выбор пути.** Для запроса ресурса `r` в group/version `g/v` берутся записи `fieldPaths`, чьи
`resources`/`apiGroups`/`apiVersions` совпали (пустое измерение матчит всё), и из них выигрывает
самая специфичная: `resources` весит 4, `apiGroups` — 2, `apiVersions` — 1, веса складываются, то
есть один `resources` бьёт `apiGroups` + `apiVersions` вместе. При равных весах побеждает запись,
которая идёт в списке раньше. Безскоупная запись весит 0 и служит fallback. Измерение добавляет вес,
только если действительно сужает запись: список с `*` матчит всё и весит 0, как незаданный, поэтому
`apiGroups: ["*"]` равен безскоупной записи и никогда не бьёт явный скоуп. То же для
`fieldPaths[].resources`: `resources: ["*"]` допустим и ведёт себя ровно как незаданное поле. Минимум одна запись.

**Скоупы должны укладываться в rule.** Два правила связывают `fieldPaths[]` с `rule`, потому что
запрос, к которому не применяется ни одна запись, `/is-granted`, `/defaults` и скан нарушений
пропускают без проверки — молчаливый fail-open:

- *Подмножество.* Каждое значение в `resources`/`apiGroups`/`apiVersions` записи должно входить в
  соответствующее измерение `rule`. `*` в `rule` допускает любое значение; `*` в записи ничего не
  ограничивает и тоже допустим. Запись со скоупом `pod` при `rule.resources: [pods]` — опечатка, которая
  не выбрала бы ничего.
- *Покрытие.* Каждую тройку (group, version, resource), которой соответствует `rule`, должна выбирать
  какая-то запись. Проверка перебирает списки `rule` (в `rule.apiGroups` `*` не бывает, CRD его
  отклоняет); `*` в `rule.apiVersions` или `rule.resources` заменяется заглушкой, которой нет ни в одном
  явном списке, поэтому её покрывает только запись без ограничения по этому измерению. Безскоупный
  fallback — один способ покрыть всё; scoped-записи, вместе покрывающие все сочетания, — другой
  (`rule.resources: [jobs, cronjobs]` и по записи на ресурс).

Скоуп по ресурсу нужен потому, что одного пути на group/version не хватает: в core/v1 у Pod это
`$.spec.priorityClassName`, а у ReplicationController — `$.spec.template.spec.priorityClassName`; в
batch/v1 у Job это `$.spec.template.spec.priorityClassName`, а у CronJob —
`$.spec.jobTemplate.spec.template.spec.priorityClassName`.

Поля `fieldPaths[]`: `{apiGroups?, apiVersions?, resources?, path, match?, defaulting?}`. `match` =
`{fieldPath, equals|in}` (guard, применяется только когда предикат истинен). `defaulting`: `None`
(только валидация), `FillEmpty` (дозаполнить пустое поле дефолтом проекта), `Coerce` (плюс переписать
недопустимое значение — для полей, что предзаполняет встроенный admission).

### ClusterResourceGrantPolicy (без изменений)

Per-проект allow-list и дефолт; `projectSelector` вычисляется для каждого неймспейса по объединению
меток Project и меток неймспейса (при совпадении ключа побеждает неймспейс), так что метка на Project
выбирает все его неймспейсы (`internal/resolve.GrantsForNamespace`, общий для вебхуков и catalog
reconciler), на
ресурс (`resourceName`) задаёт `allowed`/`allowedSelector`/`denied`/`deniedSelector`/`default`/
`availabilityDefault`. Непустой allow-лист или `allowedSelector` подразумевает базу `None`; пустой `allowed: []` — нет.

### AvailableClusterResource

Каталог доступного для проекта (имена, их [поля каталога](#поля-каталога) и дефолт), который
контроллер рендерит в неймспейсы проекта. Живёт ровно столько, сколько живёт его
`GrantableClusterResourceDefinition`: при удалении регистрации контроллер на следующем реконсайле
удаляет каталог из всех неймспейсов проектов. Регистрация, удерживаемая финализатором, считается
удалённой с момента выставления `deletionTimestamp`. Очистка выполняется на каждом реконсайле, даже
если другая регистрация не резолвится; каталог самой сбойной регистрации остаётся в последнем
корректном состоянии — кроме namespaced `grantedResource`, каталог которого удаляется.
Конфигурационная ошибка (namespaced или необслуживаемый kind) пишется в лог, а не возвращается, и не
замедляет пересчёт остальных регистраций.

```yaml
status:
  grantedResourceKind: StorageClass
  default: fast
  availableCount: 1
  available:
    - name: fast
      default: true
      fields:                      # из catalogFields определения; значения сохраняют тип JSON
        provisioner: rbd.csi.ceph.com
        reclaimPolicy: Retain
        allowVolumeExpansion: true
```

#### Поля каталога

Тенанту, который выбирает StorageClass, мало одного имени: нужны provisioner, политика возврата тома,
возможность расширения. Владелец определения перечисляет такие поля в `spec.catalogFields`, а
реконсайлер каталога копирует их значения из выданных объектов в `status.available[].fields`.
Платформа даёт только механизм; какие поля показывать, решает команда-владелец определения.

Правила (проекция — `internal/engine/catalog.go`):

- `name` — ключ в lowerCamelCase (`^[a-z][a-zA-Z0-9]*$`, не длиннее 63 символов), уникальный в списке
  (`listType: map`).
- `path` — **singular query** по RFC 9535: только имена полей и индексы, не больше одного значения.
  Без wildcard, рекурсивного спуска, срезов и фильтров.
- Пути, пересекающиеся с `metadata.managedFields` или аннотацией
  `kubectl.kubernetes.io/last-applied-configuration`, отвергаются. В обоих лежит копия всего объекта,
  поэтому отвергаются и их родители (`$`, `$.metadata`, `$.metadata.annotations`). Сравнение идёт по
  разобранным сегментам, так что `$["metadata"]["managedFields"]` тоже ловится.
- Не больше 10 полей (`maxItems` в схеме; для объекта в обход схемы проекция берёт первые 10, при
  дубликате имени — первое вхождение).
- Значение длиннее 512 байт в JSON-сериализации, отсутствующее значение и `null` не выводятся.
- У одного каталога (одного `AvailableClusterResource`) с полями бюджет 512 КиБ
  (`engine.MaxCatalogFieldsBytes`, проверяет `engine.CatalogSize`: длина JSON-сериализации всего списка
  `status.available` — записи с именами, флагами `default` и полями; точная для `status.available`,
  остальной объект не считается. Записи считаются по одной, и подсчёт останавливается на первой, что
  выходит за бюджет). Каталог сверх бюджета не выводит `fields` ни у одной записи; имена и дефолт
  остаются. Всё или ничего, так что результат зависит только от каталога, а не от порядка чтения
  объектов. Без бюджета сотни объектов × 10 полей × 512 байт вывели бы объект за лимит etcd (~1,5 МиБ):
  запись статуса падала бы, и каталог застывал бы вместе со всеми именами.
- У порога каталог может мигать: значение, меняющее длину, переводит каталог через порог, и он на
  соседних проходах переключается между «с полями» и «без», причём каждое переключение — запись статуса
  в каждом неймспейсе проекта, где каталог рендерится. Гистерезиса нет; каталог так близко к лимиту —
  повод выводить меньше полей или покороче.
- Значения сохраняют тип JSON (`map[string]apiextensionsv1.JSON`; в схеме — `additionalProperties`
  с `x-kubernetes-preserve-unknown-fields`).
- Только для object-backed определений: у value-backed объектов нет, поэтому вебхук там список
  отвергает (сохранённый проекция игнорирует).
- Показывать можно только несекретные данные: их читает каждый пользователь каждого проекта, которому
  доступен объект. Шипящееся определение `storageclasses` не берёт `parameters`: они зависят от
  драйвера, бывают большими и могут называть секреты.

Значения обновляются на каждом реконсайле каталога; на сами выданные объекты вотчей нет, так что
изменение доезжает за `ResyncInterval` (2 минуты).

Что где проверяется:

- **Объявление** (пути, value-backed) отвергается при apply вебхуком `grantableclusterresourcedefinitions`
  (см. [Вебхуки](#вебхуки)), а на сохранённом объекте о нём сообщает condition `CatalogFieldsValid`:
  `True`/`Valid` или `False`/`InvalidCatalogFields` с текстом отказа в message. Определение без
  `catalogFields` тоже получает `True` (message `No catalogFields are declared.`): condition есть у
  каждого определения, и его отсутствие значит только «ещё не отреконсайлено». Оба используют
  `engine.DefinitionProblems`. Невалидная запись, всё же попавшая в кластер, при проекции по-прежнему
  пропускается и больше ничего не ломает.
- **Значения** зависят от объектов и проекта, поэтому condition на кластерном определении их не
  передаст. Значение длиннее 512 байт и каталог сверх бюджета видны по `Warning`-событию
  `CatalogFieldsSkipped` на определении: в нём неймспейс, каталог и пропущенные пары
  `<объект>/<поле>` (первые пять, дальше «and N more»). Событие пишется на каждом проходе с
  пропуском, так что не истекает, пока пропуск есть; спам-фильтр рекордера ограничивает его пачкой,
  а затем одним событием в пять минут на определение при любом числе проектов. У лога такого фильтра
  нет, поэтому он пишется только при смене набора пропусков каталога (неймспейс + определение):
  пропущенные пары и то, превышен ли бюджет, но не размер, который меняется с любым значением
  (хранится их хеш). Строка Info несёт текст события, строка `V(1)` — полный список; проход без
  пропусков забывает каталог, как и проход после удаления определения, так что вернувшийся пропуск
  снова попадёт в лог. По каталогу, а не раз на определение: набор отличается между проектами, а
  проход и так идёт по одному неймспейсу. События, а не метрика: владелец определения смотрит
  `kubectl describe` рядом с `CatalogFieldsValid`, а алертинга по содержимому каталога, который
  метрика бы питала, в модуле нет.

**Почему allow-list в определении, а не маркер в схеме CRD.** Маркер на полях схемы выданного ресурса
(расширение `x-...` «показывать тенантам») выглядит естественнее, но:

- схема CRD принимает только фиксированный набор расширений `x-kubernetes-*`, определённый apiserver'ом,
  а CRD с любым другим ключом отвергается целиком — в том числе с выдуманным `x-kubernetes-...`
  (`field not declared in schema` при server-side apply, `strict decoding error: unknown field` при
  create). К тому же этот префикс зарезервирован за самим Kubernetes. Маркер просто негде держать;
- главные регистрируемые ресурсы (StorageClass, ClusterRole) — встроенные типы, схемы
  CRD для разметки у них нет вовсе;
- opt-out-маркер показывает всем тенантам любое неразмеченное поле и любое поле, добавленное в схему
  позже, без всякого решения. Opt-in-маркер этого лишён, но упирается в два пункта выше. Явный список в
  определении — это и есть opt-in, а шипит его модуль-владелец ресурса, так что решение в любом случае
  остаётся за владельцем.

## Покрытие: какой CRD какую историю закрывает

| CRD / компонент | закрывает истории |
|-----------------|-------------------|
| `GrantableClusterResourceDefinition` | D1 (регистрация), D3 (`defaultFrom`), D4 (`excluded`), D6 (`enforcement: External`), O1 (`status.references`) |
| `GrantableClusterResourceReference` | D2 (регистрация пути), D5 (`defaulting`), O2 (`status.bound`) |
| `ClusterResourceGrantPolicy` | A1 (allow-list), A2 (дефолт), A3 (`availabilityDefault`), A4 (`denied`) |
| `AvailableClusterResource` | T1 (discovery) |
| вебхуки `/is-granted` + `/defaults` | T2 (деёны + дефолтинг) |
| Kubernetes `ResourceQuota` (делегировано) | квота — вне области |

У каждой истории есть владелец; ни одна история не осталась без покрытия, и ничего в модели нет без
истории.

## Резолв доступности

Приоритет «может ли проект P использовать имя N ресурса R»:
`excluded → denied → allowed → policy availabilityDefault → registration defaultAvailability`.
Реализован в одном месте (`internal/resolve`), общем для вебхука и контроллера.

## Дефолтинг

Per-путь (`fieldPaths[].defaulting`):

- `None` — только валидация. Для ссылки, отсутствие которой осмысленно (аннотация-переключатель
  `cert-manager.io/cluster-issuer`).
- `FillEmpty` — на CREATE дозаполнить пустое поле дефолтом проекта.
- `Coerce` — `FillEmpty` плюс переписать недопустимое значение в дефолт (поля, предзаполняемые
  встроенным admission, напр. `DefaultStorageClass` у PVC). О замене автору сообщается admission
  warning с исходным и подставленным значением.

`FillEmpty` и `Coerce` ограничивают путь. Валидация читает значение полноценным RFC 9535-вычислителем,
а дефолтинг пишет JSON Patch, и ему нужно ровно одно однозначное место: `path` должен быть простым
путём по именам полей (`$.spec.storageClassName`, `$.metadata.annotations['cert-manager.io/cluster-issuer']`),
без подстановочных знаков, индексов и фильтров. Reference, нарушающий это, отклоняется в момент
применения валидирующим вебхуком `GrantableClusterResourceReference`
(`/validate/v1alpha1/grantableclusterresourcereferences`), а не биндится, чтобы потом молча ничего не
дефолтить. С `None` подходит любой корректный RFC 9535-путь. Проверка простого пути берёт сегменты из
того же дерева разбора RFC 9535, что и вычислитель, поэтому экранирования декодируются одинаково с обеих
сторон; пустое имя поля (`$['']`) она тоже отвергает.

Значение дефолта берётся из `default` политики, fallback — `defaultFrom` definition; `defaultFrom`
принимает объект, только если значение аннотации — `true` (без учёта регистра), так что класс с
`is-default-class: "false"` дефолтом не является.

## Вебхуки

Генерируются из набора `GrantableClusterResourceReference` (их `rule` задают перехватываемые GVK —
регистрация reference автоматически расширяет перехват на CRD модуля):

- **`/is-granted`** (validating) — по GVK запроса находим подходящие references → их definition →
  деним, если имя недоступно проекту. На UPDATE уже присутствующие значения grandfather'ятся.
  Reference, у которого выбранный `path` или `match.fieldPath` не вычисляется, логируется (имя
  reference, индекс записи, путь) и пропускается; остальные references проверяются как обычно. Это
  осознанный fail-open только для сломанной ссылки: вебхук references работает с
  `failurePolicy: Ignore`, так что такой объект всё равно может оказаться в кластере, а ответ ошибкой
  при `failurePolicy: Fail` этого вебхука заблокировал бы CREATE/UPDATE всех ресурсов её `rule` во
  всех проектах из-за одного битого объекта. Видимость дают лог и `FieldPathsValid=False` у reference.
  Ошибки, не относящиеся к одной ссылке (список references, чтение namespace или grants, декодирование,
  резолв), по-прежнему валят запрос. `/defaults` и скан нарушений такие ссылки уже пропускали и не
  менялись.
- **`/defaults`** (mutating, CREATE) — применяем `fieldPaths[].defaulting`.

Регистрируется статически, не выводится из references:

- **`/validate/v1alpha1/grantableclusterresourcereferences`** (validating, CREATE/UPDATE) — отклоняет
  элемент `fieldPaths[]`, у которого `path` или `match.fieldPath` не компилируется RFC 9535-парсером,
  которым пользуется `/is-granted` (при любом `defaulting`: сохранённый некомпилируемый путь `/is-granted`
  пропускает, и reference молча не проверял бы ничего),
  либо у которого `defaulting` не `None`, а `path` не является простым путём по именам полей, а также
  spec, нарушающий правило подмножества или покрытия (см. *Скоупы должны укладываться в rule*). Все
  проблемы перечисляются в одном отказе. На UPDATE пути проверяются только у элементов, новых или
  изменённых относительно `oldObject` (сравнение по содержимому, не по индексу); скоуп элемента
  перепроверяется, если изменился элемент или `rule` (удаление ресурса из `rule` не должно оставить
  сохранённый элемент со скоупом на него); покрытие — свойство всего spec — перепроверяется, если
  изменились `rule` или `fieldPaths`. Объект с `deletionTimestamp` не проверяется вовсе. Так reference,
  сохранённый до вебхука или пока тот был недоступен, остаётся редактируемым (метаданные,
  финализаторы), а не отвергается на каждой записи; исправленный элемент — изменённый, он проверяется
  и проходит. Правила живут в одном месте, `engine.ReferenceProblems`, общем с binding reconciler.
  `failurePolicy: Ignore` и без исключения системных писателей: эти объекты пишут
  разработчики модулей (на стенде — `system:masters`) и deckhouse-контроллер, применяющий релиз
  модуля, так что исключение не оставило бы вебхуку никого; `Ignore` не даёт недоступному бэкенду
  заблокировать релиз.
- **`/validate/v1alpha1/grantableclusterresourcedefinitions`** (validating, CREATE/UPDATE) — отклоняет
  namespaced `grantedResource` (см. [Только cluster-scoped `grantedResource`](#grantableclusterresourcedefinition)),
  элемент `catalogFields[]`, чей `path` отвергает `engine.CompileCatalogPath` (не компилируется, не
  singular query, пересекается с запрещённым путём), и `catalogFields` у value-backed определения. Все
  проблемы в одном отказе: сначала scope, затем элементы с индексом. Scope берётся из REST-маппера
  грантов (см. [REST-маппер грантов](#grantableclusterresourcedefinition)) — того же, с которым резолвят
  реконсайлеры каталога и определений, поэтому вебхук отвергает ровно то, что отвергают они:
  неизвестный ему kind (no matches for kind) пропускается, любая другая ошибка маппинга логируется и
  тоже пропускается, потому что сбой discovery не должен отвергать релиз модуля. Kind берётся из тела
  запроса, но этот маппер повторяет discovery API только один раз после каждого сброса, так что
  выдуманная группа своего discovery не стоит; kind, чей CRD установлен после последнего сброса,
  пропускается до следующего (не дольше `ResyncInterval`), и о нём сообщает реконсайлер. На
  UPDATE scope проверяется, только если изменился `grantedResource`, элементы — только новые или
  изменённые относительно `oldObject` (по содержимому, не по индексу), а правило value-backed —
  только если изменились `catalogFields` или `grantedResource`; объект с `deletionTimestamp` не
  проверяется.
  `failurePolicy: Ignore` и без исключения системных писателей по тем же причинам, что у вебхука
  references: определения шипят модули через deckhouse-контроллер. Имена, их уникальность и лимит 10
  держит схема. Правила живут в `engine.DefinitionProblems`, общем с binding reconciler.
- **`/protect`** (validating) — держим `AvailableClusterResource` read-only (с исключениями для
  системных групп). Статуса квоты больше нет.

`/is-granted`, `/defaults`, вебхуки references и definitions и reconciler'ы project, reference и
definition делят одну фабрику JSONPath
(`jsonpath.NewWithCache` в `cmd/main.go`), поэтому путь везде компилируется одинаково. Её кеш
распарсенных путей — ограниченный LRU (`MaxCachedPaths`, 1024 записи; выражения длиннее
`MaxCachedPathLen`, 256 байт, парсятся, но не кешируются; ошибки парсинга не кешируются никогда).
Граница нужна, потому что в кеш попадают и пути из отвергнутых и dry-run объектов, которые не
сохраняются, а webhook-сервер принимает запросы от любого пода без клиентского сертификата:
неограниченный кеш можно раздуть до OOM контроллера, а вместе с ним упадёт и `/is-granted` с
`failurePolicy: Fail`. Легитимных путей — из сохранённых references, definitions и их match-guard'ов —
десятки или сотни, они помещаются с большим запасом.

## Контроллер

- **Catalog reconciler** (по namespace) — рендерит `AvailableClusterResource` per-проект per-definition
  из резолва доступности и удаляет принадлежащие модулю каталоги неймспейса, у которых больше нет
  definition (каталог read-only для всех, кроме контроллера, — удалить его больше некому).
  Поля каталога, пропущенные из-за размера, он сообщает событиями `CatalogFieldsSkipped` на
  определении (см. [Поля каталога](#поля-каталога)).
- **Binding reconciler** (по `GrantableClusterResourceReference` и
  `GrantableClusterResourceDefinition`) — проставляет `reference.status.bound`/condition `Bound` и
  обратный индекс `definition.status.references`/`referenceCount`. Также ставит reference condition
  `FieldPathsValid`: проверки вебхука, применённые ко всему сохранённому объекту без ratcheting, так что
  reference, сохранённый в обход вебхука, виден (`False`/`InvalidFieldPaths`, message — текст отказа).
  Определению так же ставится `CatalogFieldsValid` (`False`/`InvalidCatalogFields`), а для scope его
  `grantedResource` — `GrantedResourceValid`.
- **Policy reconciler** (по `ClusterResourceGrantPolicy`) — выставляет `SelectorsValid` (селектор,
  который схема принимает, а библиотека селекторов отвергает, иначе молча не матчил бы ничего) и
  `AllowedEffective` (allowed-имя, которое фильтр `excluded` definition всё равно отвергает, ничего не
  даёт; для ClusterRole сообщение называет лейбл `rbac.deckhouse.io/delegatable`).
- **Скан нарушений** (внутри catalog reconciler) — после каждого рендера каталога обходит
  перехватываемые объекты неймспейса и отдаёт `d8_cluster_objects_grant_violated{project,grant,
  violating_resource,violating_object_name,violating_field}` на метрик-эндпоинте контроллера `:9091` для
  объектов, чьё имя больше недоступно (на UPDATE они grandfather'ятся, так что видны только здесь).

## Примеры

**StorageClass** — definition + путь PVC (PVC-путь `defaulting: Coerce` — встроенный DefaultStorageClass):

```yaml
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
spec:
  grantedResource:
    apiGroup: storage.k8s.io
    kind: StorageClass
  defaultAvailability: All
  defaultFrom:
    annotationKey: storageclass.kubernetes.io/is-default-class
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-pvc
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups:
      - ""
    apiVersions:
      - v1
    resources:
      - persistentvolumeclaims
  fieldPaths:
    - path: $.spec.storageClassName
      defaulting: Coerce
```

**Косвенность (PostgresDatabase → PVC).** `PostgresDatabase` ссылается на StorageClass, оператор под
капотом создаёт PVC. Регистрируем validation-only reference для CRD; PVC валидируется своим reference.
Оба валидируются; квоты нет — проблемы двойного учёта нет:

```yaml
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-postgres
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups:
      - acid.zalan.do
    apiVersions:
      - v1
    resources:
      - postgresqls
  fieldPaths:
    - path: $.spec.volume.storageClass
      defaulting: None
```

**ClusterIssuer — два пути.** Certificate (`spec.issuerRef`, guard `kind == ClusterIssuer`,
`FillEmpty`) и аннотация Ingress (переключатель — `defaulting: None`, никогда не заполняется):

```yaml
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: clusterissuers
spec:
  grantedResource:
    apiGroup: cert-manager.io
    kind: ClusterIssuer
  defaultAvailability: All
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterissuers-certificate
spec:
  grantableClusterResourceName: clusterissuers
  rule:
    apiGroups:
      - cert-manager.io
    apiVersions:
      - v1
    resources:
      - certificates
  fieldPaths:
    - path: $.spec.issuerRef.name
      match:
        fieldPath: $.spec.issuerRef.kind
        equals: ClusterIssuer
      defaulting: FillEmpty
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterissuers-ingress
spec:
  grantableClusterResourceName: clusterissuers
  rule:
    apiGroups:
      - networking.k8s.io
    apiVersions:
      - "*"
    resources:
      - ingresses
  fieldPaths:
    - path: $.metadata.annotations['cert-manager.io/cluster-issuer']
      defaulting: None
```

**ClusterRole** — availability-only; делегируемый набор через метку `rbac.deckhouse.io/delegatable`:

```yaml
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: clusterroles
spec:
  grantedResource:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
  defaultAvailability: All
  excluded:
    - matchExpressions:
        - key: rbac.deckhouse.io/delegatable
          operator: NotIn
          values: ["true"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterroles-rolebinding
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups:
      - rbac.authorization.k8s.io
    apiVersions:
      - v1
    resources:
      - rolebindings
  fieldPaths:
    - path: $.roleRef.name
      match:
        fieldPath: $.roleRef.kind
        equals: ClusterRole
      defaulting: None
```

## Почему без квоты

Kubernetes `ResourceQuota` уже покрывает то, что фича квотировала бы, и делает это race-free
(резервирование в квота-контроллере) на **терминальном потребителе**:

- per-storage-class storage и счёт PVC — нативно (`<sc>.storageclass.storage.k8s.io/...`), и проект уже
  рендерит `ResourceQuota`;
- суммарный счёт любого ресурса — `count/<resource>.<group>`;
- LoadBalancer/NodePort — `services.loadbalancers`/`services.nodeports`.

Чего `ResourceQuota` не выражает — узко (per-name счёт для не-storage, per-name внутри произвольных
CRD, суммирование произвольных quantity-полей). Сегодня ни один поставляемый ресурс этого не требует
(per-`loadBalancerClass`-value счёт спорен; хватает суммарного). Поэтому квота убрана; при появлении
конкретной потребности per-name/CRD её вводят позже и сразу race-safe через резервирование в status.

## Что убрано относительно прежнего дизайна

`ClusterResourceGrant` (пул квоты) и всё измерение: поля `measure`/`countable`/`quantities`, квота-ветка
вебхука, `internal/quota`, per-namespace rendered-объекты квоты. `usageReferences` ушли из
`GrantableClusterResourceDefinition` в новый CRD `GrantableClusterResourceReference`. `coerceToDefault`
ушёл из definition в `fieldPaths[].defaulting: Coerce`.
