# Миграция модуля на ролевую модель RBACv2 из DKP 1.78

Кому: командам, которые сопровождают модуль DKP с шаблонами `templates/rbacv2/**`. Вся работа идёт в
репозитории модуля — кластер здесь ни при чём. Все модули из репозитория платформы уже мигрированы;
этот документ, лежащий рядом скрипт и [промпт для агента](./RBACV2_MODULE_MIGRATION_PROMPT.md) — для
модулей, которые живут отдельно.

Пользовательская сторона изменения — имена ролей, кастомные роли, привязки — описана в документации
модуля ([README](../README_RU.md), [FAQ](../FAQ_RU.md)). Здесь речь только про объекты, которые модуль
поставляет сам.

## Коротко

```shell
# В репозитории модуля:
curl -sO https://raw.githubusercontent.com/deckhouse/deckhouse/main/modules/140-user-authz/docs/internal/rbacv2-migrate-module.sh
chmod +x rbacv2-migrate-module.sh
./rbacv2-migrate-module.sh -n     # посмотреть диф
./rbacv2-migrate-module.sh        # применить
```

Дальше — [что проверить руками](#чего-скрипт-решить-не-может): метки и имена скрипт переписывает
верно, но он не знает, на своём ли месте лежит то или иное право.

## Что изменилось в модели

Права живут в **capabilities** — объектах `ClusterRole` с правилами и одной или несколькими метками
`rbac.deckhouse.io/aggregate-to-<линейка>-as: <уровень>`. **Роли** — пустые `ClusterRole`, чей
`aggregationRule` выбирает эти метки; наполняет их сам Kubernetes. Модуль поставляет только
capabilities, роли принадлежат `user-authz` и `multitenancy-manager`.

| Линейка | Уровни | Выдаётся через | Что кладёт туда модуль |
|---------|--------|----------------|------------------------|
| `namespace` | `viewer`, `user`, `manager`, `admin`, `superadmin` | `RoleBinding` в пространстве имён | Ресурсы модуля внутри пространства имён — то, с чем работает тенант |
| `<подсистема>`: `deckhouse`, `infrastructure`, `kubernetes`, `networking`, `observability`, `security`, `storage` | `viewer`, `manager`, `superadmin` | `ClusterRoleBinding` | Кластерные ресурсы модуля и его `ModuleConfig` |
| `system` | `viewer`, `manager`, `superadmin` | `ClusterRoleBinding` | Напрямую ничего: системные роли агрегируют подсистемные |
| `project` | `viewer`, `user`, `manager`, `admin`, `superadmin` | `ProjectRoleBinding` | Ресурсы структуры проекта (только multitenancy-manager) |

В прежней схеме были те же два уровня под другими именами: `use` — то, что происходит внутри
пространства имён, `manage` — собственная конфигурация модуля. Миграция это деление сохраняет:
переименовывает объекты, меняет метки, по которым идёт агрегация, и переносит namespace-уровень с
подсистемы `kubernetes` на линейку `namespace`, которой он и принадлежит.

## Соответствие

| Прежде | Теперь |
|--------|--------|
| `d8:use:capability:module:<модуль>:<view\|edit>` | `d8:namespace-capability:<модуль>:<view\|edit>` |
| `d8:manage:permission:module:<модуль>:<view\|edit>` | `d8:system-capability:<модуль>:<view\|edit>` |

| Прежняя метка | Новая метка |
|---------------|-------------|
| `rbac.deckhouse.io/kind: use` или `manage` | `rbac.deckhouse.io/kind: capability` |
| — | `rbac.deckhouse.io/scope: namespace` (было `use`) или `system` (было `manage`) |
| — | `rbac.deckhouse.io/capability: "<область>-capability.<модуль>.<действие>"` — глобально уникальна; именно она позволяет кастомной роли и консоли выбрать ровно эту capability |
| `rbac.deckhouse.io/aggregate-to-kubernetes-as: <уровень>` на объекте `use` (и вторая линейка рядом, если была) | одна метка `rbac.deckhouse.io/aggregate-to-namespace-as: <уровень>`, уровень тот же |
| `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` на объекте `manage` | остаётся как есть, в том числе несколько сразу |
| `rbac.deckhouse.io/level: module` | удаляется, её никто не читает |
| `rbac.deckhouse.io/namespace: d8-<модуль>` | остаётся — но читается только на объектах с областью `system` или `subsystem` |

Локализованные заголовки и описания теперь обязательны на каждом объекте, а их формулировки едины по
всей платформе (скрипт генерирует ровно такие):

```yaml
  annotations:
    en.meta.deckhouse.io/title: "Module <модуль>: view"
    ru.meta.deckhouse.io/title: "Модуль <модуль>: просмотр"
    en.meta.deckhouse.io/description: "Read-only access to <модуль> resources in a namespace."
    ru.meta.deckhouse.io/description: "Доступ только на чтение к ресурсам модуля <модуль> в пространстве имён."
```

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

И конфигурация модуля, которая остаётся в той же подсистемной линейке, что и была:

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

Модуль, у которого метки шаблонизированы через `helm_lib_module_labels`, оставляет их внутри вызова
`dict` — меняются те же пары и точно так же.

## Скрипт

`rbacv2-migrate-module.sh` обходит `**/templates/rbacv2/**`, переписывает каждую найденную capability
прежней схемы и отдельно перечисляет то, что не решился тронуть. На файлах платформы он воспроизводит
проделанную миграцию побайтово в 93 случаях из 95 (в оставшихся двух правили ещё и сами правила — это
решение, а не переписывание), так что механической части можно доверять.

```shell
./rbacv2-migrate-module.sh -n путь/к/модулю   # только диф
./rbacv2-migrate-module.sh путь/к/модулю      # переписать на месте
```

Скрипт идемпотентен: уже мигрированный файл он отметит как нетронутый. Файлы он не переименовывает,
поэтому диф остаётся читаемым.

### Чего скрипт решить не может

1. **На своём ли уровне лежит право.** Namespace-capability теперь агрегируется в namespace-роли, а
   правило на кластерный ресурс там не даёт ничего. В прежней схеме это сходило с рук, потому что
   `use`-capability попадала в кластерную подсистемную роль, где кластерное правило работало. Проверьте
   каждое правило в мигрированных файлах `use/`: если ресурс кластерный, перенесите правило в `manage/`.
2. **Уровень.** `view` → `viewer` и `edit` → `manager` — то, что использовало большинство модулей, и
   скрипт сохраняет найденное. В namespace-линейке есть ещё `user` — уровень тенанта, который просто
   работает в пространстве имён, и `admin` — того, кто им управляет. Если ваша `edit`-capability на
   самом деле нужна обычному тенанту, её уровень — `user`.
3. **Две линейки сразу.** У прежних `use`-capability иногда стояли и `kubernetes`, и `networking`;
   вторая отбрасывается, потому что namespace-правам в подсистемной линейке делать нечего. Если модулю
   действительно нужна и кластерная часть в `networking`, её место — в отдельной system-capability.
4. **Формулировки.** Сгенерированные заголовки и описания следуют общей конвенции и называют модуль по
   имени; прочитайте их один раз — именно их консоль показывает рядом с capability.

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
роль с `aggregationRule` и без собственных правил против capability с правилами и без `aggregationRule`,
уникальный маркер `rbac.deckhouse.io/capability` и хотя бы одна метка агрегации на каждой capability,
а также `rbac.deckhouse.io/delegatable` только на namespace- и проектных ролях.

В кластере с установленным модулем:

```shell
# Namespace-права доехали до namespace-ролей.
d8 k get clusterrole d8:namespace:viewer -o json | jq '[.rules[] | select(.apiGroups[] == "mygroup.io")]'

# Права на конфигурацию доехали до подсистемной роли.
d8 k get clusterrole d8:subsystem:networking:manager -o json | jq '[.rules[] | select(.resourceNames[]? == "mymodule")]'
```

## Что будет, если модуль не мигрировать

Старые объекты никто не отвергнет и ничего не залогирует — просто метки теперь читает другой код.
Три следствия, все проверены на живом кластере:

- **`use`-capability выдаётся на весь кластер вместо пространства имён.** Метка
  `aggregate-to-kubernetes-as` теперь принадлежит кластерной подсистеме `kubernetes`, поэтому
  namespace-правила модуля попадают в `d8:subsystem:kubernetes:*` и, вверх по лестнице, в
  `d8:system:*` — их получает каждый обладатель этих ролей во всём кластере. А в `d8:namespace:*` их
  больше нет вовсе, так что тенанты теряют доступ, который у них был.
- **`use`-capability уровня `user` или `admin` исчезает.** В подсистемных линейках таких уровней нет,
  объект не выбирает никто, и его правила не достаются никому.
- **Автоматический `RoleBinding` в пространстве имён модуля перестаёт создаваться.** Хук, который
  разворачивает `ClusterRoleBinding` на системную или подсистемную роль в namespace-привязки, смотрит
  только на объекты с меткой `rbac.deckhouse.io/scope: system` или `subsystem`, поэтому без неё метка
  `rbac.deckhouse.io/namespace` не будет прочитана.

`manage`-capability продолжает агрегироваться в ту же подсистемную роль, то есть права не переезжают, —
но она остаётся невидимой для консоли и для permission browser, которые различают объекты по
`rbac.deckhouse.io/kind` и `rbac.deckhouse.io/scope`.
