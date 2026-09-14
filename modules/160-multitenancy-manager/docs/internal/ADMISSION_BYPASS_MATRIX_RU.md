# Матрица исключений admission

Кого *не* проверяет каждая точка admission multitenancy-manager, где живёт исключение и почему. Механизмов два.
**matchConditions** вычисляет apiserver до обращения к webhook или к CEL-правилам: исключённый запрос модуля не
касается вовсе. **Backstop в хендлере** — проверка внутри кода webhook; она важна для unit-тестов и для конфигурации
webhook, потерявшей свои условия, в остальном дублирует matchConditions.

`system:masters` исключён только там, где это сказано в таблице. Во всех остальных строках администратор кластера
проверяется как любой другой пользователь; ответ на вопрос «почему я как admin не могу сделать X» обычно в этой
таблице.

| Точка admission | Объект | Пропускается на apiserver (matchConditions) | Backstop в хендлере | Почему исключение |
|---|---|---|---|---|
| `projects.multitenancy-webhook` (`/validate/v1alpha3/projects`) | Project CREATE/UPDATE | `system:apiserver`, SA `d8-system:deckhouse`, SA `d8-multitenancy-manager:multitenancy-manager`, SA `d8-user-authz:controller`; группы `system:serviceaccounts:d8-system`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-user-authz`, `system:masters`, `system:nodes` (`templates/admission/validation.yaml`, `$excludeSystemWriters`) | нет | Helm-релиз модуля и адопция создают Project от системных идентичностей; при `failurePolicy: Fail` отказ заблокировал бы очередь модуля. `system:masters` здесь ради super-admin.conf: платформа должна уметь починить Project, который webhook отклонил бы; правила имён к masters не применяются. |
| `projecttemplates.multitenancy-webhook` (`/validate/v1alpha1/templates`) | ProjectTemplate | тот же `$excludeSystemWriters` | `templatewebhook.Register(..., serviceAccount)` пропускает SA контроллера при установке встроенных шаблонов | Та же причина; контроллер переписывает встроенные шаблоны при каждом старте. |
| webhooks `projectrolebindings`, `clusterprojectrolebindings`, `projectnamespaces` | PRB / CPRB / ProjectNamespace | **никто** | webhook PRB: `isPrivileged` для SA контроллера на его собственных fan-out-записях | Эти объекты пишут пользователи, контроллер их не создаёт. `system:masters` проверяется: masters получает те же отказы, что администратор проекта (e2e-проверки `--as T_PRIV` это отражают). |
| `/is-granted` (guardrail грантов) | любой объект использования, который называет GrantableClusterResourceReference | тот же набор, что у webhook Project (`hooks/configure_grant_validation_webhook.go`, `systemWriterMatchConditions`) | `isAutomatedSystemWriter`: группы `system:nodes`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-system`, `system:serviceaccounts:d8-user-authz` — **без** `system:masters`, без имён пользователей (`internal/webhooks/is_granted.go`) | Helm-релизы модулей и fan-out user-authz кладут проверяемые объекты в неймспейсы проектов; отказ заблокировал бы модуль. Backstop уже условий намеренно: прямой вызов хендлера (unit-тесты) проверяет и cluster-admin. |
| `/defaults` (мутирующий: FillEmpty / Coerce) | те же | те же `systemWriterMatchConditions` | `isSystemRequest`: имена пользователей + группы, включая `system:masters` (`internal/webhooks/protect.go`) | Как у `/is-granted`; backstop шире, потому что мутацию объекта системного писателя его владелец откатил бы при следующем apply. |
| `/protect` (каталог только для чтения) | AvailableClusterResource | те же `systemWriterMatchConditions` | `isSystemRequest` + SA контроллера | Каталог принадлежит контроллеру; со спеки 004 он ещё несёт `heritage: multitenancy-manager`, и политика heritage ниже отклоняет его fail-closed даже при недоступном webhook. |
| VAP `d8-multitenancy-manager` (защита heritage) | любой объект с `heritage: multitenancy-manager`, включая Namespace | группы `system:nodes`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-system`; пользователи `system:sudouser`, `system:apiserver`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`, `dhctl`, `observability`; `rollout restart` (аннотация restartedAt) (`templates/validation.yaml`) | н/д (CEL) | Только компоненты кластера. **`system:masters` не исключён**: объект, принадлежащий шаблону, меняется через ProjectTemplate или Project. На **Namespace** проекта UPDATE, затрагивающий только лейблы и аннотации вне ключей модуля, разрешён любому, кому позволяет RBAC (спека 004, Q29). |
| VAP `d8-multitenancy-manager-namespace-ownership` | Namespace CREATE/UPDATE, ставящий или меняющий `projects.deckhouse.io/project` | SA `d8-multitenancy-manager:multitenancy-manager`, SA `d8-system:deckhouse` | н/д (CEL) | По лейблу владения контроллер решает, чей это неймспейс; поставленный вручную лейбл не давал ни адопции, ни защиты (спека 004, Q13c). **`system:masters` не исключён.** Снятие лейбла здесь не запрещено. |
| webhook user-authz `system_resources` (edit/exec системных подов, heritage-объекты) | Pods, объекты модулей | см. `modules/140-user-authz/docs/internal/WEBHOOK_BYPASS_MATRIX.md` | — | Принадлежит user-authz; здесь потому, что это вторая защита вокруг неймспейсов проектов, которая администраторов кластера *исключает* (роли superadmin, `cluster-admin`). |

## Как читать таблицу

- Отказ от webhook называет webhook (`admission webhook "…" denied the request`); отказ от политики называет политику
  (`ValidatingAdmissionPolicy '…' with binding '…' denied request`).
- Тесты, которым нужен отказ от webhook, пропускающего masters, имперсонируют cluster-admin не из masters
  (`e2e-priv@example.com`, привязан к `cluster-admin`) — обе admission-политики отклоняют и эту идентичность.
- Добавляя исключение, добавляйте его в matchConditions (именно они делают модуль неблокируемым) и, только если
  хендлер должен вести себя так же в изоляции, в backstop; затем обновите эту таблицу.
