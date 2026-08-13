# Миграция модуля на ролевую модель RBACv2 из DKP 1.78

**Кому:** командам, которые поддерживают DKP-модуль за пределами репозитория платформы и держат в нём
шаблоны `templates/rbacv2/**`. Модули из репозитория `deckhouse` уже мигрированы. Этот документ,
скрипт рядом с ним и [промпт для агента](./RBACV2_MODULE_MIGRATION_PROMPT.md) нужны только тем, чьи
модули живут в отдельных репозиториях. Вся работа идёт в репозитории модуля — на уже работающем
кластере миграция ничего не меняет, пока вы не выпустите и не установите новую версию модуля.

Здесь описана только техническая сторона миграции — объекты, которые поставляет сам модуль.
Пользовательская сторона модели RBACv2 (имена встроенных ролей, как заводить кастомные роли и
привязки) — в документации модуля user-authz: [README](../README_RU.md) и [FAQ](../FAQ_RU.md). Термины
ниже («область», «уровень», «capability») взяты оттуда же: если что-то в этом документе непонятно,
скорее всего, объяснение найдётся там.

## Коротко

Требуется миграция шаблонов ролей написанных под rbac v2. 
Для автоматизации написан скрипт, который нужно выполнить в корне с проектом модуля.

```shell
# В репозитории модуля:
curl -sO https://raw.githubusercontent.com/deckhouse/deckhouse/main/modules/140-user-authz/docs/internal/rbacv2-migrate-module.sh
chmod +x rbacv2-migrate-module.sh
./rbacv2-migrate-module.sh -n     # посмотреть диф
./rbacv2-migrate-module.sh        # применить
```

