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
Сначала укажите подсистему модуля в поле `subsystems` файла `module.yaml` ([Подсистемы](#подсистемы)).

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

Результат проверьте dmt версии v0.2.7 или новее: если `dmt lint --linter rbac .` не выдаёт находок
`contract`, модуль мигрирован ([Проверка в dmt](#проверка-в-dmt)).

Описание RBAC модуля в `rbac.yaml` в эту миграцию не входит. Это [следующий шаг](#после-миграции-rbac-модуля-в-rbacyaml),
за который команда берётся, когда готова.

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
  - `namespace` заменяет прежний `use`;
  - `system` заменяет прежний `manage`, а подсистему, в которую агрегируется такая capability, называет лейбл линии `rbac.deckhouse.io/aggregate-to-<подсистема>-as` (`cluster`, `network` и так далее).

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
| `<подсистема>`, одна из [восьми подсистем платформы](#подсистемы) или [своя подсистема модуля](#своя-подсистема-модуля) | `viewer`, `manager`, `superadmin`                  | `ClusterRoleBinding`              | Кластерные ресурсы модуля и его `ModuleConfig`                      |
| `system`                                                                                                          | `viewer`, `manager`, `superadmin`                  | `ClusterRoleBinding`              | Напрямую ничего: системные роли агрегируют подсистемные             |
| `project`                                                                                                         | `viewer`, `user`, `manager`, `admin`, `superadmin` | `ProjectRoleBinding`              | Ресурсы структуры проекта (только multitenancy-manager)             |

Обычному модулю из всей таблицы нужны только две строки: `namespace` (бывший `use`) и своя подсистема
(бывший `manage`). Строки `system` и `project` модуль не заполняет сам — `system` агрегирует
подсистемные capabilities выше по цепочке, а `project` целиком относится к multitenancy-manager.

Миграция переносит namespace-уровень из подсистемы `kubernetes`, где он раньше жил вместе с `manage`,
в отдельную область `namespace`, которой он и принадлежит по смыслу.

### Подсистемы

В ролевой модели восемь подсистем. Это `iam`, `security`, `cluster`, `delivery`, `network`, `storage`,
`observability` и `managed-services`. Роли `d8:subsystem:<подсистема>:{viewer,manager,superadmin}` каждой
из них выпускает user-authz, поэтому модуль не выпускает роли подсистем платформы.

Таблица подсистем DKP, внутренний документ, относит каждый модуль DKP к одной из них. Перед запуском
скрипта укажите в поле `subsystems` файла `module.yaml` подсистему модуля из этой таблицы, скрипт её
читает.

В прежней схеме было семь подсистем. Каждая перешла в подсистему ролевой модели.

| Подсистема прежней схемы                    | Подсистема ролевой модели                                       |
| ------------------------------------------- | --------------------------------------------------------------- |
| `deckhouse`, `infrastructure`, `kubernetes` | `cluster`                                                       |
| `networking`                                | `network`                                                       |
| `observability`, `security`, `storage`      | та же                                                           |
| своя подсистема модуля (`virtualization`)   | та, которую модулю даёт таблица (у virtualization это `cluster`) |

`iam`, `delivery` и `managed-services` новые. Линию прежней подсистемы не агрегирует ни одна роль модели,
и периода, когда на capability висят оба лейбла, нет. Capability, оставшаяся на прежней линии, не попадает
ни в одну роль, поэтому скрипт переводит каждую линию подсистемы у system-capability. Если `module.yaml`
называет одну подсистему ролевой модели, все такие линии переходят в неё, какую бы подсистему они ни
называли раньше. Так скрипт делает переносы из таблицы подсистем DKP, например local-path-provisioner из
`infrastructure` в `storage` и user-authz из `security` в `iam`. Если `module.yaml` не называет ни одной
из них или называет несколько, скрипт переводит линии прежних подсистем по таблице выше и сообщает о
линии, которую `module.yaml` не объявляет. Поле `subsystems` в `rbac.yaml`, если оно есть, скрипт, как и
dmt, читает вместо `module.yaml`.

### Своя подсистема модуля

Восемь подсистем покрывают платформу. Модуль, который таблица подсистем DKP не относит ни к одной из них,
может держать свою подсистему. Сейчас таблица относит каждый модуль DKP к одной из восьми, так что это
оставлено для модуля не из DKP, и вебхук и правило `contract` в `dmt` защищают такую подсистему, пока
модель ролей её допускает.

- `module.yaml` объявляет подсистему в `subsystems` (`subsystems: [<имя>]`).
- Роли подсистемы `d8:subsystem:<имя>:{viewer,manager,superadmin}` модуль выпускает сам, по образцу ролей
  user-authz (`templates/rbacv2/global/subsystem/roles/cluster/`):

  ```yaml
  metadata:
    name: d8:subsystem:<имя>:manager
    annotations: {en|ru.meta.deckhouse.io/title|description: ...}
    labels:
      rbac.deckhouse.io/kind: role
      rbac.deckhouse.io/scope: subsystem
      rbac.deckhouse.io/subsystem: <имя>
      rbac.deckhouse.io/use-role: admin                     # viewer: viewer, superadmin: superadmin
      rbac.deckhouse.io/aggregate-to-<имя>-as: superadmin   # лестница; у viewer — manager, у superadmin нет
      rbac.deckhouse.io/aggregate-to-system-as: manager     # в d8:system:<уровень>
  aggregationRule:
    clusterRoleSelectors:
      - matchLabels: {rbac.deckhouse.io/aggregate-to-<имя>-as: manager}
        # use-capability прошлой схемы даёт права в неймспейсе, кластерная роль её не берёт
        matchExpressions: [{key: rbac.deckhouse.io/kind, operator: NotIn, values: [use]}]
  ```

- Его system-capabilities `d8:system-capability:<модуль>:<действие>` агрегируются лейблом
  `rbac.deckhouse.io/aggregate-to-<имя>-as`, как в любую подсистему.

Вебхук `rbacv2-cluster-roles` читает роли подсистем в кластере, поэтому линия такой подсистемы защищена так
же, как платформенная. Объект, который не является capability модели, не может нести её лейбл агрегации,
а роль, выдаваемая в пространстве имён или проекте, не может её агрегировать. Правило `contract` в `dmt`
принимает подсистему, которую объявляет `module.yaml` модуля. Другой модуль агрегироваться в неё не может,
потому что для линтера это чужая подсистема.

Модуль, который таблица относит к подсистеме платформы, не выпускает роли подсистемы даже временно. Два
Helm-релиза не могут владеть одним объектом, поэтому роль `d8:subsystem:cluster:*` в чарте модуля
конфликтует с ролью user-authz. У модуля virtualization в прежней схеме была своя подсистема, теперь он в
`cluster`. Его system-capabilities агрегируются лейблом `rbac.deckhouse.io/aggregate-to-cluster-as`, а
`d8:subsystem:cluster:*` выпускает user-authz.

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
| `rbac.deckhouse.io/aggregate-to-<подсистема>-as: <уровень>` на объекте `manage`                                         | тот же лейбл [подсистемы ролевой модели](#подсистемы), в которую перешла подсистема, уровень тот же, несколько сразу сохраняются; два, попавшие в одну подсистему, сливаются в один |
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

Скрипт правильно переписывает имена и лейблы, но не читает сами правила `rules` — поэтому семь вещей
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
   в `network`, её нужно вынести в отдельную system-capability, а не пытаться сохранить второй
   лейбл на namespace-capability.
4. **Формулировки.** Заголовки и описания, которые сгенерировал скрипт, следуют общей для платформы
   конвенции и называют модуль по имени. Прочитайте их один раз — именно их консоль показывает рядом с
   capability. Если написано некорректно - поправьте.
5. **Собственное действие модуля.** Capability, названная не `view` и не `edit`
   (`d8:use:capability:module:state-snapshotter:download_snapshots`), сохраняет своё действие:
   `d8:namespace-capability:state-snapshotter:download_snapshots`. Конвенции для её текстов нет, поэтому
   скрипт пишет их из имени действия и сообщает об этом. Перепишите их на обоих языках.
6. **Ось модуля и роль, которая её собирает.** Лейбл `rbac.deckhouse.io/aggregate-to-<ось>-as`, чья ось — не
   линия платформы (`backup-agent` у state-snapshotter), скрипт оставляет, чтобы роль, которая собирает
   capability, её не потеряла, и сообщает о нём: контракт 1.78 отклоняет такой лейбл как неизвестную линию.
   Capability только с такой осью скрипт не трогает: она не агрегируется ни в один уровень платформы, и это
   контракт тоже отклоняет. Выберите ей уровень и запустите скрипт снова. Роль со старым именем, которая
   только агрегирует (есть `aggregationRule` и нет правил), — не capability, и её скрипт тоже не трогает.
   Роли модуля в модели 1.78 нет. Поставьте обычную ClusterRole вне модели:
   - без `rbac.deckhouse.io/kind`;
   - с именем `d8:<модуль>:<имя>`;
   - с `aggregationRule` по лейблу модуля вне `rbac.deckhouse.io/`
     (`state-snapshotter.deckhouse.io/aggregate-to-backup-agent: "true"`).

   Этот лейбл повесьте на каждую capability, которую она собирает, во всех модулях, где такие есть. Старое
   имя оставьте алиасом в `templates/rbacv2-compat/`, как это делает user-authz: пустая ClusterRole, у
   которой `aggregationRule` повторяет селектор новой роли, с лейблом `rbac.deckhouse.io/deprecated: "true"`
   и аннотацией `rbac.deckhouse.io/deprecated-replaced-by`. Привязка к алиасу вызывает алерт
   `D8UserAuthzDeprecatedRBACv2ModuleRoleInUse` и релиз Deckhouse не держит. Аннотация обязательна:
   алиас без неё игнорируется с предупреждением в логе. Так же работает и следующее переименование роли
   `d8:<модуль>:<имя>`: у алиаса может быть любое имя `d8:`, кроме семейств ролей самой платформы
   (`d8:system:`, `d8:subsystem:`, `d8:namespace:`, `d8:project:`, `d8:custom:`, `d8:dict`).
7. **Своя подсистема модуля.** Модуль, у которого в прежней схеме была своя подсистема (`subsystems: [virtualization]`),
   называл свои объекты по ней:
   - `d8:use:capability:<модуль>:<действие>`;
   - `d8:manage:permission:subsystem:<модуль>:<действие>`;
   - namespace-роли `d8:use:role:<уровень>:<модуль>`;
   - подсистемные роли `d8:manage:<подсистема>:<уровень>`.

   Все они висели на линии подсистемы, а различались лейблом `rbac.deckhouse.io/kind`. Куда они попадут, решает
   `module.yaml`. Пока он объявляет свою подсистему модуля, подсистема остаётся за модулем. Когда вместо неё он
   объявляет одну подсистему платформы (`subsystems: [cluster]`), модуль переезжает в неё. Скрипт мигрирует их так:
   - use-capability становится namespace-capability: уровень переносится с линии подсистемы на
     `aggregate-to-namespace-as`, а сам лейбл линии снимается. Роли подсистемы в 1.78 выбирают только по нему, и
     namespace-capability с этим лейблом влила бы свои правила в кластерную роль;
   - manage-permission становится system-capability на линии своей подсистемы модуля или подсистемы платформы,
     в которую модуль переехал;
   - `d8:manage:<подсистема>:<уровень>` становится ролью подсистемы `d8:subsystem:<подсистема>:<уровень>` (kind `role`,
     scope `subsystem`, `aggregate-to-all-as` заменён на `aggregate-to-system-as`, селектор только по линии), пока
     подсистема остаётся за модулем. Когда модуль переезжает в подсистему платформы, у роли нет пары, потому что роли
     этой подсистемы выпускает user-authz, и она остаётся только в legacy-ветке гейта;
   - у namespace-роли модуля в 1.78 пары нет — capabilities собирает `d8:namespace:<уровень>`, — поэтому она остаётся
     только в legacy-ветке гейта, а с `--replace` скрипт оставляет её вам на удаление.

   Если на имя роли без пары есть привязки, сохраните его алиасом в `templates/rbacv2-compat/`, как в пункте 6. Алиас
   даёт не больше прав, чем давала роль. Он выбирает capabilities модуля по его лейблу `module`,
   `rbac.deckhouse.io/kind: capability` и лейблу линии на каждом уровне до своего включительно. Селекторы скрипт
   называет в отчёте. Остальное зависит от того, где роль выдаётся.
   - Алиас namespace-роли, `d8:use:role:<уровень>:<модуль>` или `d8:use:<модуль>:superadmin`, несёт лейбл
     `rbac.deckhouse.io/delegatable: "true"` и аннотацию `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"`,
     как алиасы `d8:use:role:*` у user-authz. RoleBinding на него внутри неймспейса проекта продолжает работать, а новый
     ProjectRoleBinding или ClusterProjectRoleBinding на него отклоняется.
   - Алиас роли подсистемы, `d8:manage:<модуль>:<уровень>`, не несёт ни того, ни другого. Он выдаётся через
     ClusterRoleBinding и в проект не попадает.

   В чарте, который сохраняет legacy-ветку, оберните алиас в гейт,
   `{{- if eq (include "<module>.rbacv2_new_scheme" .) "true" }}`, потому что ниже 1.78 legacy-ветка по-прежнему
   рендерит роль под тем же именем.

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: d8:use:role:user:mymodule
     annotations:
       en.meta.deckhouse.io/title: "Deprecated: d8:use:role:user:mymodule"   # и ещё три
       rbac.deckhouse.io/deprecated-replaced-by: d8:namespace:user
       rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"
     labels:
       heritage: deckhouse
       module: mymodule
       rbac.deckhouse.io/deprecated: "true"
       rbac.deckhouse.io/delegatable: "true"
   aggregationRule:
     clusterRoleSelectors:
       - matchLabels: {module: mymodule, rbac.deckhouse.io/kind: capability, rbac.deckhouse.io/aggregate-to-namespace-as: viewer}
       - matchLabels: {module: mymodule, rbac.deckhouse.io/kind: capability, rbac.deckhouse.io/aggregate-to-namespace-as: user}
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: d8:manage:mymodule:viewer
     annotations:
       en.meta.deckhouse.io/title: "Deprecated: d8:manage:mymodule:viewer"   # и ещё три
       rbac.deckhouse.io/deprecated-replaced-by: d8:subsystem:cluster:viewer
     labels:
       heritage: deckhouse
       module: mymodule
       rbac.deckhouse.io/deprecated: "true"
   aggregationRule:
     clusterRoleSelectors:
       - matchLabels: {module: mymodule, rbac.deckhouse.io/kind: capability, rbac.deckhouse.io/scope: system, rbac.deckhouse.io/aggregate-to-cluster-as: viewer}
   ```

   Кроме того, скрипт сообщает о маркере длиннее 63 символов: такой лейбл API-сервер отклонит, поэтому действие нужно
   сократить. Имя с сегментом `subsystem:` мигрируется, только если подсистема своя для модуля: её объявляет
   `module.yaml` сверх подсистем платформы или она названа по имени модуля. Шаблон, который пишет лейблы через
   `helm_lib_module_labels`, скрипт относит к модулю своего чарта (имя из `Chart.yaml`). Нераспознанную
   use-capability, на которой остался лейбл подсистемы, скрипт выводит в отчёт, её нужно мигрировать вручную.

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

А это конфигурация модуля. Её линия переходит в подсистему ролевой модели, которая приняла прежнюю, из
`networking` в `network`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    heritage: deckhouse
    module: mymodule
    rbac.deckhouse.io/aggregate-to-network-as: manager
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
скрипт принять не может (см. следующий раздел). Ещё одно отличие касается линий подсистем: скрипт
переводит их в подсистемы ролевой модели.

```shell
./rbacv2-migrate-module.sh -n PATH          # только диф
./rbacv2-migrate-module.sh PATH             # оставить обе модели под флагом версии
./rbacv2-migrate-module.sh --replace PATH   # только новая модель
```

Скрипт идемпотентен: уже мигрированный файл он отметит как нетронутый. В файле, мигрированном до того,
как подсистемы ролевой модели были утверждены, скрипт переводит линии подсистем, а в файле с флагом меняет
только новую ветку. Если прежний запуск написал `d8:subsystem:<модуль>:<уровень>` для модуля, который
`module.yaml` теперь относит к подсистеме платформы, скрипт сообщает, что эту роль нужно убрать, а в файле
с флагом убрать только новую ветку. Он же называет алиас, который нужен выпущенному имени
`d8:manage:<модуль>:<уровень>` ([пункт 7](#что-стоит-проверить-руками)). Файлы он не переименовывает,
поэтому диф остаётся читаемым.

### Как отдавать обе модели из одной ветки

Внешний модуль ставится на кластеры разных версий платформы, а две модели друг друга не видят: объект
одной для другой невидим. Поэтому по умолчанию скрипт оставляет объект прежней схемы, кладёт рядом
мигрированный и оборачивает оба:

```text
{{- if eq (include "<module>.rbacv2_new_scheme" .) "true" }}
<мигрированный объект>
{{- else }}
<объект прежней схемы, без изменений>
{{- end }}
```

Сам флаг попадает в `templates/_rbacv2_compat.tpl`, он пишется один раз на модуль. Флаг читает
`global.deckhouseVersion` и отвечает `true` начиная с DKP 1.78. Это значение — содержимое файла
`/deckhouse/version`, которое стартовый хук кладёт в глобальные values, и до рендера внешнего модуля
оно доходит как любое другое глобальное значение: `upmeter`, например, уже собирает из него
user-agent. Модуль, поставляемый как `ModulePackage`, получает его там же; только пакет
`Application` читает глобальные values как `.Platform.<путь>`, а capability RBACv2 так не поставляют.

В самом флаге важны шесть деталей:

- это значение равно `dev` на dev-сборке и `unknown`, если файла версии нет, а `semverCompare` на
  обоих падает, и падение рушит рендер всего модуля, поэтому всё, что не является версией, должно
  отвечать без сравнения;
- отвечает оно `true`, и это выбор направления ошибки, а не ставка на более вероятную ветку. В файле
  версии лежит `CI_COMMIT_TAG`, поэтому сборка с любой ветки говорит `dev`, и dev-стенд, собранный с
  `release-1.77`, неотличим от собранного с `main`. Из двух способов ошибиться опасен только один:
  новый объект на кластере прежней схемы инертен, потому что там никто не выбирает
  `aggregate-to-namespace-as`, и права модуля просто отсутствуют; use-capability прежней схемы на
  кластере с ролевой моделью и линией подсистемы,
  которую ролевая модель сохранила (`observability`, `security`, `storage`), агрегируется в
  `d8:subsystem:<подсистема>:<уровень>`, который выдаётся через `ClusterRoleBinding`, так что правила,
  рассчитанные на один неймспейс, становятся кластерными. Dev-стенду на релизной ветке ниже
  1.78, которому нужны объекты прежней схемы, достаточно поменять в хелпере `true` на `false` в своей
  сборке;
- сравнение идёт только по мажорной и минорной части, потому что semver ставит `1.78.0-rc.1` ниже
  `1.78.0`, и релиз-кандидат 1.78 иначе получил бы объект прежней схемы;
- в регулярном выражении стоит `[.]`, так как обратный слеш внутри строкового литерала Go-шаблона —
  ошибка разбора;
- версия пропускается через `toString`, потому что `deckhouseVersion: 1.78` без кавычек в values-файле
  это YAML-float, а `regexFind` на не-строке рушит рендер;
- `.Values.global` взят в скобки, иначе чарт, отрендеренный вообще без global-значений, падает на
  nil pointer вместо того, чтобы взять значение по умолчанию.

Тесты шаблонов самого модуля требуют внимания: тест, который ищет ClusterRole по имени, найдёт его
только в той ветке, которую выбрал рендер, а харнесс, не задающий `global.deckhouseVersion`, рендерит
новую схему. В таком тесте версию нужно задавать явно и покрывать обе ветки — заодно это самый
дешёвый способ убедиться, что флаг работает.

Модули платформы обязаны использовать `--replace`, а не просто предпочитать его: контрактный тест
платформы (`testing/rbacv2/`) читает каждый файл под `templates/rbacv2/**` как сырой YAML, и файл с
флагом его не проходит — по той же причине внутренние алиасы совместимости лежат в
`templates/rbacv2-compat/`. Внешних модулей это не касается: dmt рендерит чарт через helm и проверяет
уже готовые объекты, то есть видит только одну ветку.

Модулю, который в `module.yaml` уже требует 1.78 или новее, ветка прежней схемы не нужна: `--replace`
пишет только новый объект. Так же поступают модули платформы, они едут вместе с той версией, на
которую рассчитаны. Когда модуль перестаёт поддерживать кластеры ниже 1.78, флаг и ветку `else`
нужно убрать.

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

### Проверка в dmt

С версии v0.2.7 тот же контракт dmt проверяет в репозитории любого модуля. Правило `contract` линтера
`rbac` — это перенос `testing/rbacv2`, поэтому модуль из отдельного репозитория проверяется так же,
как платформа проверяет свои.

```shell
# В репозитории модуля:
dmt lint --linter rbac .
```

`contract` работает по отрендеренным ClusterRole из `templates/rbacv2/`, `rbac.yaml` ему не нужен.
Что он сообщает:

- имя, лейбл или аннотацию, которые нарушают контракт выше, и роль или capability, форма которой не
  соответствует её виду;
- лейбл агрегации в неизвестную линию, в том числе в подсистемы схемы до DKP 1.78 (`deckhouse`,
  `infrastructure`, `kubernetes`, `networking`), с указанием подсистемы, которая её заменила;
- `subsystems` в `module.yaml`, которые не совпадают с подсистемами, куда агрегируются системные
  capability;
- кластерный ресурс в namespace-capability: через RoleBinding такое правило ничего не выдаёт (первый
  пункт раздела [«Что стоит проверить руками»](#что-стоит-проверить-руками));
- по одной находке на каждый объект схемы до DKP 1.78, с предложением запустить скрипт. Объект прежней
  схемы под условием по версии, которое ставит скрипт, не считается: модуль, который отдаёт обе модели
  намеренно, проверку проходит.

Нет находок `contract` — модуль мигрирован. Правило выдаёт предупреждения и сборку не валит. Уровень
задаётся только в корневом `.dmtlint.yaml`, в `global.linters-settings.rbac.rules.contract.impact`; если
dmt запущен на каталог одного модуля, корневым считается `.dmtlint.yaml` этого модуля. Если в корневом
уровень правила не задан, берётся уровень всего линтера `rbac` модуля, когда он ниже `warn`:
`linters-settings.rbac.impact: ignored` в `.dmtlint.yaml` модуля выключает `contract` для этого модуля,
и «нет находок» тогда ничего не доказывает. `dmt lint remote` это правило не запускает.

С версии v0.2.7 `no-cyrillic` сам пропускает аннотации `ru.meta.deckhouse.io/title` и `description`,
которых требует ролевая модель. Исключать `templates/rbacv2` из `no-cyrillic` в `.dmtlint.yaml` модуля
больше не нужно. Своя проверка кириллицы, если она есть помимо dmt, тоже должна пропускать
`ru.meta.deckhouse.io/*`.

В кластере с установленным модулем:

```shell
# Namespace-права доехали до namespace-ролей.
d8 k get clusterrole d8:namespace:viewer -o json | jq '[.rules[] | select(.apiGroups[] == "mygroup.io")]'

# Права на конфигурацию доехали до подсистемной роли.
d8 k get clusterrole d8:subsystem:network:manager -o json | jq '[.rules[] | select(.resourceNames[]? == "mymodule")]'
```

## Чего алиасы совместимости не несут

Старые имена ролей живут один релиз как агрегирующие алиасы. Привязка к `d8:manage:*` или
`d8:use:role:*` сохраняет то, что давала старая роль, и это может быть уже, чем роль, которая её заменяет.
`d8:manage:deckhouse:*`, `d8:manage:infrastructure:*` и `d8:manage:kubernetes:*` выбирают только
capabilities модулей, которые были в этих подсистемах, а `d8:subsystem:cluster:*` охватывает всю
подсистему `cluster`. `d8:manage:security:*` по-прежнему даёт права на user-authn и user-authz, которые
переехали в `iam`. `d8:manage:networking:*` и `d8:manage:storage:*` выбирают линию `network` и `storage`
без keepalived и local-path-provisioner, которые пришли в эти подсистемы из `infrastructure`. Алиасы
`deckhouse`, `infrastructure`, `kubernetes` и `networking` и `d8:manage:all:*` выбирают ещё и старую линию
своей подсистемы, которую может по-прежнему нести модуль с собственным циклом выпуска. Алиасы, которые
модуль оставляет для своих ролей, тоже узкие ([пункт 7](#что-стоит-проверить-руками)).

ClusterRoleBinding на алиас `d8:manage:*` сохраняет RoleBinding, которые прошлый релиз создал для него в
неймспейсах модулей старой подсистемы, `d8:use:<роль>:binding:<имя>` на `d8:use:role:<роль>`, и
user-authz-controller поддерживает их, как и раньше. Пересоздание привязки на новое имя заменяет их на
RoleBinding на namespace-роль новой роли в неймспейсах модулей новой подсистемы.

Не возвращаются только `serviceaccounts/token` и `impersonate`, которые переехали с admin на superadmin
намеренно.

Проверьте и собственную документацию: примеры с `d8:manage:*` или `d8:use:role:*` работают только этот релиз.
Алиасы `d8:use:role:*` делегируемы, поэтому обычный RoleBinding на такой алиас внутри неймспейса проекта продолжает работать; отклоняется новый ProjectRoleBinding или ClusterProjectRoleBinding на старое имя, потому что алиас несёт аннотацию `rbac.deckhouse.io/disabled-for-direct-use-in-projects`. Алиасы, которые модуль оставляет для своих namespace-ролей, несут тот же лейбл и ту же аннотацию (см. [пункт 7](#что-стоит-проверить-руками)).

## Перед обновлением: роли с платформенным лейблом агрегации

ClusterRole с лейблом `rbac.deckhouse.io/aggregate-to-<lineage>-as` вливается в платформенную роль этой
линии, и контроллер агрегации не читает ничего, кроме лейбла. Вебхук `rbacv2-cluster-roles.deckhouse.io`
теперь допускает такой лейбл только на `custom-capability`. Вебхук видит записи: роль, которая уже несёт
лейбл, не будучи custom capability, после обновления продолжает агрегироваться, а её следующий `UPDATE` —
в том числе повторный apply из GitOps — будет отклонён. Такие роли экспортируются как
`d8_user_authz_foreign_aggregation_label{name, lineage, kind}` и поднимают алерт
`D8UserAuthzForeignAggregationLabel`; перечислите их до обновления:

```bash
d8 k get clusterroles -o json | jq -r '.items[]
  | select((.metadata.labels // {}) | to_entries | any(.key | startswith("rbac.deckhouse.io/aggregate-to-")))
  | select((.metadata.labels.heritage // "") != "deckhouse")
  | select((.metadata.labels["rbac.deckhouse.io/kind"] // "") != "custom-capability")
  | .metadata.name'
```

Каждую такую роль либо оформите как настоящую custom capability (`d8:custom:<scope>-capability:<имя>`,
`rbac.deckhouse.io/kind: custom-capability`, `rbac.deckhouse.io/scope: <scope>`, без `aggregationRule`),
либо снимите с неё лейблы `aggregate-to-*-as`.

## Что будет, если модуль не мигрировать

Старые объекты никто не отвергнет и ничего не залогирует — они останутся валидными Kubernetes-объектами.
Изменится не их валидность, а то, что читает по их лейблам остальная платформа. Три следствия.

- **`use`-capability выдаётся на весь кластер вместо неймспейса или не выдаётся вовсе.** Её лейбл линии
  теперь принадлежит кластерной подсистеме или никому. С лейблом подсистемы, которую ролевая модель
  сохранила (`observability`, `security`, `storage`), namespace-правила модуля попадают в
  `d8:subsystem:<подсистема>:*` и, вверх по цепочке агрегации, в `d8:system:*`, их получает каждый
  обладатель этих ролей во всём кластере. Лейбл `deckhouse`, `infrastructure`, `kubernetes` или
  `networking` не выбирает ни одна роль модели. В обоих случаях в `d8:namespace:*` этих правил больше
  нет, так что тенанты теряют доступ, который у них раньше был.
- **`use`-capability уровня `user` или `admin` исчезает совсем.** В подсистемных областях таких уровней
  нет, поэтому такую capability никто не выбирает, и её правила не достаются никому.
- **Автоматический `RoleBinding` в пространстве имён модуля перестаёт создаваться.** Есть хук, который
  разворачивает `ClusterRoleBinding` на системную или подсистемную роль в отдельные namespace-привязки.
  Он смотрит только на объекты с лейблом `rbac.deckhouse.io/scope: system` или `subsystem`. Без него
  лейбл `rbac.deckhouse.io/namespace` для хука просто не существует, и привязка не создаётся.

`manage`-capability на `observability`, `security` или `storage` в этом смысле безопаснее: она продолжает
агрегироваться в ту же подсистемную роль, то есть права никуда не переезжают. Но она остаётся невидимой
для консоли и для permission browser — они отличают роли от capabilities и capabilities друг от друга
именно по лейблам `rbac.deckhouse.io/kind` и `rbac.deckhouse.io/scope`, которых на немигрированном
объекте нет. На `deckhouse`, `infrastructure`, `kubernetes` или `networking` она не попадает ни в одну
роль, и её права пропадают, пока модуль не переведёт её в [подсистему ролевой модели](#подсистемы).

## После миграции: RBAC модуля в rbac.yaml

Для DKP 1.78 не обязательно. Когда модуль уже на ролевой модели, весь его RBAC можно описать в одном
файле — `rbac.yaml` в корне модуля:

- доступ к каждому ресурсу модуля во всех ролевых моделях: уровни RBACv2 `namespace` и `system` и уровни
  user-authz `legacy`;
- права ServiceAccount модуля (`serviceAccounts`);
- доступ других субъектов (`access`);
- доступ Prometheus к метрикам (`prometheusAccess`).

Шаблоны `templates/rbacv2/`, `templates/user-authz-cluster-roles.yaml`, `templates/**/rbac-for-us.yaml`
и `templates/rbac-to-us.yaml` после этого пишет dmt, а lint сверяет их с декларацией. Формат и все
правила описаны в [README линтера rbac в dmt](https://github.com/deckhouse/dmt/blob/main/pkg/linters/rbac/README.md),
раздел «The module RBAC declaration». Ниже — порядок работы.

### Правила

Кроме `contract` ([Проверка в dmt](#проверка-в-dmt)), в dmt v0.2.7 для декларации есть ещё два правила:

- `sync` — в каждом модуле, который рендерит RBAC-объекты. Без `rbac.yaml` он даёт одну находку
  «rbac.yaml is missing». С ним отрендеренные RBAC-объекты и декларация должны совпадать в обе стороны:
  правила как кортежи `(apiGroup, resource, resourceName, verb)`, лейблы агрегации, `roleRef`, субъекты и
  условия `{{ if }}` вокруг объектов. `sync` отвечает за три вида объектов: роли user-authz прежней
  модели, собственные capability модуля в RBACv2 и объекты, имена которых строит декларация. Всё
  остальное в рендере (`helm_lib_csi_controller_rbac`, роли контроллеров с произвольными или
  вычисляемыми именами) dmt не пишет и не проверяет, оно остаётся рукописным.
- `coverage` — в модулях с `rbac.yaml`: для каждого CRD из `crds/` в `resources` есть запись — уровни
  доступа или `noAccess` с причиной.

Оба по умолчанию выдают предупреждения, как и `contract`, и `impact: error` для всего линтера `rbac` их
не повышает. Уровни — в `global.linters-settings.rbac.rules.<правило>.impact` корневого `.dmtlint.yaml`,
исключения — в `linters-settings.rbac.exclude-rules.<правило>`. Правило с уровнем `ignored` не
запускается, и `--fix` за него ничего не пишет. В репозитории платформы все три правила стоят на `warn`,
пока модули не переведены на `rbac.yaml`.

Не запускайте для этого `dmt lint --fix` без `--linter`. Без него `--fix` применяет исправления всех
линтеров, а с v0.2.7 ещё и создаёт `rbac.yaml` в каждом модуле с RBAC-объектами и переписывает шаблоны
в каждом модуле, где `rbac.yaml` уже есть. Pre-commit-хук или шаг CI с `dmt lint --fix` должен
указывать `--linter` того линтера, который имеется в виду.

### Перед началом

1. **dmt v0.2.7 или новее.**
2. **Нет находок `contract`.** Шаблоны, которые пишет dmt, следуют `subsystems` из `module.yaml`, и
   подсистема, которая есть только в старых шаблонах, из них пропадёт.
3. **Чистое дерево.** Всё остальное закоммитьте или уберите: в первом коммите должно быть только то, что
   записал dmt.
4. **Значения, при которых рендерятся все RBAC-объекты.** dmt описывает только то, что отрендерил. Если
   часть RBAC-объектов стоит под условием, которое при значениях по умолчанию ложно, передайте
   `--values-file` со значениями, которые его включают, или `--matrix` (все сочетания openapi-значений
   модуля, сверх лимита — попарно). Во всех запусках ниже значения должны быть одинаковыми.
5. **В репозитории платформы** модуль рендерится только в дереве, подготовленном так же, как для
   `tests_dmt_lint` (`tools/dmt-lint.sh`: оверлеи редакций слиты, ссылки `/deckhouse/*` разрешены).
   Работайте в такой копии и переносите обратно только собственные файлы модуля — `rbac.yaml` и
   изменения в `templates/`, но не файлы оверлеев редакций, слитые в копию. В оверлеи редакций dmt
   ничего не пишет.
6. **Модуль, который отдаёт обе модели ролей** под условием по версии из скрипта
   (`rbacv2_new_scheme`, [Как отдавать обе модели из одной ветки](#как-отдавать-обе-модели-из-одной-ветки)),
   переходит только частично. `sync` сверяет с `rbac.yaml` ветку 1.78 такого шаблона и пропускает
   ветку прежней модели, но `--fix` никогда не переписывает шаблон под условием (переписывание
   выкинуло бы прежнюю ветку), а первый запуск не переносит в `rbac.yaml` права из таких
   `templates/rbacv2/`: их ресурсы получают `noAccess: "TODO"`. Такой модуль записывает уровни этих
   ресурсов в `rbac.yaml` руками, по ветке 1.78, и дальше правит шаблоны под условием тоже руками.
   Пока они расходятся, `sync` выдаёт на файл находку «The autofix leaves the file as it is», а когда
   совпадут, находка уходит. Вся процедура, в которой dmt пишет и `templates/rbacv2/`, применима,
   когда модуль больше не обслуживает кластеры ниже DKP 1.78 и условие с прежней веткой удалены.

### Шаг 1. Голый прогон dmt (первый коммит)

Перевод модуля — это два исправления, которые зависят друг от друга: сначала записать `rbac.yaml` по
шаблонам, потом переписать шаблоны по `rbac.yaml`. Запуск `--fix` выполняет только исправления тех
находок, которые `dmt lint` нашёл в начале этого запуска, и после исправлений модуль заново не
проверяет (в README: «`--fix` does not re-lint»). Поэтому один запуск не может сделать оба:

| Запуск | Что dmt видит в начале | Что делает |
|---|---|---|
| 1 | `rbac.yaml` нет; одна находка `sync` «rbac.yaml is missing» | Пишет `rbac.yaml` по рендеру. Шаблоны не трогает: в начале запуска декларации не было, сверять было не с чем |
| 2 | `rbac.yaml` есть, `sync` сверяет с ним шаблоны | Переписывает шаблоны, которые расходятся с декларацией, и создаёт недостающие файлы. Шаблон, который нельзя переписать без потерь (объект вне декларации, `include`, `range`, условие, которого нет в декларации), оставляет как есть, и находка говорит, что поменять руками |
| 3 | всё сходится | Ничего не меняет. Это контрольный запуск, на нём останавливаемся |

Если после первого запуска шаблоны уже совпадают с декларацией, второй запуск ничего не меняет и сам
становится контрольным: хватает двух. Цепочка конечна, потому что после переписывания декларация и
шаблоны совпадают. Если за пять запусков изменения не прекратились, это ошибка dmt — сообщите о ней.

Этот цикл останавливается сам. После каждого запуска он добавляет изменения в индекс, и запуск, который
ничего не добавил поверх индекса, последний. Ошибки dmt цикл оставляет в терминале: запуск, который
упал (нет dmt, ошибка рендера, неверный `--values-file`), ничего не меняет и иначе выглядел бы как
сошедшийся.

```shell
# В корне репозитория модуля, на чистом дереве.
dmt --version                 # v0.2.7 или новее
git add -A .
for i in 1 2 3 4 5; do
  dmt lint --linter rbac --fix . >/dev/null      # при необходимости --values-file или --matrix
  if [ -z "$(git status --porcelain . | grep -v '^[MADR] ')" ]; then
    echo "сошлось за $i запуск(а)"; break
  fi
  git add -A .
  [ "$i" = 5 ] && echo "НЕ сошлось за 5 запусков: остановиться и сообщить"
done
[ -f rbac.yaml ] || echo "rbac.yaml нет: dmt не отработал, смотрите его ошибки выше"
git status --short .          # первый коммит
dmt lint --linter rbac .      # что осталось решить человеку
```

Что первый запуск пишет в `rbac.yaml`:

- все RBAC-объекты, которые модуль рендерит сейчас и которые формат умеет описать;
- условия `{{ if }}` вокруг них — как `when`;
- `TODO` там, где решение за человеком: область ресурса без CRD, CRD, к которому не выдано ни одного
  пользовательского права, namespace-ресурс на системном уровне, условие, которое dmt не смог прочитать
  из текста шаблона;
- комментарий в начале файла: что декларация не описывает, переименования, переносы между файлами и
  объекты, которые есть в шаблонах, но не отрендерились при значениях запуска.

Первый запуск сохраняет имена объектов. У каждого вида объектов есть необязательное поле имени
(`roleName`, `bindingName`, `clusterRoleBindingName` и другие), и dmt заполняет его, если правило
размещения принимает существующее имя. Переименовывает он объект только там, где правило размещения
отвергает существующее имя и принимает производное, и пишет об этом в комментарии.

Права не меняются: рендер после цикла совпадает с рендером до него, кроме того, что назван в
комментарии (объект, который переезжает в файл, куда его кладёт декларация).

В первом коммите только вывод dmt, и это можно доказать: на свежем клоне базы тот же цикл с теми же
значениями даёт те же файлы байт в байт. Так на dmt v0.2.7 проверены пилоты:

- **user-authn** (deckhouse !697): два запуска, с `--values-file`, который включает
  `userAuthn.internal.dexUsersCRDs`. Первый записал `rbac.yaml` (547 строк), второй ничего не поменял,
  поэтому первый коммит — один этот файл. При значениях по умолчанию объекты user-api не рендерятся, и
  dmt пишет об этом в комментарии, а не описывает их.
- **security-events-manager**
  ([security-events-manager!71](https://fox.flant.com/deckhouse/platform-security/security-events-manager/-/merge_requests/71)):
  три запуска. Первый записал `rbac.yaml` (262 строки). Второй
  дописал `templates/rbacv2/manage/view.yaml` и `edit.yaml`: декларация создаёт системные capability
  модуля для подсистемы из `module.yaml`, а в шаблонах их не было. Остальные шаблоны он не тронул и
  объяснил почему: в них есть то, чего формат не описывает (`imagePullSecrets` у ServiceAccount), и
  переписывание это потеряло бы. Третий ничего не поменял.

### Шаг 2. Решения (второй коммит)

Теперь `dmt lint --linter rbac .` перечисляет то, что осталось решить человеку. Типичные решения, на
примерах пилотов:

- **`noAccess: "TODO"`** для CRD, к которому не выдано ни одной пользовательской роли: либо выдать
  уровни, либо написать причину. В обоих пилотах внутренние CRD (в user-authn — десять ресурсов хранилища
  Dex) не выдавались ни одной пользовательской роли, и им записан `noAccess` с причиной.
- **Уровни, которые шаблоны задают сейчас.** Запишите их как есть: перевод на `rbac.yaml` — не повод
  менять доступ. В user-authn create на `dexauthenticators/allow-access-to-kubernetes` остаётся на
  системном уровне.
- **То, чего формат не описывает** и что не даёт `--fix` переписать файл:
  - `imagePullSecrets` у ServiceAccount: задайте их в спецификации подов (security-events-manager);
  - Role, которую объявленный аккаунт только привязывает (`bindRoles`), но которая лежит в его
    `rbac-for-us.yaml`: перенесите её в файл, который декларация не пишет (security-events-manager:
    `templates/controller/log-shipper-secrets.yaml`);
  - лейблы и аннотации ролей прежней модели или capability, для которых в формате нет поля.
- **Файлы, которые dmt не удаляет.** `--fix` никогда не удаляет файлы и не переносит объекты между
  файлами. Если декларация кладёт объект в другой файл (user-authn: `templates/user-api/rbac.yaml` должен
  называться `rbac-for-us.yaml`), перенесите или переименуйте его руками; до этого объект описан дважды.
- **Объекты, которые dmt напишет, а модуль поставлять не должен.** В security-events-manager
  сгенерированные системные capability `view` и `edit` дали бы ролям подсистемы ModuleConfig модуля и
  проекцию в его пространство имён, которых сейчас нет ни у одной роли. Их удалили и исключили из `sync`
  в `.dmtlint.yaml`, чтобы `--fix` не вернул их обратно; заголовок `rbac.yaml` об этом говорит.
- **Побочные эффекты за пределами модуля**, если шаблоны читает какой-то инструмент. В user-authn
  переименование в `rbac-for-us.yaml` добавило ServiceAccount user-api в политику аудита, которую
  `tools/audit_policy` собирает из `templates/**/rbac-for-us.yaml`; её перегенерировали в том же коммите.
- **Комментарии.** Комментарии шаблонов теперь место рядом с записями `rbac.yaml`, а заголовок
  `rbac.yaml` должен называть шаблоны, которые dmt из него пишет.

Потом снова запустите цикл из шага 1 с теми же значениями: следующий запуск перепишет шаблоны под
исправленную декларацию, а то, что из этого следует, увидит только запуск после него. Результат —
второй коммит.

Держите коммиты раздельно. Первый — это то, что записал dmt, и его можно воспроизвести; второй — то, что
решил человек. Ревьюер читает их отдельно, и никому не приходится гадать, какая строка `rbac.yaml`
пришла из dmt.

### Когда готово

- `dmt lint --linter rbac .` ничего не выдаёт — со значениями по умолчанию и со значениями цикла (и с
  `--matrix`, где это важно);
- ещё один `dmt lint --linter rbac --fix .` ничего не меняет;
- рендер отличается от базы только тем, что второй коммит меняет намеренно, и в описании изменения это
  сказано. Сравните RBAC-объекты модуля, отрендеренные на базе и на результате, с одинаковыми значениями.

Модуль, который отдаёт обе модели ролей, приходит к тому же состоянию, но шаблоны под условием он ведёт
руками и держит в согласии с `rbac.yaml` (пункт 6 из [Перед началом](#перед-началом)).

Дальше источник — `rbac.yaml`: правите его, запускаете цикл, коммитите и то и другое.
