# Мультинеймспейсные проекты — дизайн

> Статус: **черновик / обсуждение.** Как проект Deckhouse эволюционирует от «проект == один
> namespace» к проекту, который его администратор может делить на несколько namespace — *опционально*,
> потому что self-service нужен не всем. Соседний документ —
> [Гранты на кластерные ресурсы](./CLUSTER_OBJECT_GRANTS_DESIGN_RU.md) (per-project гранты/квоты).

## Проблема

Сейчас проект — это ровно один namespace (1:1): `Project` создаёт namespace с тем же именем. Хотим
проект, внутри которого админ может создавать **несколько** namespace. Но self-service не бесплатен —
для многих проектов один namespace проще и безопаснее. Поэтому **режим выбирается при создании**
проекта, а single-namespace остаётся по умолчанию.

## Режимы

Выбирается один раз, при создании, через `Project.spec.namespaces.selfService`:

- **Single-namespace (по умолчанию, классика).** Проект ⇔ один namespace с именем проекта. Текущее
  поведение; ни `ProjectNamespace`, ни префиксов. Для существующих проектов ничего не меняется.
- **Multi-namespace (self-service).** Проект — логический контейнер. Его админ создаёт рабочие
  namespace через [`ProjectNamespace`](#projectnamespace). Закрытый
  [control namespace](#control-namespace) с именем проекта держит только управляющие объекты.

## Изменения спеки Project (first-class поля)

По мотивам Capsule `Tenant`. Новые/изменённые `Project.spec`:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: team-a
  labels:
    environment: production        # используется в ClusterResourceGrantPolicy.projectSelector
spec:
  projectTemplateName: default
  parameters: {}
  # Кто администрирует проект; контроллер биндит их во всех namespace проекта + control ns.
  owners:
  - kind: Group                    # User | Group | ServiceAccount
    name: team-a-admins
    accessLevel: Admin             # маппится на Deckhouse access-level / ClusterRole
  # Политика namespace для проекта.
  namespaces:
    selfService: true              # false (по умолчанию) = классический single-namespace проект
    prefix: team-a                 # обязателен в multi-ns; по умолчанию = имя проекта
    max: 10                        # макс. число namespace в проекте (Capsule namespaceOptions.quota)
  # Pod Security Standard проекта (эффективный enforce = max(cluster floor, это)).
  podSecurityStandard: Restricted          # Privileged | Baseline | Restricted
  # Дефолтное размещение подов проекта — проставляется на каждый namespace проекта (см. Фичи по умолчанию).
  nodeSelector:
    node-pool: tenants
  tolerations:
  - key: dedicated
    operator: Equal
    value: tenants
    effect: NoSchedule
  # Per-project тогглы фич, проброшенные в другие модули (no-op, если модуль выключен) — см. Фичи по умолчанию.
  features:
    vulnerabilityScanning: true            # operator-trivy сканирует namespace проекта
    monitoring: true                       # мониторинг тенанта: scrape + PodMonitor/PrometheusRule + Grafana
  # Общая COMPUTE-квота проекта (нативные ключи ResourceQuota), распределяется по namespace.
  # Квота ОБЪЕКТОВ (per-class storage/LB/… лимиты) живёт в отдельном ClusterResourceGrant — см. Квоты.
  quota:
    compute:
      requests.cpu: "40"
      requests.memory: 80Gi
      limits.cpu: "60"
      limits.memory: 120Gi
      pods: "400"
```

| Поле | Почему first-class |
|------|--------------------|
| `owners[]` | идентичность — на уровне проекта, должна применяться ко всем его namespace; сейчас RBAC рендерится шаблоном, что не годится для растущего набора namespace |
| `namespaces.selfService` | переключатель режима (single vs multi) |
| `namespaces.prefix` / `max` | именование + лимит числа namespace — проектные инварианты, проверяются при каждом создании namespace |
| `podSecurityStandard` | first-class PSS-уровень (`Privileged`/`Baseline`/`Restricted`); ставит `pod-security.kubernetes.io/enforce`, эффективный = max(cluster floor, это) |
| `nodeSelector` / `tolerations` | дефолтное размещение подов проекта; проставляется на каждый namespace проекта (см. [Фичи по умолчанию](#фичи-по-умолчанию-для-проектов-baseline)) |
| `features.vulnerabilityScanning` / `features.monitoring` | first-class тогглы, проброшенные в `operator-trivy` / стек мониторинга для namespace проекта (метки + RBAC); no-op, если модуль выключен |
| `quota` | общий **compute**-бюджет проекта (нативные ключи `ResourceQuota`), распределяемый по namespace; квота объектов живёт в `ClusterResourceGrant` (см. [Квоты](#квоты)) |

Пер-namespace *рендеримые* ресурсы (NetworkPolicy, LimitRange, security-профиль, дефолтный RBAC)
остаются в `ProjectTemplate` (см. [ProjectTemplate в multi-namespace](#projecttemplate-в-multi-namespace)).

## ProjectNamespace

В режиме multi-namespace админ проекта заказывает рабочий namespace ресурсом `ProjectNamespace` —
**namespaced**-ресурсом, валидным **только в основном (control) namespace проекта** (чтобы управлять
обычным namespace-scoped RBAC, без кластерных прав). В любом другом namespace admission его
**отклоняет**.

`ProjectNamespace` — это одновременно **namespace claim** и **quota claim** этого namespace: его
`spec.quota` (слайс `compute`/`objects`) — порция бюджета проекта для этого namespace (контроллер
рендерит нативный `ResourceQuota` из `compute` и per-namespace `ClusterResourceGrant` из `objects`, держа
`Σ слайсов ≤ пул`; compute-пул: `Project.spec.quota.compute`; пул объектов: `ClusterResourceGrant` проекта; см.
[Квоты](#квоты)).

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectNamespace
metadata:
  name: backend                 # суффикс; итоговый namespace = <prefix>-<suffix>
  namespace: team-a             # control namespace проекта
spec:
  quota:                        # слайс общего лимита проекта для этого namespace (опционально)
    compute:
      requests.cpu: "8"
      requests.memory: 16Gi
    objects:
      storageclasses:
        fast:
          requests.storage: 50Gi
      loadbalancerclasses:
        external:
          services: 2
status:
  namespace: team-a-backend     # созданный контроллером namespace
  appliedQuota:                 # весь реально выданный слайс (каждый пункт с клампом до остатка бюджета)
    compute:
      requests.cpu: "8"
      requests.memory: 16Gi
    objects:
      storageclasses:
        fast:
          requests.storage: 50Gi
      loadbalancerclasses:
        external:
          services: 2
  conditions: []
```

Контроллер создаёт `Namespace` `<prefix>-<suffix>` (здесь `team-a-backend`), вешает метку
`projects.deckhouse.io/project=team-a`, рендерит в него пер-namespace ресурсы `ProjectTemplate`
(сетевая изоляция, лимиты, безопасность — см. [Сетевую изоляцию и безопасность](#сетевая-изоляция-и-безопасность)),
рендерит нативный `ResourceQuota` из `spec.quota.compute` и per-namespace `ClusterResourceGrant` из
`spec.quota.objects`. `status.appliedQuota` отражает **весь** применённый слайс (каждый пункт `compute`
и `objects`, с клампом по ключу до остатка бюджета проекта — не только CPU). Удаление `ProjectNamespace`
удаляет namespace и возвращает слайс в бюджет проекта.

## ProjectRoleBinding

Доступ на весь проект без правки каждого namespace. `Project.spec.owners` — шорткат для
администраторов проекта; `ProjectRoleBinding` — общая форма (любой subject, любая роль), которой
админ проекта раздаёт доступ команде в self-service. Это **namespaced**-ресурс, валидный **только в
основном (control) namespace проекта** (в другом admission отклоняет); контроллер фанаутит
`RoleBinding` в **каждый** namespace проекта.

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectRoleBinding
metadata:
  name: developers
  namespace: team-a             # control namespace проекта
spec:
  subjects:
  - kind: Group
    name: team-a-developers
  accessLevel: Editor           # Deckhouse access level, либо roleRef на ClusterRole
status:
  namespaces: [team-a, team-a-backend, team-a-frontend]   # где созданы биндинги
  conditions: []
```

Когда в проект добавляется новый namespace, контроллер распространяет в него существующие
`ProjectRoleBinding` (доступ следует за проектом, а не за фиксированным списком namespace). Админ
проекта может выдать роль не выше своего уровня доступа (guard от эскалации).

## Именование и префикс

Мультинеймспейсным проектам нужны имена без коллизий:

- `Project.spec.namespaces.prefix` **обязателен** (по умолчанию = имя проекта). Control namespace —
  `<prefix>`; каждый рабочий namespace — `<prefix>-<suffix>`.
- **Зарезервированные префиксы запрещены:** `d8-`, `kube-`, `upmeter-`, `default` и любой
  существующий системный namespace.
- **Валидация коллизии префикса** (admission на create/update `Project`). Отклоняем префикс `P`, если
  он **пересекается** с чем-либо, где пересечение = одна строка является префиксом другой:
  - `P` пересекается с зарезервированным префиксом (например, зарезервирован `d8` ⇒ `d8-test`
    отклоняется);
  - `P` пересекается с префиксом другого проекта (`team` vs `team-a` — в обе стороны);
  - любой существующий namespace, **не** принадлежащий этому проекту, равен `P` или начинается с `P-`.
  Реализация: вести индекс префиксов проектов; вебхук проверяет `P` против
  (зарезервированные ∪ префиксы других проектов ∪ имена существующих namespace) на отношение «префикс».
- Суффикс `ProjectNamespace` валидируется, чтобы итоговое имя было RFC1123 и в пределах длины.

## ProjectTemplate в multi-namespace

Сейчас `ProjectTemplate` рендерит ресурсы в единственный namespace проекта. В multi-namespace:

- Ресурсы шаблона рендерятся в **каждый рабочий namespace** при его создании — шаблон
  параметризуется **неймспейсом**, а не проектом (`.namespace` вместо неявного единственного
  namespace).
- Разделение по «высоте»:
  - **пер-namespace** (NetworkPolicy, LimitRange, дефолтный RBAC, security-профиль, слайс квоты) —
    рендерится в каждый namespace;
  - **пер-проект** (биндинги owner'ов, compute-квота, политика namespace) — переезжает в `Project.spec`
    (выше); квота объектов — в `ClusterResourceGrant` проекта, а не в шаблон.

Открыто: рендерим весь шаблон пер-namespace или даём автору шаблона помечать ресурсы как
пер-namespace / пер-проект? Склоняюсь: по умолчанию пер-namespace; проектное — это поля `Project.spec`,
а не объекты шаблона.

## Control namespace

В multi-namespace контроллер всё равно создаёт namespace с именем проекта (**control namespace**), но
он **не рабочий**:

- **validating-вебхук** разрешает в нём только белый список kinds — `ProjectNamespace`,
  `ProjectRoleBinding`, `ClusterResourceGrant` проекта (пул квоты объектов), read-only каталог `AvailableClusterResource`
  — и **отклоняет ворклоады** (Pods, Deployments, Services, PVC, …). И наоборот: `ProjectNamespace` и
  `ProjectRoleBinding` валидны **только** здесь и отклоняются в любом другом namespace;
- это консоль админа проекта: заказывать namespace, `kubectl get available`, управлять
  owner-биндингами.

В single-namespace отдельного control namespace нет — единственный namespace проекта и есть рабочий,
как сейчас.

## Сетевая изоляция и безопасность

Обе — **на уровне проекта** (должны покрывать каждый его namespace) и обе опираются на метки, которые
контроллер ставит на namespace проекта. Два предусловия, которые гарантирует контроллер на каждом
namespace проекта:

- `projects.deckhouse.io/project=<project>` — идентифицирует проект (per-project изоляция);
- **проброшенные метки `Project`** (например `environment=production`) — копируются с `Project` на его
  namespace (идея Capsule `additionalMetadata`), чтобы админ мог таргетить *класс* проектов по метке.

Но производятся они **разными** акторами/триггерами — это то, что прошлый черновик переврал:

### Сетевая изоляция — рендерит контроллер (триггер: создание namespace)

Сетевая изоляция — часть `ProjectTemplate`. **Контроллер рендерит её в namespace при его создании**
(на реконсиляции `ProjectNamespace`) и перерендеривает во все namespace проекта при изменении
шаблона. Админ проекта `NetworkPolicy` **не** пишет.

Единственное, что multi-namespace меняет в *существующей* template-политике: сейчас она изолирует
**один** namespace; в multi-namespace проекте соседние namespace должны видеть друг друга, поэтому
внутрипроектное правило должно выбирать **все namespace проекта по метке**, а не только локальный —
сохраняя всё, что шаблон уже разрешает (default-deny, DNS/системный egress, ingress-контроллеры).
Несущая правка — только селектор:

```yaml
# внутри template-рендеримой NetworkPolicy — правило, разрешающее внутрипроектный трафик:
- from:
  - namespaceSelector:
      matchLabels:
        projects.deckhouse.io/project: team-a   # каждый namespace ЭТОГО проекта, не только локальный
```

Валидная отдельная NetworkPolicy обязана также сохранить DNS/системный egress (иначе поды не
резолвят имена) — это уже есть в текущем шаблоне и не меняется; multi-namespace лишь расширяет
внутрипроектный селектор с «этого namespace» на «этот проект». То есть существующую template-политику
не заменяем, только расширяем её внутрипроектный селектор.

### Security-политики — преднастраивает админ, матчит по лейблу (как гранты)

PSS / seccomp / capabilities **не** рендерятся на проект. Cluster-admin **преднастраивает**
`SecurityPolicy` (admission-policy-engine) один раз и **навешивает на проекты по label-селектору** —
ровно модель «author once, match by label» как у `ClusterResourceGrantPolicy`. Задача контроллера — лишь
обеспечить нужную метку на каждом namespace проекта (через проброшенные метки `Project` выше).

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: restricted-production        # админский, переиспользуется на многих проектах
spec:
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          environment: production     # проброшено с Project на его namespace
  policies:
    allowPrivileged: false
    runAsUser:
      rule: MustRunAsNonRoot
    seccompProfiles:
      allowedProfiles:
      - RuntimeDefault
```

Итого: **сетевая изоляция = наш рендеринг шаблона** (триггер: создание namespace / изменение шаблона;
селектор расширен до проекта); **безопасность = преднастроенный админом `SecurityPolicy`, навешенный
на проекты по лейблу** (как гранты). Контроллер владеет только метками namespace, которые это
включают.

## Фичи по умолчанию для проектов (baseline)

Cluster-admin хочет, чтобы для проектов **по умолчанию** что-то выполнялось — PSS Restricted везде,
набор `SecurityPolicy`/`OperationPolicy`, поды на определённых нодах. Это делится по **скоупу** (все
проекты / класс / один проект) и **силе** (*дефолт*, который проект может переопределить, vs *жёсткий
floor*, который можно только ужесточить). Механизм единый: **контроллер всегда проставляет служебные
метки на каждый namespace проекта, а политики выбирают эти метки.** Так как метка принадлежности
*всегда есть*, политика, выбирающая её, покрывает **все** проекты и **не зависит от шаблона** —
небрежный `ProjectTemplate` её не обойдёт.

Три слоя, от сильного к локальному:

1. **Floor — все проекты, ослабить нельзя.** Cluster-admin задаёт `SecurityPolicy` / PSS /
   `OperationPolicy`, чей `namespaceSelector` матчит `projects.deckhouse.io/project` (Exists) (обычно ещё
   `namespace-role: workload`, чтобы пропустить control namespace). Один объект, все проекты, без
   per-project работы. Контроллер лишь гарантирует метку.
2. **Класс — проекты с общей проброшенной меткой.** То же, но селектор матчит проброшенную метку
   `Project` (например `environment: production`). Админ вешает метку на `Project`; контроллер
   пробрасывает **allowlist** ключей на namespace.
3. **Per-project — first-class поля `Project.spec` (слой per-project).** `podSecurityStandard`,
   `nodeSelector`/`tolerations`, тогглы `features.*`; `ProjectTemplate` рендерит прочие per-namespace
   экстра. Они могут добавлять/ужесточать, но не ослаблять floor.

Два рода фич, по **тому, кто действует на метку**:

- **энфорсит этот контроллер** (PSS, размещение) — мы сами пишем метку/аннотацию namespace;
- **делегировано другому модулю** (`features.*`) — мы только проставляем метку/аннотацию + выдаём RBAC,
  которые потребляет *другой* модуль; фича живёт там, поэтому тоггл — **no-op, если модуль выключен**.

Конкретно по кейсам:

- **`SecurityPolicy` на всех namespace проекта** → `SecurityPolicy`, выбирающий метку принадлежности
  (floor). Ровно модель «author once, match by label» с всегда-присутствующей меткой.
- **PSS Restricted всегда** → контроллер ставит `pod-security.kubernetes.io/enforce` из
  `Project.spec.podSecurityStandard`; **эффективный уровень = max(floor, запрос проекта)**, так что
  проект может ужесточить (`restricted` поверх floor `baseline`), но не ослабить.
- **Поды всегда на определённые ноды** → `Project.spec.nodeSelector`/`tolerations`; контроллер ставит
  аннотацию `scheduler.alpha.kubernetes.io/node-selector` на каждый namespace проекта (плагин
  `PodNodeSelector` инжектит её в поды). Для *кластерного* пина — admission-policy-мутация по метке
  принадлежности/класса делает то же по всему флоту.
- **Сканирование на уязвимости** → `Project.spec.features.vulnerabilityScanning`; контроллер помечает
  namespace проекта для `operator-trivy` и выдаёт тенанту read на namespaced `VulnerabilityReport`. Мы
  не сканируем — это делает `operator-trivy`.
- **Мониторинг** → `Project.spec.features.monitoring`; контроллер помечает namespace для scrape и выдаёт
  тенанту RBAC на создание `PodMonitor`/`ServiceMonitor`/`PrometheusRule` и скоуп Grafana. Пайплайн
  метрик — у стека мониторинга, не у нас.

**Control namespace** исключается из workload-ориентированных дефолтов (размещение, внутрипроектный
`NetworkPolicy`, сканирование) через `namespace-role: control` — там управляющие объекты, не поды.

## Служебные метки и аннотации

Контроллер проставляет согласованный набор меток, чтобы политики могли таргетить namespace, владение
было явным, а рендеримые объекты ссылались на свой источник (для обновления и GC).

**На каждом управляемом контроллером объекте** (namespace проекта, `AvailableClusterResource`, рендеримый
`ClusterResourceGrant`, рендеримые `RoleBinding`/`NetworkPolicy`/`ResourceQuota`):

| метка | значение | назначение |
|-------|----------|------------|
| `projects.deckhouse.io/project` | `<project>` | **принадлежность проекту** — ключ связи; `get -A -l projects.deckhouse.io/project=team-a` покажет всё по проекту; на неё опирается GC контроллера |
| `heritage` | `deckhouse` | объект под управлением Deckhouse (существующая конвенция) |
| `module` | `multitenancy-manager` | владелец-модуль — защитная admission-политика запрещает запись в объекты с этой меткой не-контроллерными service account |

**На namespace проекта дополнительно:**

| метка / аннотация | значение | назначение |
|-------------------|----------|------------|
| `kubernetes.io/metadata.name` | `<namespace>` | ставит kube-apiserver; позволяет таргетить namespace по имени |
| `projects.deckhouse.io/namespace-role` | `control` \| `workload` | lockdown control namespace; таргетить только рабочие namespace |
| `pod-security.kubernetes.io/enforce`\|`warn`\|`audit` | `<level>` | PSS; эффективный `enforce` = max(floor, запрос проекта) |
| проброшенные ключи `Project` (allowlist), напр. `environment` | из `Project.metadata.labels` | таргетить *класс* проектов по метке |
| `scheduler.alpha.kubernetes.io/node-selector` (аннотация) | `<selector>` | дефолтное размещение, из `Project.spec.nodeSelector` |
| метка, которую выбирает `operator-trivy` (напр. `security.deckhouse.io/vulnerability-scan`) | `"true"` | ставится при `features.vulnerabilityScanning` — включает namespace в CVE-сканирование |
| scrape-метка, которую выбирает стек мониторинга (напр. `monitoring.deckhouse.io/enabled`) | `"true"` | ставится при `features.monitoring` — включает namespace в scrape |

**Метки-ссылки по имени** (на рендеримых объектах, значение = имя источника — для обновления и GC):

| метка | на чём | указывает на |
|-------|--------|--------------|
| `projects.deckhouse.io/namespace-claim` | созданный рабочий `Namespace` | `ProjectNamespace`, который его заказал (удалить claim ⇒ удалить namespace) |
| `projects.deckhouse.io/project-role-binding` | каждый рендеримый `RoleBinding` | `ProjectRoleBinding`, из которого он создан |

**Ссылочные, author-defined (не наши)** — метки на granted-объектах, которые матчат селекторы гранта:
например `shared: "true"` (`allowedSelector`), `rbac.deckhouse.io/tenant-bindable`,
`storageclass.deckhouse.io/system` (`excluded`). Живут на кластерных ресурсах, ставятся их авторами, не
этим модулем.

Проброс — это **allowlist** ключей меток `Project` (конфигурируемый), не «все метки», чтобы метка не
могла случайно совпасть с привилегированным селектором политики.

## Квоты

У проекта есть **общий бюджет**, распределяемый по его namespace, две части по механизму:

- **compute — нативная, в `Project.spec.quota.compute`.** `requests.cpu`/`memory`, `limits.*`, `pods`,
  сырой `count/<resource>`. Контроллер рендерит нативный `ResourceQuota` в каждый namespace. Это и есть
  естественная модель `ResourceQuota`, поэтому остаётся нативной.
- **objects — наша, в `ClusterResourceGrant`.** Per-class лимиты по ключу **grantable-ресурс → имя granted (или
  `*`) → мера** (storage по StorageClass, count по LoadBalancerClass/IngressClass, …). Пул — `ClusterResourceGrant`
  в control namespace; контроллер рендерит read-only `ClusterResourceGrant` в каждый рабочий namespace с расходом.
  Полная модель — в [дизайне грантов](./CLUSTER_OBJECT_GRANTS_DESIGN_RU.md#clusterresourcegrant).

```yaml
# compute-пул — на Project
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: team-a
spec:
  quota:
    compute:
      requests.cpu: "40"
      requests.memory: 80Gi
      limits.cpu: "60"
      limits.memory: 120Gi
      pods: "400"
---
# пул объектов — ClusterResourceGrant в control namespace (пишет только cluster-admin)
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrant
metadata:
  name: objects
  namespace: team-a
spec:
  objects:
    storageclasses:
      "*":                       # все storage-классы вместе
        requests.storage: 1Ti
      fast:                      # класс "fast", строже
        requests.storage: 200Gi
    loadbalancerclasses:
      external:
        services: 5              # кейс «5 внешних LB» — нативного per-class ключа нет
      internal:
        services: -1             # без ограничения
```

**Почему лимиты объектов наши, даже когда у Kubernetes есть нативный ключ.** Нативный `ResourceQuota`
*умеет* ограничивать storage per class (`fast.storageclass.storage.k8s.io/requests.storage`), но **не
умеет** ограничивать per `LoadBalancerClass` или per `IngressClass` (знает только *total*
`services.loadbalancers`, `count/ingresses`). Поэтому все per-class лимиты объектов — наши и
единообразные, в `ClusterResourceGrant`, независимо от того, есть ли нативный ключ.

| Лимит | Где |
|-------|-----|
| cpu/memory (requests/limits), pods, сырой `count/<resource>` | `Project.spec.quota.compute` (нативно) |
| storage по StorageClass (`*` и по классу) | `ClusterResourceGrant.spec.objects.storageclasses` |
| count по LoadBalancerClass (напр. 5 external) | `ClusterResourceGrant.spec.objects.loadbalancerclasses` |
| count по IngressClass / по кастомному granted-объекту | `ClusterResourceGrant.spec.objects.<resource>` |
| **число namespace** | `Project.spec.namespaces.max` (проверяется при создании namespace) |

**Пул, слайс, RBAC.** Оба бюджета — пулы проекта, общие на его namespace. **Пул** задаёт cluster-admin
(`Project.spec.quota.compute` и `ClusterResourceGrant.spec` в control namespace — оба пишет только cluster-admin,
поэтому тенант не повысит итог). Админ проекта опционально нарезает **per-namespace слайсы** через
`ProjectNamespace.spec.quota` (`Σ слайсов ≤ пул`). Контроллер рендерит нативный `ResourceQuota` и
read-only per-namespace `ClusterResourceGrant` в каждый рабочий namespace (видно тенанту). Полная таблица RBAC —
в [дизайне грантов](./CLUSTER_OBJECT_GRANTS_DESIGN_RU.md#квоты).

### Учёт расхода — детализация по namespace, свод в проект

Расход считается на уровне namespace и сводится, чтобы весь бюджет был виден:

- **по namespace (детализация)** — `compute` это нативный `ResourceQuota.status.used`; `objects` это
  отрендеренный `ClusterResourceGrant.status` этого namespace;
- **свод compute** — `Project.status.quota` несёт `total` (общее потребление) + массив `namespaces[]`,
  compute `used` суммой по `ResourceQuota.status.used` каждого namespace;
- **свод объектов** — `ClusterResourceGrant.status` в control namespace несёт итог по проекту (`projectUsed`
  против лимита) по каждой мере объектов.

```yaml
# свод compute на Project
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: team-a
status:
  quota:
    total:                       # общее потребление проекта (compute)
      compute:
      - name: requests.cpu
        limit: "40"
        used: "26"               # Σ ResourceQuota.status.used по namespace
      - name: requests.memory
        limit: 80Gi
        used: 51Gi
    namespaces:                  # разбивка по namespace (compute)
    - namespace: team-a-backend
      compute:
      - name: requests.cpu
        limit: "24"
        used: "16"
    - namespace: team-a-frontend
      compute:
      - name: requests.cpu
        limit: "16"
        used: "10"
```

Расход объектов живёт в `ClusterResourceGrant` (итог по проекту — в объекте control namespace; по namespace — в
отрендеренном `ClusterResourceGrant` каждого рабочего namespace) — см.
[дизайн грантов](./CLUSTER_OBJECT_GRANTS_DESIGN_RU.md#clusterresourcegrant).

Compute-`ResourceQuota` заставляет Kubernetes требовать `requests`/`limits` на подах; дефолты даёт
`LimitRange`, который `ProjectTemplate` и так рендерит — это штатный механизм куба, ничего нового.

## Гранты на кластерные ресурсы навешиваются по лейблу

Квота живёт на `Project` (compute) и `ClusterResourceGrant` (objects); **доступность** — *какие* кластерные
объекты можно проекту и per-project дефолт — задаётся отдельно как `ClusterResourceGrantPolicy` и
**навешивается на проект по лейблу**.
`projectSelector` гранта матчит **метки Project**; контроллер разворачивает совпавшие Project в их
namespace и материализует там доступность. Это та же модель «author once, match by label», что и у
`SecurityPolicy` (см. [Security-политики](#security-политики--преднастраивает-админ-матчит-по-лейблу-как-гранты));
полная модель грантов — в [дизайне грантов](./CLUSTER_OBJECT_GRANTS_DESIGN_RU.md).

```yaml
# Переиспользуемый пресет: навешивается на каждый Project с меткой environment=production.
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production
spec:
  projectSelector:
    matchLabels:
      environment: production    # матчит Project.metadata.labels
  resources:
  - resourceName: storageclasses  # только allow-list + default (квота — на Project)
    allowed:
    - standard
    default: standard
```

То есть чтобы дать проекту набор доступных ресурсов, вы **навешиваете метку на Project** (например
`environment: production`), и совпавшие гранты применяются — отдельный грант на проект писать не надо.
Квота **опциональна и ортогональна**: ресурс можно сделать доступным вообще без квоты (у многих мерить
нечего), а запись в `ClusterResourceGrant` без гранта, делающего ресурс доступным, ничего не даёт.

## Сквозной пример

Продакшн multi-namespace проект: сложная compute/storage квота, owners, два рабочих namespace со
слайсами квоты, self-service биндинг разработчиков и гранты на кластерные ресурсы (storage + LB) —
сетевая изоляция и PSS приходят из шаблона/политик выше.

```yaml
# 1. Проект: режим, префикс, owners и общий нативный лимит проекта.
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: team-a
  labels:
    environment: production
spec:
  projectTemplateName: default
  namespaces:
    selfService: true
    prefix: team-a
    max: 10
  owners:
  - kind: Group
    name: team-a-admins
    accessLevel: Admin
  podSecurityStandard: Restricted   # first-class PSS (>= cluster floor)
  nodeSelector:
    node-pool: tenants
  features:
    vulnerabilityScanning: true     # operator-trivy на namespace проекта
    monitoring: true                # мониторинг тенанта (scrape + мониторы + Grafana)
  quota:
    compute:                        # нативные ключи ResourceQuota (объекты — в ClusterResourceGrant ниже)
      requests.cpu: "40"
      limits.cpu: "60"
      requests.memory: 80Gi
      limits.memory: 120Gi
      pods: "400"
      count/jobs.batch: "50"
---
# 2a. Пул квоты объектов — ClusterResourceGrant в control namespace (пишет только cluster-admin).
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrant
metadata:
  name: objects
  namespace: team-a
spec:
  objects:
    storageclasses:
      "*":
        requests.storage: 1Ti
      fast:
        requests.storage: 200Gi
    loadbalancerclasses:
      external:
        services: 5
      internal:
        services: -1
---
# 2b. Cluster-admin выдаёт, какие кластерные ресурсы можно проекту (storage / LB-классы и дефолты).
#     Грант делает только allow-list + default; per-class лимиты живут в ClusterResourceGrant выше.
#     (Домен дизайна грантов; матчится projectSelector'ом по меткам Project.)
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
  - resourceName: storageclasses     # allow-list + default
    allowed:
    - standard
    allowedSelector:
      matchLabels:
        tier: fast
    default: standard
  - resourceName: loadbalancerclasses   # allow-list + default
    allowed:
    - external
    - internal
    default: internal
---
# 3. Админ проекта заказывает два рабочих namespace, каждый со слайсом квоты (Σ ≤ лимита проекта).
apiVersion: deckhouse.io/v1alpha2
kind: ProjectNamespace
metadata:
  name: backend
  namespace: team-a
spec:
  quota:
    compute:
      requests.cpu: "24"
      requests.memory: 48Gi
    objects:
      storageclasses:
        fast:
          requests.storage: 150Gi
      loadbalancerclasses:
        external:
          services: 3
---
apiVersion: deckhouse.io/v1alpha2
kind: ProjectNamespace
metadata:
  name: frontend
  namespace: team-a
spec:
  quota:
    compute:
      requests.cpu: "16"
      requests.memory: 32Gi
    objects:
      storageclasses:
        fast:
          requests.storage: 50Gi
      loadbalancerclasses:
        external:
          services: 2
---
# 4. Админ проекта выдаёт команде разработчиков Editor на весь проект (все namespace).
apiVersion: deckhouse.io/v1alpha2
kind: ProjectRoleBinding
metadata:
  name: developers
  namespace: team-a
spec:
  subjects:
  - kind: Group
    name: team-a-developers
  accessLevel: Editor
```

Итог: `team-a` (control), `team-a-backend`, `team-a-frontend`; compute/storage ограничены по каждому
namespace и суммируются в пределах лимита проекта; использование LB и storage-классов ограничено и
задефолчено грантом; разработчики — Editor везде в проекте; namespace общаются между собой, но
изолированы от других проектов; PSS применён. Тенант видит `kubectl get available storageclasses -n
team-a-backend`.

## Миграция (существующие проекты)

Существующие проекты — single-namespace (проект == namespace) и должны продолжать работать без правок.

1. **Аддитивный API с дефолтами.** Каждое новое поле `Project.spec` опционально, дефолт повторяет
   текущее поведение: `namespaces.selfService` по умолчанию `false` ⇒ существующие проекты остаются
   single-namespace (ни префикса, ни control namespace, ни `ProjectNamespace`); пустые `owners`/`quota`
   ⇒ RBAC и `ResourceQuota` ровно как их рендерит `ProjectTemplate` сейчас. Существующим проектам
   нужно **ноль изменений**. Если режется новая версия API, conversion-вебхук проектов (уже есть)
   выставляет `selfService: false` сконвертированным объектам.

2. **Single → multi-namespace — явный управляемый opt-in, не автопереключение.** В namespace живого
   проекта уже есть ворклоады, поэтому он не может молча стать закрытым control namespace. При
   миграции:
   - существующий namespace **адоптируется** как первый/дефолтный рабочий namespace проекта
     (его представляет авто-созданный `ProjectNamespace`); `prefix` по умолчанию = имя проекта;
   - для **мигрированных** проектов namespace с именем проекта **остаётся рабочим** (не закрывается) —
     строгий управляющий control namespace применяется к проектам, **созданным** в режиме
     multi-namespace (они стартуют пустыми). Это задокументированный компромисс совместимости; позже
     отдельным шагом «strict» можно перенести мигрированный проект на отдельный закрытый control
     namespace, когда ворклоады переедут;
   - последующие namespace — `<prefix>-<suffix>` как обычно.

3. **Передача квоты / RBAC.** Пока `Project.spec.quota` / `owners` не заданы, авторитетны
   `ResourceQuota` / RBAC, отрендеренные `ProjectTemplate`. Когда заданы — контроллер берёт на себя
   per-namespace `ResourceQuota` и **добавляет** owner-биндинги (не удаляя template-RBAC). Приоритет
   задокументирован, чтобы переключение было предсказуемым.

4. **Валидация префикса против существующего кластера.** Проверка коллизий считает существующие
   namespace входным множеством. Проект, мигрирующий в multi-namespace, должен иметь `prefix` (= его
   имя), проходящий проверку reserved/коллизий; проект, чьё имя пересекается с зарезервированным
   префиксом или другим namespace, не может уйти в multi-namespace до разрешения (редкий edge case).

5. **Режим задаётся при создании; миграция выше — единственный санкционированный способ его сменить**
   (без свободного тогглинга), так что переход всегда явная отрецензированная операция.

## Решения

- **Два режима, выбор при создании**: single-namespace (по умолчанию, без изменений) vs
  multi-namespace (self-service). Self-service — opt-in, не навязывается проектам, которым не нужен.
- **`ProjectNamespace`** (namespaced, в control namespace) — как админ проекта заказывает namespace в
  multi-namespace; удаление убирает namespace.
- **`ProjectNamespace` и `ProjectRoleBinding` валидны только в основном (control) namespace проекта**
  — в любом другом namespace admission их отклоняет.
- **Обязательный префикс** в multi-namespace (по умолчанию = имя проекта); **валидация коллизий**
  против зарезервированных, префиксов других проектов и существующих namespace по отношению «префикс».
- **Control namespace** = имя проекта, закрыт admission'ом до управляющих kinds; без ворклоадов.
- **Спека Project получает first-class** `owners`, `namespaces` (`selfService`/`prefix`/`max`) и
  `quota`; пер-namespace рендеримые ресурсы остаются в `ProjectTemplate`.
- **ProjectTemplate рендерит пер-namespace** в multi-namespace; проектное переезжает в `Project.spec`.
- **Квоты, по механизму**: compute → `Project.spec.quota.compute` (нативный `ResourceQuota`, по
  namespace); objects → `ClusterResourceGrant` (`spec`-пул в control namespace + рендер read-only по namespace).
  Оба пула задаёт cluster-admin; админ проекта нарезает per-NS слайсы через `ProjectNamespace.spec.quota`
  (`Σ ≤ пул`). Грант делает только allow-list + default. `max` namespace проверяется при создании.
- **Доступность через гранты навешивается по лейблу**: `ClusterResourceGrantPolicy.projectSelector` матчит
  метки Project (переиспользуемый пресет, как `SecurityPolicy`). Квота опциональна и ортогональна
  доступности.
- **Свод расхода**: compute — в `Project.status.quota` (`total` + `namespaces[]`, суммой по per-NS
  `ResourceQuota.status.used`); objects — в `ClusterResourceGrant.status` (итог по проекту в control namespace,
  по namespace — в каждом отрендеренном `ClusterResourceGrant`).
- **Compute-`ResourceQuota` всегда рендерится в паре с `LimitRange`** (дефолты + min/max из шаблона),
  т.к. Kubernetes отклоняет поды без requests/limits под compute-квотой.
- **Контроллер пробрасывает метки `Project` на его namespace**, чтобы сетевая изоляция и админский
  `SecurityPolicy` могли таргетить класс проектов по лейблу.
- **Сетевую изоляцию рендерит контроллер** (из шаблона, на каждый namespace, при создании) с
  селектором на уровне проекта; **`SecurityPolicy` преднастраивает админ и матчит по лейблу**, как
  гранты.
- **Фичи по умолчанию = floor + класс + per-project**, всё через выбор по меткам. Floor (политика по
  всегда-присутствующей метке принадлежности) нельзя ослабить шаблоном или проектом; эффективный PSS
  `enforce` = max(floor, запрос); control namespace исключается через `namespace-role`.
- **First-class поля фич в `Project.spec`**: `podSecurityStandard` и `nodeSelector`/`tolerations`
  (энфорсит этот контроллер); `features.vulnerabilityScanning` / `features.monitoring` (делегировано —
  контроллер только проставляет метки + RBAC для `operator-trivy` / стека мониторинга; no-op, если
  модуль выключен). Сами сканирование и мониторинг мы не реализуем.
- **Дефолтное размещение подов** через `Project.spec.nodeSelector`/`tolerations` → аннотация
  `scheduler.alpha.kubernetes.io/node-selector` на каждом namespace проекта.
- **Служебные метки** (см. [Служебные метки и аннотации](#служебные-метки-и-аннотации)): каждый
  управляемый объект несёт `projects.deckhouse.io/project` (принадлежность), `heritage: deckhouse`,
  `module: multitenancy-manager`; namespace также несут `namespace-role` и PSS-метки; рендеримые объекты
  несут метки-ссылки по имени для обновления/GC. Проброс меток `Project` — это allowlist.

## Открытые вопросы

- `ProjectNamespace`: namespaced в control namespace (склоняюсь) vs cluster-scoped.
- Префикс: форсить `prefix == имя проекта` или разрешить произвольный с проверкой коллизий?
- `ProjectTemplate`: рендерить весь шаблон пер-namespace или добавить маркеры пер-namespace/пер-проект?
- Мигрированные проекты держат namespace-с-именем-проекта как рабочий (не закрытый) — предлагать ли
  позже шаг «strict» (перенести ворклоады и закрыть), или оставлять мигрированные с послабленным
  control namespace навсегда? (см. [Миграцию](#миграция-существующие-проекты))
- Точный белый список kinds для control namespace.
- Как `Project.spec.quota` уживается с `ResourceQuota`, которую `ProjectTemplate` рендерит сейчас (кто
  владеет пер-namespace `ResourceQuota` в каждом режиме).