Скрипт верно переименовывает объекты и переписывает их лейблы — это механическая часть миграции. Но он
не понимает смысла правил внутри `rules`, поэтому после него нужно [проверить руками](#что-стоит-проверить-руками)
несколько вещей, которые он мог перенести формально правильно, но по смыслу — нет.  

## Что было и что стало

До RBACv2 модуль выдавал права двумя видами объектов, которые различались лейблом
`rbac.deckhouse.io/kind`:

- `use` — права на ресурсы внутри пространства имён, то есть то с чем взаимодействует пользователь конкретного проекта (например,
  разработчик, который работает только в своём пространстве имён);
- `manage` — права на настройку самого модуля: его `ModuleConfig` и его кластерные (cluster-wide)
  ресурсы.

Пример ресурса:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    heritage: deckhouse
    module: operator-trivy
    rbac.deckhouse.io/aggregate-to-security-as: manager
    rbac.deckhouse.io/kind: manage                        # речь об этом лейбле
    rbac.deckhouse.io/level: module
    rbac.deckhouse.io/namespace: d8-operator-trivy
  name: d8:manage:permission:module:operator-trivy:edit
rules:
- apiGroups:
  - trivy.deckhouse.io
  resources:
  - clustercompliancereports
  verbs:
  - create
  - update
  - patch
  - delete
  - deletecollection
```

RBACv2 сохраняет то же деление, но убирает два разных вида объектов в пользу одного вида с явным
атрибутом:

- оба вида объектов теперь называются одинаково — **capability** (лейбл `rbac.deckhouse.io/kind:  capability` вместо `use`/`manage`);
- вместо вида объекта у каждой capability есть **область** (лейбл `rbac.deckhouse.io/scope`)
  -  `namespace` — это то же самое, что раньше было `use`; 
  -  `system` или имя подсистемы модуля (`kubernetes`, `networking` и так далее) — то же самое, что раньше было `manage`.

Capability — это всё тот же обычный `ClusterRole` (стандартный ресурс Kubernetes) с правилами, только с другими лейблами и именем. 
Итоговые **роли** (`d8:namespace:viewer`, `d8:system:manager` и подобные) модуль не должен генерировать: их создают `user-authz` и `multitenancy-manager`. Роль — это пустой `ClusterRole`, в котором
`aggregationRule` выбирает capabilities модуля по лейблу `rbac.deckhouse.io/aggregate-to-<область>-as:
<уровень>`; правила в роль записывает сам Kubernetes.

{% alert level="info" %}
Если в вашем модуле есть генерация ролей, то проверьте не осталось ли в них с поля `rules`. Если есть — уберите это поле
полностью, даже как `rules: []`. Полем `rules` роли безраздельно владеет
`clusterrole-aggregation-controller`: если чарт объявит его тоже, server-side apply начнёт
конфликтовать с контроллером (`conflict with "clusterrole-aggregation-controller": .rules`). При
принудительном разрешении этого конфликта объект будет перезаписываться дважды за каждый реконсайл, и
в промежутке между этими двумя записями роль ненадолго перестанет давать какие-либо права.
{% endalert %}

## Области и уровни доступа

У каждой capability есть:
 - область — в какую встроенную роль она агрегируется 
 - уровень внутри области — сколько прав она даёт, от простого просмотра до полного управления. 
Уровни идут по нарастанию: `viewer` → `user` → `manager` → `admin` → `superadmin`, но не каждая область содержит все
уровни.

| Область                                                                                                           | Уровни                                             | Выдаётся через                    | Что кладёт туда модуль                                              |
| ----------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- | --------------------------------- | ------------------------------------------------------------------- |
| `namespace`                                                                                                       | `viewer`, `user`, `manager`, `admin`, `superadmin` | `RoleBinding` в пространстве имён | Ресурсы модуля внутри пространства имён — то, с чем работает пользователь проекта |
| `<подсистема>`: `deckhouse`, `infrastructure`, `kubernetes`, `networking`, `observability`, `security`, `storage` | `viewer`, `manager`, `superadmin`                  | `ClusterRoleBinding`              | Кластерные ресурсы модуля и его `ModuleConfig`                      |
| `system`                                                                                                          | `viewer`, `manager`, `superadmin`                  | `ClusterRoleBinding`              | Напрямую ничего: системные роли агрегируют подсистемные             |
| `project`                                                                                                         | `viewer`, `user`, `manager`, `admin`, `superadmin` | `ProjectRoleBinding`              | Ресурсы структуры проекта (только multitenancy-manager)             |

Обычному модулю из всей таблицы нужны только две строки: `namespace` (бывший `use`) и своя подсистема
(бывший `manage`). Строки `system` и `project` модуль не заполняет сам — `system` агрегирует
подсистемные capabilities выше по цепочке, а `project` целиком относится к multitenancy-manager.

Миграция переносит namespace-уровень из подсистемы `kubernetes`, где он раньше жил вместе с `manage`,
в отдельную область `namespace`, которой он и принадлежит по смыслу.

## Переименование объектов и лейблов

Имя объекта и его лейблы меняются по фиксированным правилам — это и делает скрипт автоматически.

| Прежнее имя                                         | Новое имя                                       |
| --------------------------------------------------- | ----------------------------------------------- |
| `d8:use:capability:module:<модуль>:<view\|edit>`    | `d8:namespace-capability:<модуль>:<view\|edit>` |
| `d8:manage:permission:module:<модуль>:<view\|edit>` | `d8:system-capability:<модуль>:<view\|edit>`    |

| Прежний лейбл                                                                                                           | Новый лейбл                                                                                                                                                            |
| ----------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `rbac.deckhouse.io/kind: use` или `manage`                                                                              | `rbac.deckhouse.io/kind: capability`                                                                                                                                   |
| —                                                                                                                       | `rbac.deckhouse.io/scope: namespace` (было `use`) или `system` (было `manage`)                                                                                         |
| —                                                                                                                       | `rbac.deckhouse.io/capability: "<область>-capability.<модуль>.<действие>"` — глобально уникален; именно по нему кастомная роль и консоль выбирают ровно эту capability |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` на объекте `use` (и второй такой же лейбл рядом, если он был) | один лейбл `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>`, уровень тот же                                                                                    |
| `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` на объекте `manage`                                         | остаётся как есть, в том числе несколько сразу                                                                                                                         |
| `rbac.deckhouse.io/level: module`                                                                                       | удаляется, его никто не читает                                                                                                                                         |
| `rbac.deckhouse.io/namespace: d8-<модуль>`                                                                              | остаётся — но читается только на объектах с областью `system` или `subsystem`                                                                                          |

Помимо изменений лейблов, добавлены изменения связанные с локализацией. Они нужны для корректной отрисовки прав в веб-интерфейсе модуля console.
Локализованные заголовки и описания теперь обязательны на каждом объекте, размещены через аннотации а их формулировки едины по всей платформе (скрипт генерирует ровно такие):

```yaml
  annotations:
    en.meta.deckhouse.io/title: "Module <модуль>: view"
    ru.meta.deckhouse.io/title: "Модуль <модуль>: просмотр"
    en.meta.deckhouse.io/description: "Read-only access to <модуль> resources in a namespace."
    ru.meta.deckhouse.io/description: "Доступ только на чтение к ресурсам модуля <модуль> в пространстве имён."
```


### Что стоит проверить руками

Скрипт правильно переписывает имена и лейблы, но не читает сами правила `rules` — поэтому четыре вещи
он либо не может решить, либо решает по умолчанию, и их стоит перепроверить самостоятельно.

1. **На своём ли уровне лежит право.** 
   Раньше можно было указать кластерный ресурс в namespace-роли и это работало. Теперь перестало.
   Проверьте каждое правило в мигрированных файлах `use/`: если ресурс кластерный (а не namespaced), перенесите
   это правило в `manage/`. 
2. **Уровень.** 
   Нужно рассмотреть использование модуля в режиме мультитенантности (когда несколько проектов и у каждого свои пользователи).
   Скрипт сохраняет то соответствие, которое использовало большинство модулей: `view` →
   `viewer`, `edit` → `manager`. Но в области `namespace` есть и другие уровни: `user` — то, что просто
   работает в пространстве имён и нужно обычному пользователю проекта, и `admin` — тому, кто этим проектом
   управляет. Если ваша `edit`-capability на самом деле рассчитана на обычного пользователя проекта, а не на
   администратора, поставьте ей уровень `user`, а не `manager`.
3. **Два лейбла агрегации сразу.** 
   У прежних `use`-capability иногда стояли одновременно и `aggregate-to-kubernetes-as`, и `aggregate-to-networking-as`
    — capability попадала в обе подсистемы.
   Скрипт оставляет только один из них (`aggregate-to-namespace-as`), потому что namespace-правам в
   подсистемной области делать нечего. Если модулю на самом деле нужна ещё и отдельная кластерная часть
   в `networking`, её нужно вынести в отдельную system-capability, а не пытаться сохранить второй
   лейбл на namespace-capability.
4. **Формулировки.** Заголовки и описания, которые сгенерировал скрипт, следуют общей для платформы
   конвенции и называют модуль по имени. Прочитайте их один раз — именно их консоль показывает рядом с
   capability. Если написано некорректно - поправьте. 

## Полный пример

Было — права модуля внутри пространства имён:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer
    rbac.deckhouse.io/kind: use
  name: d8:use:capability:module:mymodule:view
rules:
  - apiGroups: ["mygroup.io"]
    resources: ["myresources"]
    verbs: ["get", "list", "watch"]
```

Стало:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-namespace-as: viewer
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/capability: "namespace-capability.mymodule.view"
    rbac.deckhouse.io/scope: namespace
  name: d8:namespace-capability:mymodule:view
  annotations:
    en.meta.deckhouse.io/title: "Module mymodule: view"
    ru.meta.deckhouse.io/title: "Модуль mymodule: просмотр"
    en.meta.deckhouse.io/description: "Read-only access to mymodule resources in a namespace."
    ru.meta.deckhouse.io/description: "Доступ только на чтение к ресурсам модуля mymodule в пространстве имён."
rules:
  - apiGroups: ["mygroup.io"]
    resources: ["myresources"]
    verbs: ["get", "list", "watch"]
```

А это конфигурация модуля — она остаётся в той же подсистемной области, в которой была раньше, меняются
только имя и лейблы:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-networking-as: manager
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/capability: "system-capability.mymodule.edit"
    rbac.deckhouse.io/scope: system
    rbac.deckhouse.io/namespace: d8-mymodule
  name: d8:system-capability:mymodule:edit
  annotations:
    en.meta.deckhouse.io/title: "Module mymodule: edit configuration"
    ru.meta.deckhouse.io/title: "Модуль mymodule: управление конфигурацией"
    en.meta.deckhouse.io/description: "Manage the mymodule module configuration."
    ru.meta.deckhouse.io/description: "Управление конфигурацией модуля mymodule."
rules:
  - apiGroups: ["deckhouse.io"]
    resources: ["moduleconfigs"]
    resourceNames: ["mymodule"]
    verbs: ["create", "update", "patch", "delete"]
```

Если лейблы модуля шаблонизированы через `helm_lib_module_labels`, то же самое меняется внутри вызова
`dict` — те же пары лейблов, только другие значения.

## Скрипт

`rbacv2-migrate-module.sh` обходит `**/templates/rbacv2/**`, переписывает каждую найденную capability
прежней схемы и отдельно перечисляет то, что не решился тронуть. Чтобы вы понимали, насколько ему можно
доверять: на всех модулях платформы он воспроизвёл вручную сделанную миграцию побайтово в 93 файлах из
95. В оставшихся двух отличие — не ошибка скрипта, а решение о том, куда отнести правило, которое
скрипт принять не может (см. следующий раздел).

```shell
./rbacv2-migrate-module.sh -n PATH   # только диф
./rbacv2-migrate-module.sh PATH      # переписать на месте
```

Скрипт идемпотентен: уже мигрированный файл он отметит как нетронутый. Файлы он не переименовывает,
поэтому диф остаётся читаемым.


## Как проверить результат

В репозитории:

```shell
# Шаблоны по-прежнему рендерятся.
helm template . --set global.enabledModules='{}' >/dev/null

# Ничего от прежней схемы не осталось.
grep -rn 'rbac.deckhouse.io/kind: \(use\|manage\)\|d8:use:capability\|d8:manage:permission\|rbac.deckhouse.io/level' templates/
```

Контракт, который платформа проверяет на своих модулях, лежит в
[`testing/rbacv2`](../../../../testing/rbacv2) репозитория `deckhouse` (`go test -tags validation ./testing/rbacv2/`):
префикс имени `d8:` и шаблон имени для каждой области, наличие `kind` и `scope`, четыре i18n-аннотации,
роль с `aggregationRule` и без собственных правил (без ключа `rules` вообще) против capability
с правилами и без `aggregationRule`,
уникальный маркер `rbac.deckhouse.io/capability` и хотя бы один лейбл агрегации на каждой capability,
а также `rbac.deckhouse.io/delegatable` только на namespace- и проектных ролях.

В кластере с установленным модулем:

```shell
# Namespace-права доехали до namespace-ролей.
d8 k get clusterrole d8:namespace:viewer -o json | jq '[.rules[] | select(.apiGroups[] == "mygroup.io")]'

# Права на конфигурацию доехали до подсистемной роли.
d8 k get clusterrole d8:subsystem:networking:manager -o json | jq '[.rules[] | select(.resourceNames[]? == "mymodule")]'
```

## Что будет, если модуль не мигрировать

Старые объекты никто не отвергнет и ничего не залогирует — они останутся валидными Kubernetes-объектами.
Изменится не их валидность, а то, что читает по их лейблам остальная платформа. Три следствия, и все
три проверены на живом кластере.

- **`use`-capability выдаётся на весь кластер вместо пространства имён.** Лейбл
  `aggregate-to-kubernetes-as` теперь принадлежит не namespace-уровню, а кластерной подсистеме
  `kubernetes`. Поэтому namespace-правила модуля попадают в `d8:subsystem:kubernetes:*` и, вверх по
  цепочке агрегации, в `d8:system:*` — их получает каждый обладатель этих ролей во всём кластере. А в
  `d8:namespace:*` этих правил больше нет вовсе, так что тенанты теряют доступ, который у них раньше
  был.
- **`use`-capability уровня `user` или `admin` исчезает совсем.** В подсистемных областях таких уровней
  нет, поэтому такую capability никто не выбирает, и её правила не достаются никому.
- **Автоматический `RoleBinding` в пространстве имён модуля перестаёт создаваться.** Есть хук, который
  разворачивает `ClusterRoleBinding` на системную или подсистемную роль в отдельные namespace-привязки.
  Он смотрит только на объекты с лейблом `rbac.deckhouse.io/scope: system` или `subsystem`. Без него
  лейбл `rbac.deckhouse.io/namespace` для хука просто не существует, и привязка не создаётся.

`manage`-capability в этом смысле безопаснее: она продолжает агрегироваться в ту же подсистемную роль,
то есть права никуда не переезжают. Но она остаётся невидимой для консоли и для permission browser —
они отличают роли от capabilities и capabilities друг от друга именно по лейблам
`rbac.deckhouse.io/kind` и `rbac.deckhouse.io/scope`, которых на немигрированном объекте нет.
