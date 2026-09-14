# Матрица исключений webhooks

Кого каждый validating webhook user-authz **не** проверяет, где исключение применяется и почему.
Механизмов два. **matchConditions** вычисляет apiserver до обращения к webhook-handler: исключённый
запрос модуля не касается, и исключение действует, пока handler недоступен, — именно это делает
fail-closed webhook безопасным для компонентов, которые должны уметь починить кластер.
**Проверки в теле** живут в коде хука; они важны для unit-тестов и для конфигурации, потерявшей свои
условия, в остальном дублируют matchConditions.

Администраторы кластера исключены только там, где это сказано в таблице. В остальных строках
`system:masters` проверяется как обычный пользователь; e2e имперсонирует cluster-admin не из masters
(`e2e-priv@example.com`), когда проверяется отказ webhook, пропускающего masters.

| Webhook (binding) | Объект | Пропускается на apiserver (matchConditions) | Проверка в теле | Почему |
|---|---|---|---|---|
| `rbacv2-cluster-roles.deckhouse.io` (`cluster_roles.py`) | ClusterRole CREATE/UPDATE — зарезервированные имена `d8:`, лейблы kind, правила custom-ролей, lineage | `system:apiserver`, SA `d8-system:deckhouse`, SA `kube-system:clusterrole-aggregation-controller` | нет | Роли модели пишут чарт и aggregation controller; никто другой не может занять имя `d8:`. Masters проверяется. |
| `role-validating.deckhouse.io` (`role_binding.py`) | ClusterRoleBinding CREATE — запрет CRB на роли тенантов (`d8:namespace:*`, `d8:project:*`) | `system:apiserver`, SA `d8-system:deckhouse`, SA `d8-user-authz:controller` | `PRIVILEGED_USERS` = те же три | user-authz-controller материализует биндинги правил; роль тенанта, привязанная на весь кластер, утекла бы из своих неймспейсов. Masters проверяется. |
| `rbacv2-system-resource-edit.deckhouse.io` (`system_resources.py`) | UPDATE/DELETE объектов с лейблом `deckhouse.io/system-resource` или `heritage: multitenancy-manager` | `system:apiserver`, SA `d8-system:deckhouse`, SA `kube-system:clusterrole-aggregation-controller`, SA `d8-multitenancy-manager:multitenancy-manager`; группы `system:masters`, `kubeadm:cluster-admins`, `superadmins`, `system:sudousers`, `system:nodes`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-system` | `PRIVILEGED_USERS` + `BYPASS_GROUPS` (те же наборы) + держатели `PRIVILEGED_ROLES` (`d8:{namespace,project,system}:superadmin`, `cluster-admin`, `user-authz:super-admin`) для системных ресурсов; heritage-объекты отклоняются и им | Break-glass для администраторов и компонентов; admission-политика multitenancy-manager защищает heritage-объекты от администраторов независимо от этого webhook. |
| `rbacv2-system-resource-exec.deckhouse.io` (`system_resources.py`) | CONNECT `pods/exec`, `pods/attach`, `pods/portforward` в системный под | те же четыре пользователя и те же группы, что у edit-binding (два SA добавлены спекой 004) | как у edit-binding; отказ называет все роли `PRIVILEGED_ROLES` и ничего лишнего | Exec матчит каждый под кластера; недоступный handler не должен стоять между компонентом или администратором и подом. |
| `rbacv2-role-escalation.deckhouse.io` (`role_escalation.py`) | Role/ClusterRole CREATE/UPDATE — мутирующие глаголы на kinds управления проектами | `system:apiserver`, SA `d8-system:deckhouse`, SA `kube-system:clusterrole-aggregation-controller`; группы `system:masters`, `kubeadm:cluster-admins` | `PRIVILEGED_USERS` + `BYPASS_GROUPS` (те же) | Администратор кластера может писать любую роль; администратор проекта не может выдать себе управление проектом через custom-роль. |
| `d8-user-authz-identity-assign.deckhouse.io` (`identity_privilege.py`, логика в `identity_assign.py`) | User user-authn CREATE/UPDATE/DELETE — идентичности нельзя выдать привилегии выше, чем у выдающего | `system:apiserver`, `system:sudouser`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`, `dhctl`, `observability`, SA `d8-system:deckhouse`; группа `system:masters` | `EXEMPT_USERS` (те же плюс SA `kube-system:clusterrole-aggregation-controller`) и `EXEMPT_GROUPS` (`system:masters`, `system:serviceaccounts:kube-system`) | Идентичностями управляют компоненты и администратор кластера; остальные ограничены собственной привилегией. |
| `d8-user-authz-group-authorization-rule-collision.deckhouse.io`, `d8-user-authz-user-authorization-rule-collision.deckhouse.io` (`identity_collision.py`) | Group / User user-authn CREATE/UPDATE/DELETE — имя, которое уже привязано ClusterAuthorizationRule или AuthorizationRule | `system:apiserver`, `system:sudouser`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`, `dhctl`, `observability`, SA `d8-system:deckhouse` | `EXEMPT_USERS` (те же плюс SA `d8-commander:cluster-manager`) и `EXEMPT_GROUPS` (`system:masters`, `system:serviceaccounts:kube-system`, `system:serviceaccounts:d8-system`) | Инсталляторы и администратор кластера заводят идентичности; защита от коллизий существует для self-service. **Расхождение**: masters исключён в теле, но не в matchConditions, поэтому при недоступном handler запрос masters отклоняется (fail-closed), хотя тело его пропустило бы. |
| `d8-user-authz-car-multitenancy-related-options.deckhouse.io`, `d8-user-authz-module-multitenancy-related-options.deckhouse.io` (`multitenancy.py`) | ClusterAuthorizationRule / ModuleConfig CREATE/UPDATE — EE-only поля multitenancy, гейт `enableMultiTenancy` | нет (binding по ModuleConfig сужен до `request.name == "user-authz"`) | нет | Валидация конфигурации применяется ко всем, включая masters: EE-only поле на CE-кластере ошибочно, кто бы его ни написал. |

## Смежные защиты вне модуля

- Точки admission multitenancy-manager (webhooks Project/ProjectTemplate, webhooks грантов, две
  admission-политики) описаны в своей таблице: `modules/160-multitenancy-manager/docs/internal/ADMISSION_BYPASS_MATRIX.md`.
- EE-авторизатор (`ee/be/.../authorizer/multitenancy/engine.go`) считает привилегированными для ответа
  AccessibleNamespaces группы `system:masters`, `kubeadm:cluster-admins` и `system:sudousers` — те же
  три, что исключает webhook системных ресурсов, плюс `superadmins`, которую перечисляет только webhook.
  Списки должны совпадать; разница зафиксирована здесь, пока один из них не изменят намеренно.

## Как сопровождать таблицу

- Исключение сначала добавляется в matchConditions (это и делает модуль неблокируемым), в тело — только
  если handler должен вести себя так же в изоляции; затем обновляется эта таблица и, для webhook
  системных ресурсов, контрактные тесты конфигурации в `system_resources_test.py`.
- Отказ называет webhook (`admission webhook "…" denied the request`); формулировка отказов webhook
  системных ресурсов выводится из `PRIVILEGED_ROLES`, поэтому роли, на которые он указывает, — всегда
  те, что проверка принимает.
