---
title: "Гранулярная модель авторизации"
permalink: ru/admin/configuration/access/authorization/rbac-experimental.html
description: "Настройка гранулярной модели RBAC-авторизации в Deckhouse Platform: области действия ролей, уровни доступа, проектные роли"
lang: ru
---

Гранулярная [ролевая модель](../../../../glossary.html#rbac) построена на принципе агрегации: она объединяет низкоуровневые роли в более крупные, охватывающие типовые задачи. Это упрощает расширение модели за счёт добавления собственных ролей.

Для использования гранулярной ролевой модели в кластере должен быть включён модуль [`user-authz`](/modules/user-authz/).
Модуль создаёт набор специальных агрегированных кластерных ролей (ClusterRole), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %}
В модуле реализованы две ролевые модели: гранулярная (описанная на этой странице, рекомендуется к использованию) и [упрощённая](rbac-current.html), построенная на ресурсах ClusterAuthorizationRule и AuthorizationRule (поддержка будет прекращена в будущих релизах).

Модели не совместимы по ресурсам — автоматическая конвертация невозможна, — но могут использоваться одновременно: права из обеих моделей суммируются.
{% endalert %}

В отличие от [упрощённой ролевой модели](rbac-current.html), гранулярная не использует ресурсы [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Права доступа настраиваются стандартным для RBAC Kubernetes способом: с помощью создания ресурсов [RoleBinding или ClusterRoleBinding](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#rolebinding-and-clusterrolebinding), с указанием в них одной из подготовленных модулем `user-authz` ролей. Для доступа сразу ко всем неймспейсам [проекта](../../../../glossary.html#project) дополнительно используются ресурсы [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) модуля `multitenancy-manager`.

{% alert level="info" %}
Выдавать доступ можно не только вручную через YAML-манифесты: в [веб-интерфейсе Deckhouse](../../../../user/web/ui.html) есть мастер выдачи доступа. Он проводит по шагам (кому выдать доступ → где → с каким уровнем), автоматически выбирает подходящий тип привязки (RoleBinding, ClusterRoleBinding, ProjectRoleBinding или ClusterProjectRoleBinding) и позволяет собрать собственную роль из готовых блоков без написания YAML.
{% endalert %}

Модуль создаёт специальные агрегированные кластерные роли (ClusterRole). Используя эти роли в RoleBinding или ClusterRoleBinding, можно решать следующие задачи:

- Управлять доступом к модулям определённой [подсистемы](#подсистемы-ролевой-модели).

  Например, чтобы дать возможность пользователю, выполняющему функции сетевого администратора, настраивать *сетевые* модули (например, [`cni-cilium`](/modules/cni-cilium/), [`ingress-nginx`](/modules/ingress-nginx/), [`istio`](/modules/istio/) и т. д.), можно использовать в ClusterRoleBinding роль `d8:subsystem:network:manager`.
- Управлять доступом к *пользовательским* ресурсам модулей в рамках неймспейсов.

  Например, использование роли `d8:namespace:manager` в RoleBinding позволит удалять/создавать/редактировать ресурс [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) в неймспейсе, но не даст доступа к cluster-wide-ресурсам [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) и [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) модуля `log-shipper`, а также не даст возможность настраивать сам модуль `log-shipper`.

Роли, создаваемые модулем, делятся на следующие классы:

- [Namespace-роли](#namespace-роли) — для назначения прав пользователям (например, разработчикам приложений) **в конкретном неймспейсе**.
- [Проектные роли](#проектные-роли) — для назначения прав **сразу во всех неймспейсах проекта**.
- [Системные и подсистемные роли](#системные-и-подсистемные-роли) — для назначения прав администраторам платформы и администраторам части платформы.

## Области действия ролей

Каждая роль действует в одной из четырёх областей. Область определяет, *где* работают выданные права и *каким ресурсом* роль назначается:

| Область | Формат имени роли | Для кого | Каким ресурсом назначается |
|---------|-------------------|----------|-----------------------------|
| Неймспейс | `d8:namespace:<уровень>` | Пользователи приложений (разработчики) | RoleBinding в конкретном неймспейсе |
| Проект | `d8:project:<уровень>` | Команды, работающие с [проектами](/modules/multitenancy-manager/) | [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) или [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) — на весь проект, RoleBinding — на один неймспейс проекта |
| [Подсистема](../../../../glossary.html#subsystem) | `d8:subsystem:<подсистема>:<уровень>` | Администраторы части платформы | ClusterRoleBinding |
| Вся платформа | `d8:system:<уровень>` | Администраторы платформы | ClusterRoleBinding |

### Уровни доступа

В каждой области действия для ролей предусмотрено несколько уровней доступа:

- Для областей «неймспейс» и «проект» уровней пять: `viewer` → `user` → `manager` → `admin` → `superadmin`.
- Для областей «подсистема» и «вся платформа» уровней три: `viewer` → `manager` → `superadmin`. Уровней `user` и `admin` здесь нет: на системном уровне нет «пользовательских» ресурсов, которыми можно было бы пользоваться, не администрируя их.

Уровни доступа образуют иерархию с кумулятивным принципом: каждый последующий уровень наследует все права предыдущего и расширяет их. Например, уровень `manager` в области действия «неймспейс» включает набор прав уровня `user`, который, в свою очередь, включает набор прав уровня `viewer`.

## Namespace-роли

{% alert level="warning" %}
Namespace-роль нельзя назначить через ClusterRoleBinding. Используйте RoleBinding в неймспейсе либо ProjectRoleBinding или ClusterProjectRoleBinding для всех неймспейсов проекта.
{% endalert %}

Namespace-роли предназначены для назначения прав пользователю **в конкретном неймспейсе**. Под пользователями понимаются, например, разработчики, которые используют настроенный администратором кластер для развёртывания своих приложений. Таким пользователям не нужно управлять модулями DP или кластером, но им нужно иметь возможность, например, создавать свои Ingress-ресурсы, настраивать аутентификацию приложений и сбор логов с приложений.

Namespace-роль определяет права на доступ к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes (Pod, Deployment, Secret, ConfigMap и т. п.).

В DP создаются следующие namespace-роли:

| Роль | Разрешённые действия |
|------|----------------------|
| `d8:namespace:viewer` | Просмотр стандартных ресурсов Kubernetes (кроме секретов и ресурсов RBAC), ресурсов Gateway API, логов подов и метрик, аутентификация в кластере |
| `d8:namespace:user` | Всё, что даёт `viewer`, плюс: просмотр секретов, `kubectl exec`/`attach`, удаление подов (без создания/изменения), `kubectl port-forward`/`proxy`, изменение числа реплик контроллеров |
| `d8:namespace:manager` | Всё, что даёт `user`, плюс: управление ресурсами модулей (например, Certificate, PodLoggingConfig) и стандартными namespaced-ресурсами Kubernetes (Pod, Deployment, ConfigMap, Secret, Service, Ingress, NetworkPolicy, CronJob и т. п.), а также ресурсами Gateway API (Gateway, ListenerSet, HTTPRoute, GRPCRoute, ReferenceGrant и т. п.) |
| `d8:namespace:admin` | Всё, что даёт `manager`, плюс: управление ResourceQuota, LimitRange, ServiceAccount, Role, RoleBinding |
| `d8:namespace:superadmin` | Всё, что даёт `admin`, плюс опасные с точки зрения безопасности операции: выпуск токенов ServiceAccount и запросы от их имени, управление [системными ресурсами в неймспейсе](#ограничения-уровня-admin-и-права-superadmin) (например, подами Dex или подами/PVC виртуальных машин) |

Подробное разделение прав между `admin` и `superadmin` описано в разделе [«Ограничения уровня admin и права superadmin»](#ограничения-уровня-admin-и-права-superadmin).

### Автоматический доступ к справочным cluster-wide-ресурсам

Работа в неймспейсе требует чтения некоторых cluster-wide-«справочников»: например, чтобы указать в манифесте `storageClassName` или `ingressClassName`, нужно видеть список StorageClass'ов и IngressClass'ов. Поэтому каждый субъект, получивший RoleBinding на любую роль `d8:namespace:*` или `d8:project:*`, автоматически получает и доступ на чтение таких справочных ресурсов (StorageClass, IngressClass, PriorityClass, RuntimeClass, VolumeSnapshotClass, ClusterLogDestination и т. п.).

Технически это выглядит как автоматически создаваемый ClusterRoleBinding на роль [`d8:dict`](#глобальные-справочники-ресурсов) с лейблом `rbac.deckhouse.io/dict: "true"`. Такие объекты управляются платформой: они появляются при выдаче первой namespace-привязки субъекту и удаляются, когда у субъекта не остаётся ни одной, — редактировать их вручную не нужно.

## Проектные роли

{% alert level="warning" %}
Проектную роль нельзя назначить через ClusterRoleBinding — попытка будет отклонена. Для назначения роли на весь [проект](../../../../glossary.html#project) используйте [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) или [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). Допускается также обычный RoleBinding в одном из неймспейсов проекта — тогда роль действует только в нём.
{% endalert %}

Проектные роли (`d8:project:<уровень>`) предназначены для работы с [проектами](/modules/multitenancy-manager/) — изолированными окружениями, которые могут включать несколько неймспейсов. Для проектных ролей используются те же уровни доступа, что для namespace-ролей: `viewer`, `user`, `manager`, `admin`, `superadmin`.

Каждая проектная роль включает все права namespace-роли того же уровня и дополнительно даёт права на управление самим проектом:

- `d8:project:viewer` — права `d8:namespace:viewer` плюс просмотр ресурсов ProjectNamespace и ProjectRoleBinding проекта;
- `d8:project:user` — права `d8:namespace:user` плюс просмотр ресурсов ProjectNamespace и ProjectRoleBinding проекта;
- `d8:project:manager` — права `d8:namespace:manager` плюс управление [дополнительными неймспейсами проекта](../../../../glossary.html#additional-project-namespace) (ресурсы ProjectNamespace);
- `d8:project:admin` — права `d8:namespace:admin` плюс управление доступом к проекту (ресурсы ProjectRoleBinding) и право привязывать встроенные роли `d8:project:*` и `d8:namespace:*` (кроме уровня `superadmin`) другим пользователям в рамках проекта;
- `d8:project:superadmin` — аналогично соотношению `d8:namespace:superadmin` и `d8:namespace:admin`.

Назначенная через ProjectRoleBinding роль автоматически действует во **всех** неймспейсах проекта — как в основном, так и в дополнительных, включая созданные позже.

## Системные и подсистемные роли

{% alert level="warning" %}
Системные и [подсистемные](../../../../glossary.html#subsystem) роли не дают доступа к неймспейсам пользовательских приложений.

Они определяют доступ только к [системным неймспейсам](../../../../glossary.html#system-namespace) (начинающимся с `d8-` или `kube-`), и только к тем из них, в которых работают модули соответствующей подсистемы роли.

Можно ли с такой ролью раздавать доступ другим пользователям, описано в разделе [«Кто может назначать роли»](#кто-может-назначать-роли).
{% endalert %}

Системные (`d8:system:*`) и подсистемные (`d8:subsystem:*`) роли предназначены для назначения прав на управление всей платформой или её частью ([подсистемой](#подсистемы-ролевой-модели)), но не самими приложениями пользователей. С помощью подсистемной роли можно, например, дать возможность администратору безопасности управлять модулями, ответственными за функции безопасности кластера. Тогда администратор безопасности сможет настраивать политики безопасности, выпуск сертификатов и т. п., но не сможет управлять остальными функциями кластера (например, управлением доступом, настройками сети и мониторинга) и изменять настройки в неймспейсах приложений пользователей.

{% alert level="warning" %}
Системная/подсистемная роль ограничивает, к каким модулям и неймспейсам обращается субъект, но не ограничивает привилегии, которые он может получить через доступные ему модули. Это особенно важно учитывать для подсистемы `iam`: право управлять аутентификацией и авторизацией равносильно полному контролю над кластером — субъект, который может управлять модулем `user-authn`, способен зарегистрировать провайдер идентификации или сбросить учётные данные любого локального пользователя, а субъект, который может управлять модулем `user-authz`, способен изменить правила авторизации. В обоих случаях он может получить идентичность с любыми привилегиями, включая администратора кластера, поэтому при планировании доступа считайте роль подсистемы `iam` равной роли администратора кластера.
{% endalert %}

Системная/подсистемная роль определяет права на доступ:

- к cluster-wide-ресурсам Kubernetes;
- к управлению модулями DP (ресурсы ModuleConfig) в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми модулями DP для роли `d8:system:*`;
- к управлению cluster-wide-ресурсами модулей DP в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми ресурсами модулей DP для роли `d8:system:*`;
- на чтение неймспейсов модулей [подсистемы](#подсистемы-ролевой-модели) роли или всех модулей DP для роли `d8:system:*`; запись в них оставлена уровню `superadmin` (подробнее — в разделе [«Доступ системных и подсистемных ролей в неймспейсы модулей»](#доступ-системных-и-подсистемных-ролей-в-неймспейсы-модулей)).

Формат названия системной роли — `d8:system:<ACCESS_LEVEL>`, подсистемной — `d8:subsystem:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:

- `SUBSYSTEM` — подсистема роли ([список подсистем](#подсистемы-ролевой-модели));
- `ACCESS_LEVEL` — уровень доступа.

Примеры ролей:

- `d8:system:viewer` — доступ на просмотр конфигурации всех модулей DP (ресурсы ModuleConfig), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в неймспейсах всех модулей DP;
- `d8:system:manager` — аналогично роли `d8:system:viewer`, только с правом просмотра/создания/изменения/удаления конфигурации всех модулей DP (ресурсы ModuleConfig) и их cluster-wide-ресурсов; в неймспейсах модулей DP у неё остаётся доступ на чтение, как у `d8:system:viewer`;
- `d8:subsystem:observability:viewer` — доступ на просмотр конфигурации модулей DP (ресурсы ModuleConfig) из подсистемы `observability`, их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в системных неймспейсах `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

В DP существует три уровня доступа для системных и подсистемных ролей:

- `viewer` — позволяет просматривать стандартные ресурсы Kubernetes, конфигурацию модулей (ресурсы ModuleConfig), cluster-wide-ресурсы модулей и namespaced-ресурсы модулей в неймспейсе модуля;
- `manager` — дополнительно к уровню `viewer` позволяет управлять конфигурацией модулей (ресурсы ModuleConfig) и cluster-wide-ресурсами модулей; в неймспейсах модулей даёт тот же доступ на чтение, что и `viewer`;
- `superadmin` — дополнительно к уровню `manager` позволяет всё в неймспейсах модулей: управлять стандартными ресурсами Kubernetes и namespaced-ресурсами модулей, читать секреты, подключаться к подам, выпускать токены ServiceAccount и управлять системными ресурсами модулей подсистемы.

### Доступ системных и подсистемных ролей в неймспейсы модулей

Обладатель системной или подсистемной роли автоматически получает RoleBinding на namespace-роль в каждом системном неймспейсе, где работают модули подсистемы роли (роль `d8:system:*` охватывает неймспейсы всех модулей DP): уровни `viewer` и `manager` получают `d8:namespace:viewer`, уровень `superadmin` — `d8:namespace:superadmin`. Платформа сама создаёт и удаляет эти RoleBinding (на них добавляется лейбл `rbac.deckhouse.io/automated: "true"`), по одному на каждый ClusterRoleBinding роли.

Неймспейсы модулей принадлежат платформе. В них работают ServiceAccount'ы с правами на весь кластер (в том числе [контроллер Deckhouse](../../../../glossary.html#deckhouse-controller)) и хранятся учётные данные модулей, поэтому любая запись туда и любое чтение Secret там — это по сути право действовать от имени этих ServiceAccount'ов где угодно в кластере. Поэтому `manager` настраивает свою подсистему через объекты ModuleConfig и cluster-wide-ресурсы модулей, а в неймспейсах модулей только просматривает объекты, логи, метрики и события. `manager` не может читать там секреты, выполнять `kubectl exec`, `attach` и `port-forward`, удалять и масштабировать поды и создавать что-либо. `superadmin` может всё это. Выдавайте этот уровень осознанно и именованному субъекту: это аварийный путь внутрь платформы, и его действия видны в журнале аудита. Не подменяйте его имперсонацией системных пользователей (`--as`).

При работе с системными и подсистемными ролями учитывайте следующие особенности:

- RoleBinding на `d8:namespace:user` или более высокий уровень в системном неймспейсе, созданный вручную, выдаёт субъекту права всех ServiceAccount'ов этого неймспейса. То же относится к параметру `allowAccessToSystemNamespaces: true` в ClusterAuthorizationRule [упрощённой ролевой модели](rbac-current.html): с ним уровни доступа `Admin` и `ClusterAdmin` равны администратору кластера.
- Системные неймспейсы — те, которые модуль `multitenancy-manager` никогда не превращает в [проекты](../../../../glossary.html#project): `d8-*`, `kube-*`, `upmeter-*`, `default` и неймспейсы с лейблом `heritage: deckhouse` или `heritage: upmeter`.
- Собственная системная или подсистемная роль (`d8:custom:*`) проецируется как `viewer`, что бы ни стояло в её лейбле `rbac.deckhouse.io/use-role`: вебхук отклоняет большее значение, а роль, созданная с ним раньше, проецируется как `viewer` с событием `ProjectionCapped` на роли.

### Кто может назначать роли

Системная или подсистемная роль сама по себе не даёт права раздавать доступ другим людям.

Создать User или Group, которых ещё нет среди субъектов грантов, можно как обычный объект.

Создать User на email, на котором грант уже есть, записать или удалить ClusterAuthorizationRule либо сбросить такую идентичность через UserOperation — это раздача ролей. Запрос пройдёт, только если у запрашивающего уже есть покрывающие права или ему явно разрешено назначать эти роли.

Подключение DexProvider проверяется так же, но относительно идентичностей, которые провайдер способен утвердить: [`spec.allowedIdentities`](/modules/user-authn/cr.html#dexprovider-v1-spec-allowedidentities) (email, домены email, группы) и собственные фильтры групп провайдера ограничивают это множество, и учитываются только роли, уже выданные внутри него. Провайдер без ограничителя по email способен утвердить любой адрес, включая те, у которых есть `SuperAdmin`, поэтому его запись требует `SuperAdmin`. Базовая аутентификация провайдера OIDC или Crowd выполняется без участия Dex и его ограничителей, поэтому такой провайдер считается провайдером без ограничителя по email, а OIDC-провайдер — ещё и без ограничителя по группам; у Crowd-провайдера ось групп ограничивает только `crowd.groups`. Подробности и список изменений, проходящих без проверки, — в [документации модуля `user-authn`](/modules/user-authn/usage.html).

У ClusterAuthorizationRule поле `spec.accessLevel` задаёт уровень [упрощённой ролевой модели](rbac-current.html): `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin`, `SuperAdmin`. Менеджер подсистемы `iam` и обладатель `d8:system:manager` назначают уровни, которые включает `ClusterEditor`: `User`, `PrivilegedUser`, `Editor` и `ClusterEditor`, но не `Admin`, `ClusterAdmin` и `SuperAdmin`, а также не могут назначить кластерную роль Kubernetes `cluster-admin`.

ClusterAuthorizationRule, которое открывает системные неймспейсы, считается назначением уровня `ClusterAdmin`, каким бы ни был его `accessLevel`, поэтому эти менеджеры не могут его записать. Такое правило — это правило с `allowAccessToSystemNamespaces: true`, с `namespaceSelector.matchAny`, с `namespaceSelector.labelSelector`, который не помечает свои неймспейсы лейблом `kubernetes.io/metadata.name`, правило для субъекта, которого туда уже пускает другое правило (правила субъекта складываются), или любое правило при выключенном режиме мультитенантности. Поэтому без режима мультитенантности, как в DP Open/CE, эти менеджеры вообще не могут записывать ClusterAuthorizationRule.

Правила запроса складываются: правила его пользователя, каждой группы из его токена и его ServiceAccount. Поэтому правило открывает системные неймспейсы и тогда, когда другое правило уже пускает туда один из запросов, к которым оно применяется:

- локального пользователя, которого правило называет, или участника его групп на любой глубине вложенности — вместе с группами, в которых этот пользователь состоит, и группами над ними;
- самого запрашивающего — с группами его токена;
- любой запрос — через `system:authenticated` (ServiceAccount — ещё и через `system:serviceaccounts` и `system:serviceaccounts:<неймспейс>`).

Правило для `system:authenticated` применяется к любому запросу, поэтому открывает системные неймспейсы, как только их открывает любое другое правило. Правило для `system:serviceaccounts` открывает их, как только другое правило пускает туда какой-либо ServiceAccount, а правило для `system:serviceaccounts:<неймспейс>` — как только другое правило пускает туда ServiceAccount этого неймспейса. Субъект User с именем `system:serviceaccount:<неймспейс>:<имя>` — это тот же ServiceAccount.

Участник Group получает права всех групп, в которые она входит, включая группы на более высоких уровнях вложенности. Поэтому при изменении состава Group проверяются права на назначение ролей, получаемых через эти группы. По той же причине эти менеджеры не могут добавлять участников в Group, для которой или для одной из вышестоящих групп настроен ClusterAuthorizationRule, если другое правило уже предоставляет этим участникам доступ к системным неймспейсам. Группы, которые провайдер идентификации передаёт для других пользователей, вебхуку неизвестны и не учитываются.

По той же причине пользователь или группа, которых называет такое правило, считаются целью уровня `ClusterAdmin`: эти менеджеры не могут создать User на этот email, добавить участников в такую Group, выполнить UserOperation для такого пользователя или подключить DexProvider, который может утвердить такую идентичность. Выключение режима мультитенантности в ModuleConfig `user-authz` открывает системные неймспейсы всем правилам и тоже требует диапазона `ClusterAdmin`. В гранулярной модели менеджер IAM назначает роли подсистемы `iam` не выше `d8:subsystem:iam:manager`, а системный менеджер — роли любой области не выше уровня `manager`.

Эти менеджеры не записывают вебхуки и политики допуска (ValidatingWebhookConfiguration, MutatingWebhookConfiguration, ValidatingAdmissionPolicy и ValidatingAdmissionPolicyBinding): это делают суперадминистраторы подсистем `security` и `cluster` и `d8:system:superadmin`. Мутирующий вебхук может изменить любой объект, который создаёт платформа, включая её собственные поды, а удаление вебхука или политики отключает то, что они проверяют, в том числе ограничения выше. Объекты ValidatingWebhookConfiguration `d8-deckhouse-validating-webhook-handler-*`, в которых собраны вебхуки модулей Deckhouse, создают, изменяют и удаляют только `webhook-handler`, компоненты платформы и суперадминистраторы, в том числе обладатели роли со всеми глаголами на все ресурсы.

Запрашивающий, у которого есть все права во всём кластере, в этих проверках приравнивается к `SuperAdmin` и может назначить любую роль, в том числе `SuperAdmin` и `cluster-admin`. Например, такие права есть у субъекта кластерной роли Kubernetes `cluster-admin`, если роль выдана через ClusterRoleBinding или через ClusterAuthorizationRule без `namespaceSelector` и `limitNamespaces`. Если включён режим мультитенантности, такое правило даёт доступ к системным неймспейсам только с `namespaceSelector.matchAny: true` (или устаревшим `allowAccessToSystemNamespaces: true`).

Права из ClusterAuthorizationRule с `namespaceSelector` или `limitNamespaces` в эту проверку не входят, даже если правило выдаёт `cluster-admin`. Любой непустой `limitNamespaces` намеренно считается ограничением, даже если его записи подходят под все неймспейсы, например `.*` вместе с `allowAccessToSystemNamespaces: true`. По `accessLevel` такое правило учитывается как обычно. Например, `accessLevel: SuperAdmin` даёт диапазон `SuperAdmin` независимо от ограничений неймспейсов в правиле.

`limitNamespaces` и `namespaceSelector` ограничивают только доступ к ресурсам в неймспейсах. Субъект с уровнем доступа `ClusterAdmin` или `SuperAdmin` по-прежнему может создавать объекты ClusterRoleBinding и ClusterAuthorizationRule, поэтому эти поля не ограничивают то, что он может выдать на весь кластер. Выдавайте эти уровни только тем, кому можно иметь доступ ко всему кластеру. ClusterAuthorizationRule учитывается только через биндинги, которые для него создал `user-authz-controller`.

В тексте отказа вебхука уровни `accessLevel` называются basic, чтобы не путать их с уровнями гранулярных ролей (`viewer`, `manager`, `superadmin`).

Прав модуля `user-authn` на создание User или Group для этого недостаточно: они не дают ролей, уже выданных на этот email.

## Глобальные справочники ресурсов

Помимо ролей и capability гранулярной модели, модуль создаёт специальную ClusterRole `d8:dict`. Она даёт доступ для чтения кластерных «справочных» ресурсов, которые пользователям часто нужно просматривать при создании объектов. Например, пользователю, создающему PersistentVolumeClaim, нужно видеть доступные StorageClass, а создающему Ingress — IngressClass.

Роль даёт права `get`, `list`, `watch` на следующие ресурсы:

- `storageclasses` (storage.k8s.io), а также `csidrivers`, `csinodes`, `volumeattachments`;
- `volumesnapshotclasses` (snapshot.storage.k8s.io);
- `ingressclasses` (networking.k8s.io);
- `priorityclasses` (scheduling.k8s.io);
- `runtimeclasses` (node.k8s.io);
- `virtualmachineclasses`, `clustervirtualimages` (virtualization.deckhouse.io);
- `clusterlogdestinations` (deckhouse.io);
- `customresourcedefinitions` (apiextensions.k8s.io) — только `get`, `list`.

### Автоматическая привязка

`user-authz-controller` поддерживает один ClusterRoleBinding на `d8:dict` для каждого субъекта, у которого есть один из следующих RoleBinding:

- RoleBinding на роль `d8:namespace:*` или `d8:project:*`, включая RoleBinding, которые `multitenancy-manager` создаёт из ProjectRoleBinding и ClusterProjectRoleBinding;
- RoleBinding, который модуль создаёт для AuthorizationRule (уровни `User`, `PrivilegedUser`, `Editor` и `Admin`).

Пользователи `system:multitenancy-manager:*`, от имени которых действует `multitenancy-manager`, привязку не получают. Поэтому пользователи [проекта](../../../../glossary.html#project) могут просматривать справочные ресурсы без какой-либо ручной настройки. Привязка удаляется, когда у субъекта не остаётся ни одного такого RoleBinding. Создавать её вручную не нужно.

{% alert level="info" %}
Роль `d8:dict` независима от механизма управления доступом к cluster-wide-ресурсам `multitenancy-manager`: она предоставляет права **только на чтение**
справочных ресурсов для их обнаружения. Механизм контролирует, **какие значения ресурсов**
проект может фактически использовать при создании объектов.
{% endalert %}

## Подсистемы ролевой модели

Каждый модуль DP принадлежит определённой [подсистеме](../../../../glossary.html#subsystem). Для каждой подсистемы существует набор ролей с разными уровнями доступа. Роли обновляются автоматически при включении или отключении модуля.

В платформе есть подсистемы `iam`, `security`, `cluster`, `delivery`, `network`, `storage`, `observability` и `managed-services`.

Например, для подсистемы `network` существуют следующие подсистемные роли, которые можно использовать в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/):

- `d8:subsystem:network:viewer`;
- `d8:subsystem:network:manager`;
- `d8:subsystem:network:superadmin`.

Область действия роли зависит от того, к какой подсистеме она принадлежит:

- Область действия ролей `d8:system:*` — неймспейсы всех включённых модулей DP, то есть неймспейсы, указанные в лейбле `rbac.deckhouse.io/namespace` их capabilities.
- Область действия ролей подсистем — неймспейсы, в которых работают модули подсистемы (подробнее — в таблице состава подсистем ниже), а также все cluster-wide-объекты модулей подсистемы.

Таблица состава подсистем [ролевой модели](../../../../glossary.html#rbac).

{% include rbac/rbac-subsystems-list.liquid %}

## Как устроены роли: агрегация и capabilities

Ни одна встроенная роль не содержит списка прав напрямую. Права описываются в отдельных небольших кластерных ролях — **capabilities**. Каждая capability отвечает за один вид действий (например, «просмотр логов», «управление квотами», «подключение к подам») и содержит конкретные RBAC-правила. Роль (`d8:namespace:admin`, `d8:system:viewer` и т. д.) — это пустая ClusterRole с правилом агрегации (`aggregationRule`): Kubernetes автоматически собирает в неё правила из всех capabilities с подходящими лейблами.

Такое устройство даёт два практических следствия:

- Модули DP расширяют роли автоматически: при включении модуля его capabilities добавляются в соответствующие встроенные роли, при выключении — удаляются. Список прав роли всегда соответствует набору включённых модулей.
- Вы можете собирать собственные роли из готовых capabilities, не описывая RBAC-правила вручную — как показано ниже.

Имена встроенных ролей и capabilities начинаются с префикса `d8:`. Этот префикс зарезервирован: создать собственную ClusterRole с именем `d8:*` нельзя — исключение составляет только префикс `d8:custom:*`, выделенный для пользовательских ролей и capabilities.

## Создание новой роли подсистемы

Если, например, текущие подсистемы не подходят под ролевое распределение в компании, требуется создать новую [подсистему](#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `cluster`, подсистемы `network` и модуля `user-authn`.

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

- показывает, какую namespace-роль получают субъекты роли в неймспейсах модулей (объекты RoleBinding создаёт `user-authz-controller`); собственная роль проецируется не выше `viewer`:

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

- показывает, что роль является ролью [подсистемы](../../../../glossary.html#subsystem), и обрабатываться будет соответственно:

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

{% alert level="info" %}

Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей»](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

{% endalert %}

Таким образом роль получает права от подсистем `cluster`, `network` и от модуля `user-authn`.

Особенности использования кастомных ролей:

- кастомные роли и capabilities должны иметь префикс имени `d8:custom:` («остальные имена с префиксом `d8:` зарезервированы за встроенными объектами DP). Имя должно согласовываться с объявленной областью: подсистемная роль — `d8:custom:<подсистема>:<имя>` (сегмент — сама подсистема, как в примере выше), namespace- или проектная роль — `d8:custom:namespace:<имя>` и `d8:custom:project:<имя>`, capability — `d8:custom:<область>-capability:<имя>`. Имя, расходящееся с лейблом `rbac.deckhouse.io/scope`, будет отклонено;
- RoleBinding с namespace-ролью (`d8:namespace:<уровень>`) будут созданы в неймспейсах модулей агрегированных подсистем, уровень задаётся лейблом `rbac.deckhouse.io/use-role`, для собственной роли — не выше `viewer`. Без лейбла такие RoleBinding не создаются;
- собственную роль нельзя агрегировать в `d8:system:<уровень>`, потому что лейбл `rbac.deckhouse.io/aggregate-to-system-as` может нести только собственная capability. Чтобы выдать роль обладателям `d8:system:manager`, привяжите её к ним через ClusterRoleBinding.

## Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для подсистемной роли) CRD-объект — MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом. Выполните следующие шаги:

1. Дополните роль новым селектором:

   ```yaml
   rbac.deckhouse.io/aggregate-to-mycustom-as: manager
   ```

   Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей»](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

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

1. Создайте новую capability, в которой определите права для нового ресурса. Например, только чтение:

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

   Capability добавит свои права в роль [подсистемы](../../../../glossary.html#subsystem), дав права на просмотр нового объекта.

Особенности:

- называйте собственную capability `d8:custom:<ОБЛАСТЬ>-capability:<ИМЯ>`, где `<ОБЛАСТЬ>` — значение её лейбла `rbac.deckhouse.io/scope`. Имя, которое не соответствует лейблу, отклоняется.

## Расширение существующих подсистемных ролей

Если необходимо расширить существующую роль, выполните те же шаги, что и в пункте выше, но измените лейблы и название роли. Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей»](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

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

## Расширение подсистемных ролей с добавлением нового неймспейса

Неймспейсы, в которых обладатели системной или подсистемной роли получают RoleBinding с namespace-ролью, — это неймспейсы её модулей. `user-authz-controller` берёт их из лейбла `rbac.deckhouse.io/namespace` системных capabilities, которые поставляет Deckhouse, с именем на `d8:` и лейблом `heritage: deckhouse`. На собственной capability лейбл игнорируется: она добавляет свои правила ролям, которые её агрегируют, но не добавляет неймспейс.

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

## Расширение существующих namespace-ролей

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

Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей»](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

## Создание собственной namespace- или проектной роли

Иногда встроенная иерархия уровней не подходит: например, нужна роль «разработчик» — просмотр всего неймспейса плюс доступ к терминалу подов (`exec`, `attach`), но без права менять квоты или RBAC. Такая роль собирается из готовых capabilities, без написания RBAC-правил вручную.

Правила для собственных ролей:

- имя должно начинаться с `d8:custom:` (например, `d8:custom:namespace:developer`);
- роль должна иметь лейбл `rbac.deckhouse.io/kind: custom-role`;
- namespace- или проектная роль, которую будут выдавать через RoleBinding, должна также иметь `rbac.deckhouse.io/delegatable: "true"`. Каждый [пользовательский неймспейс](../../../../glossary.html#user-namespace) — это [проект](../../../../glossary.html#project), и RoleBinding в нём принимают только роли с этим лейблом. На системные и [подсистемные](../../../../glossary.html#subsystem) роли лейбл ставить нельзя — вебхук отклонит такую роль;
- роль **не может содержать собственных правил** (`rules`) — только агрегировать capabilities через `aggregationRule`. Права описываются в отдельных capabilities — так состав роли всегда прозрачен;
- нельзя в одной роли агрегировать capabilities пользовательских областей (`namespace`, `project`) вместе с административными (`system`, подсистемы) — такая роль будет отклонена.

Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей»](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

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

Собрать такую роль можно и без YAML — с помощью мастера выдачи доступа в [веб-интерфейсе Deckhouse](../../../../glossary.html#web-ui): он показывает доступные capabilities, собирает из них роль и сразу создаёт нужную привязку.

{% endalert %}

## Ограничения уровня admin и права superadmin

В [ролевой модели](../../../../glossary.html#rbac) предусмотрено два уровня администрирования:

- `admin` — повседневный администратор. Управляет ресурсами, квотами и доступом в своей области, но не может выполнять операции, которые позволяют выйти за её пределы или нарушить работу компонентов платформы.
- `superadmin` — «аварийный» администратор. Обладает всеми правами `admin` и дополнительно может выполнять опасные операции. Выдавайте этот уровень осознанно и только тем, кому он действительно необходим.

Что запрещено уровню `admin` и разрешено только уровню `superadmin`:

- **Выпуск токенов ServiceAccount'ов** (`kubectl create token`) **и выполнение запросов от имени ServiceAccount** (`kubectl --as system:serviceaccount:...`). Токен ServiceAccount'а — это готовая учётная запись: завладев токеном служебного аккаунта платформенного компонента, можно получить его права далеко за пределами неймспейса. Поэтому `admin` управляет самими объектами `ServiceAccount` (создание, удаление), но не может выпускать для них токены и действовать от их имени.
- **Изменение и удаление системных ресурсов в [пользовательских неймспейсах](../../../../glossary.html#user-namespace).** Некоторые компоненты платформы размещают свои объекты (например, поды Dex-аутентификатора или поды и диски виртуальных машин) прямо в неймспейсах приложений. Такие объекты помечены лейблом `deckhouse.io/system-resource: "true"`. Изменять и удалять их может только `superadmin`, для остальных такие операции отклоняются на уровне API-сервера с пояснением.
- **Подключение к системным подам** — `kubectl exec`, `kubectl attach` и `kubectl port-forward` в под с лейблом `deckhouse.io/system-resource: "true"` доступны только `superadmin`.

Перечисленные ограничения не действуют на администраторов кластера: у тех, кому выдана стандартная роль Kubernetes `cluster-admin` или уровень доступа `SuperAdmin` [упрощённой модели](rbac-current.html), а также у участников групп `system:masters`, `kubeadm:cluster-admins`, `superadmins` и `system:sudousers`, аварийный доступ сохраняется независимо от этих ограничений.

Для `superadmin` также есть ограничения: ресурсы, созданные из [шаблона проекта](/modules/multitenancy-manager/) (лейбл `heritage: multitenancy-manager`), не может изменить **никто**, включая `superadmin` и администратора кластера, — они управляются исключительно контроллером [проектов](../../../../glossary.html#project). Роль назначается через RoleBinding и действует только в том неймспейсе, где выдана: `superadmin` одного неймспейса не получает никаких особых прав в другом.

## Встроенные механизмы защиты ролевой модели

Ролевая модель защищена набором проверок на уровне API-сервера. Они не требуют настройки и предотвращают типовые ошибки и попытки повышения привилегий:

- **Нельзя выдать ограниченную по области роль на весь кластер.** ClusterRoleBinding на роли `d8:namespace:*`, `d8:project:*` (и их `d8:custom:*`-варианты) отклоняется — иначе роль, рассчитанная на один неймспейс или проект, действовала бы во всех неймспейсах сразу. Используйте RoleBinding в нужном неймспейсе либо ProjectRoleBinding/ClusterProjectRoleBinding для проекта.
- **Нельзя выдать capability на весь кластер.** ClusterRoleBinding на любую capability отклоняется: capability — «строительный блок» для ролей, а не самостоятельная роль. В отдельном неймспейсе привязать capability через RoleBinding можно.
- **Нельзя получить управление проектами через собственную роль.** Создание Role или ClusterRole, дающей права на изменение ресурсов управления проектами (`projects`, `projecttemplates`, `projectrolebindings`, `clusterprojectrolebindings`, `projectnamespaces`), отклоняется — эти права дают только встроенные роли `d8:project:*`.
- **Namespace- или проектная роль не может включать capabilities платформы.** Собственная роль с `scope: namespace`, `scope: project` или без области не может выбирать capabilities систем и подсистем по лейблу `rbac.deckhouse.io/capability`, а также область `system` или [подсистемы](../../../../glossary.html#subsystem) по лейблу `rbac.deckhouse.io/aggregate-to-<область>-as`. Ни одна собственная роль не может совмещать capabilities областей `namespace`/`project` и областей `system`/`subsystem`.
- **Собственная capability содержит правила и не может иметь `aggregationRule`.**
- **Имя должно соответствовать области**: `d8:custom:namespace:<имя>`, `d8:custom:project:<имя>`, `d8:custom:system:<имя>`, `d8:custom:<подсистема>:<имя>`, а для capability — `d8:custom:<область>-capability:<имя>`.
- **Лейбл `rbac.deckhouse.io/aggregate-to-<область>-as` области платформы может нести только собственная capability.**
- **Лейбл `rbac.deckhouse.io/delegatable: "true"` допускается только на роли, которая агрегирует только capabilities неймспейса и проекта.**
- **Собственные роли не могут содержать прямых RBAC-правил** — только агрегировать capabilities.
- **Лейблы `rbac.deckhouse.io/kind: role` и `rbac.deckhouse.io/kind: capability` зарезервированы за встроенными объектами DP.** ClusterRole с одним из этих лейблов отклоняется. Для собственных объектов используйте `custom-role` и `custom-capability`.

## Отображаемые названия ролей

Каждая встроенная роль и capability имеет локализованные название и описание в аннотациях:

- `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` — на русском;
- `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` — на английском.

Эти аннотации использует, например, веб-интерфейс Deckhouse при отображении списка ролей.

Если стандартное название не подходит (например, вы хотите называть роли в терминах, принятых в компании), добавьте на роль аннотации `custom.meta.deckhouse.io/title` и `custom.meta.deckhouse.io/description` — интерфейс покажет их вместо стандартных. Пример:

```shell
d8 k annotate clusterrole d8:namespace:admin custom.meta.deckhouse.io/title='Администратор команды'
```

Это единственное разрешённое изменение встроенных ролей: изменение их правил, агрегации или лейблов отклоняется.

## Устаревшие имена ролей

Прежние имена ролей гранулярной модели (`d8:manage:<подсистема>:<уровень>`, `d8:manage:all:<уровень>` и `d8:use:role:<уровень>`) устарели и будут удалены в следующем релизе. Для обратной совместимости они временно сохранены как роли-псевдонимы. Привязка к псевдониму агрегирует capabilities **новой** роли, кроме случаев, когда изменился список подсистем (подробнее [ниже](#список-подсистем-у-псевдонимов)).

Права псевдонима не совпадают с правами до миграции. Устаревшая роль `d8:use:role:admin` больше не даёт выпуск токена ServiceAccount и имперсонацию, это перешло на уровень `superadmin`. Псевдонимы `d8:manage:*` не `delegatable`. Псевдонимы `d8:use:role:*` `delegatable`, поэтому RoleBinding на них в неймспейсе проекта продолжает работать, но новые ProjectRoleBinding и ClusterProjectRoleBinding на старые имена отклоняются.

Соответствие имён:

| Устаревшее имя | Новое имя |
|-----------------|-----------|
| `d8:manage:all:<уровень>` | `d8:system:<уровень>` |
| `d8:manage:deckhouse:<уровень>`, `d8:manage:infrastructure:<уровень>`, `d8:manage:kubernetes:<уровень>` | `d8:subsystem:cluster:<уровень>`, которая даёт больше прав. Права на модули, перешедшие в другие подсистемы, дают роли этих подсистем |
| `d8:manage:networking:<уровень>` | `d8:subsystem:network:<уровень>`, которая даёт и права на `keepalived` |
| `d8:manage:security:<уровень>` | `d8:subsystem:security:<уровень>` вместе с `d8:subsystem:iam:<уровень>`, которая даёт права на `user-authz` и `user-authn`, а также другие права |
| `d8:manage:observability:<уровень>` | `d8:subsystem:observability:<уровень>` |
| `d8:manage:storage:<уровень>` | `d8:subsystem:storage:<уровень>`, которая даёт и права на `local-path-provisioner` |
| `d8:use:role:<уровень>` | `d8:namespace:<уровень>` |
| `d8:use:role:<уровень>:kubernetes` | `d8:namespace:<уровень>` (полная namespace-роль, не kubernetes-only) |

<div style="height: 0;" id="список-подсистем-у-псевдонимов"></div>

Вместе с именами ролей изменился список подсистем. `deckhouse`, `infrastructure` и `kubernetes` объединены в `cluster`, куда перешли и модули из других подсистем, `networking` переименована в `network`, а `user-authz`, `user-authn` и `multitenancy-manager` перешли в `iam`.

Поэтому псевдонимы ролей `deckhouse`, `infrastructure` и `kubernetes` не агрегируют всю подсистему `cluster`. Они выбирают capabilities модулей платформы, которые были в старой подсистеме, где бы эти модули ни находились теперь. `d8:subsystem:cluster:<уровень>` даёт больше прав, поэтому, пересоздавая такую привязку, проверьте, какие роли подсистем нужны её субъектам.

Псевдоним `d8:manage:security:<уровень>` сохраняет и права на `user-authz` и `user-authn`, которые теперь даёт `d8:subsystem:iam:<уровень>`. Эта роль даёт также права на `multitenancy-manager` и на уровне `manager` право назначать роли, которого `d8:subsystem:security:<уровень>` не даёт. Поэтому привязку к `d8:manage:security:<уровень>` заменяют две привязки с теми же субъектами, к `d8:subsystem:iam:<уровень>` и к `d8:subsystem:security:<уровень>`. Аннотация `rbac.deckhouse.io/deprecated-replaced-by` псевдонима называет только вторую.

Псевдонимы `networking` и `storage` не включают `keepalived` и `local-path-provisioner`, которые перешли в `network` и `storage` из `infrastructure`, поэтому новые роли дают больше прав, чем эти псевдонимы.

Аннотация `rbac.deckhouse.io/deprecated-replaced-by` псевдонима называет основную замену, а описание псевдонима называет роли других подсистем, в которые перешли его модули. Эти роли охватывают подсистемы целиком. Чтобы выдать права только на некоторые модули, создайте собственную роль, которая выбирает их системные capabilities по лейблу `rbac.deckhouse.io/capability`, например `system-capability.multitenancy-manager.view` (см. [«Создание новой роли подсистемы»](#создание-новой-роли-подсистемы)).

Псевдонимы ролей `deckhouse`, `infrastructure`, `kubernetes` и `networking` и `d8:manage:all:<уровень>` выбирают также лейблы этих старых подсистем, например `rbac.deckhouse.io/aggregate-to-deckhouse-as`. Их ещё могут нести модули с собственным циклом выпуска и кастомные capabilities. Ни одна роль новой модели эти лейблы не собирает, поэтому такие capabilities получают только обладатели старых имён, а алерт `D8UserAuthzRetiredAggregationLabel` называет их.

Пока в кластере остаются привязки к устаревшим именам, срабатывают алерты `D8UserAuthzDeprecatedRBACv2RoleInUse` (привязка к роли-псевдониму) и `D8UserAuthzDeprecatedRBACv2CapabilityInUse` (привязка к устаревшему имени, которого нет ни у одной ClusterRole, например к capability прошлого релиза, поэтому привязка не даёт прав). Релиз, который удаляет псевдонимы, несёт требование релиза `deprecatedRBACv2BindingsCount`: обновление остаётся в ожидании, а список привязок выводится в статусе DeckhouseRelease, пока каждая привязка к роли-псевдониму не пересоздана на новое имя роли. Привязка, которая не даёт прав, релиз не держит.

Переведите существующие RoleBinding и ClusterRoleBinding на новые имена ролей:

- Объект ClusterRoleBinding с именем `user-authz:<правило>:additional-role:<роль>` создаёт `user-authz-controller` по полю `spec.additionalRoles` правила ClusterAuthorizationRule `<правило>` и восстанавливает его после удаления. Меняйте не привязку, а имя роли в `spec.additionalRoles` этого правила.
- Привязку, которую создаёт шаблон [проекта](../../../../glossary.html#project), меняйте в ProjectTemplate, потому что `multitenancy-manager` создаёт её снова по шаблону.

Найти привязки, использующие устаревшие имена, без RoleBinding, которые для них поддерживает `user-authz-controller`, можно с помощью команды:

```bash
d8 k get clusterrolebindings,rolebindings -A -o json \
  | jq -r '.items[] | select(.roleRef.name | test("^d8:(manage|use):")) | select(.metadata.labels["rbac.deckhouse.io/automated"] != "true") | "\(.kind) \(.metadata.namespace // "-") \(.metadata.name) -> \(.roleRef.name)"'
```

### Собственные роли прежней схемы

{% alert level="warning" %}
У встроенных capabilities больше нет лейблов `rbac.deckhouse.io/kind: manage` и `rbac.deckhouse.io/kind: use`. Собственная роль прежней схемы, которая выбирает capabilities по этим лейблам, потеряла их права, а вместе с ней и её субъекты. Псевдонимы совместимости для собственных ролей не создаются.
{% endalert %}

Такие роли перечисляет алерт `D8UserAuthzLegacyRBACv2CustomRoleFound`. Как найти и исправить такие роли и как прежние лейблы соответствуют новым, описано в [разделе FAQ модуля `user-authz` о переводе кастомных ролей на новую схему](/modules/user-authz/faq.html#как-перевести-кастомные-роли-на-новую-схему).

## Получение аналога ролей ClusterAdmin и SuperAdmin упрощённой модели

В гранулярной модели нет ролей, объединяющих права в одну сущность, как [`ClusterAdmin` и `SuperAdmin`](rbac-current.html) в упрощённой модели. В ней разделяется управление платформой (системные роли) и доступ к приложениям (namespace- и проектные роли). Аналог собирается из **двух привязок**: ClusterRoleBinding на системную роль и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) на проектную роль (она действует во всех проектах, включая создаваемые позже).

Примерное соответствие ролей:

| Роль упрощённой модели | Аналог в гранулярной модели |
|--------------------------|-------------------------------|
| `User` | `d8:namespace:viewer` (через RoleBinding или ProjectRoleBinding) |
| `PrivilegedUser` | `d8:namespace:user` |
| `Editor` | `d8:namespace:manager` |
| `Admin` | `d8:namespace:admin` |
| `ClusterEditor` | Прямого аналога нет: соберите его из ClusterProjectRoleBinding на `d8:project:manager` (все проекты) и системной роли для платформенной части |
| `ClusterAdmin` | `d8:system:manager` + ClusterProjectRoleBinding на `d8:project:admin` |
| `SuperAdmin` | `d8:system:superadmin` + ClusterProjectRoleBinding на `d8:project:superadmin` |

Пример для `ClusterAdmin` (группа `k8s-admins`):

```yaml
# Платформа: конфигурация модулей DP, cluster-wide-ресурсы, просмотр неймспейсов модулей DP.
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

{% alert level="info" %}
Каждый [пользовательский неймспейс](../../../../glossary.html#user-namespace) является [проектом](../../../../glossary.html#project): неймспейс, созданный напрямую, [автоматически становится проектом](/modules/multitenancy-manager/#автоматическое-создание-проектов-для-неймспейсов). Поэтому пара «системная роль + ClusterProjectRoleBinding» покрывает и платформу, и все пользовательские неймспейсы. Неймспейс `default` эта пара не покрывает: это [системный неймспейс](../../../../glossary.html#system-namespace), который относится к виртуальному проекту `default`, ClusterProjectRoleBinding на виртуальные проекты не распространяется, а модули DP в нём не работают.

Создать собственную роль «со всеми правами» (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`) нельзя — её отклонят [встроенные механизмы защиты](#встроенные-механизмы-защиты-ролевой-модели). Если нужен неограниченный доступ вне [ролевой модели](../../../../glossary.html#rbac) платформы, используйте ClusterRoleBinding на встроенную роль Kubernetes `cluster-admin` — назначить её может только тот, у кого такие права уже есть.
{% endalert %}
