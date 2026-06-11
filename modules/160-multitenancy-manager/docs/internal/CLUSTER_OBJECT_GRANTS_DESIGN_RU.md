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
  conditions: []                   # стандартный Ready, ставит контроллер
```

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
  fieldPaths:                                     # где ИМЯ, версионно
    - path: $.spec.storageClassName               # запись без scope = дефолт (для всех версий)
      defaulting: Coerce                          # None | FillEmpty | Coerce
    # версионно-зависимый пример (класс IngressClass переезжал между версиями):
    # - apiVersions:
    #     - v1beta1
    #   path: $.metadata.annotations['kubernetes.io/ingress.class']
    #   defaulting: None
status:
  observedGeneration: 1
  bound: true                                     # grantableClusterResourceName резолвится
  conditions:
    - type: Bound
      status: "True"
      reason: Resolved                            # Resolved | UnknownResource (промахнулись именем)
```

**Выбор пути.** Для запроса group/version `g/v` берётся запись `fieldPaths`, чьи
`apiGroups`/`apiVersions` совпали; более специфичная (со scope) бьёт безскоупную; безскоупная —
fallback. Минимум одна запись; fallback рекомендуется.

Поля `fieldPaths[]`: `{apiGroups?, apiVersions?, path, match?, defaulting?}`. `match` =
`{fieldPath, equals|in}` (guard, применяется только когда предикат истинен). `defaulting`: `None`
(только валидация), `FillEmpty` (дозаполнить пустое поле дефолтом проекта), `Coerce` (плюс переписать
недопустимое значение — для полей, что предзаполняет встроенный admission).

### ClusterResourceGrantPolicy (без изменений)

Per-проект allow-list и дефолт; выбирает проекты по меткам неймспейсов через `projectSelector`, на
ресурс (`resourceName`) задаёт `allowed`/`allowedSelector`/`denied`/`deniedSelector`/`default`/
`availabilityDefault`. Allow-лист подразумевает базу `None`.

### AvailableClusterResource (без изменений)

Каталог доступного для проекта (имена + дефолт), который контроллер рендерит в неймспейсы проекта.

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
  встроенным admission, напр. `DefaultStorageClass` у PVC).

Значение дефолта берётся из `default` политики, fallback — `defaultFrom` definition.

## Вебхуки

Генерируются из набора `GrantableClusterResourceReference` (их `rule` задают перехватываемые GVK —
регистрация reference автоматически расширяет перехват на CRD модуля):

- **`/is-granted`** (validating) — по GVK запроса находим подходящие references → их definition →
  деним, если имя недоступно проекту. На UPDATE уже присутствующие значения grandfather'ятся.
- **`/defaults`** (mutating, CREATE) — применяем `fieldPaths[].defaulting`.
- **`/protect`** (validating) — держим `AvailableClusterResource` read-only (с исключениями для
  системных групп). Статуса квоты больше нет.

## Контроллер

- **Catalog reconciler** (по namespace) — рендерит `AvailableClusterResource` per-проект per-definition
  из резолва доступности.
- **Binding reconciler** (по `GrantableClusterResourceReference` и
  `GrantableClusterResourceDefinition`) — проставляет `reference.status.bound`/condition `Bound` и
  обратный индекс `definition.status.references`/`referenceCount`.

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
          operator: DoesNotExist
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
