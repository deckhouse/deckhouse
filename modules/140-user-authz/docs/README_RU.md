---
title: "Модуль user-authz"
description: "Авторизация и управление доступом пользователей к ресурсам кластера Deckhouse Kubernetes Platform."
---

Модуль отвечает за генерацию объектов ролевой модели доступа, основанной на базе стандартного механизма RBAC Kubernetes. Модуль создает набор кластерных ролей (`ClusterRole`), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %}
В модуле две ролевые модели: [основная](#основная-ролевая-модель) (используйте её) и [устаревшая](#устаревшая-ролевая-модель), построенная на ресурсах `ClusterAuthorizationRule`/`AuthorizationRule` (поддержка будет прекращена в будущих релизах).

Модели не совместимы по ресурсам — автоматическая конвертация невозможна, — но [могут использоваться одновременно](faq.html#можно-ли-использовать-устаревшую-и-основную-ролевые-модели-одновременно): права из обеих моделей суммируются.
{% endalert %}

<div style="height: 0;" id="новая-ролевая-модель"></div>
<div style="height: 0;" id="экспериментальная-ролевая-модель"></div>

## Основная ролевая модель

В отличие [от устаревшей ролевой модели](#устаревшая-ролевая-модель) DKP, основная ролевая модель не использует ресурсы `ClusterAuthorizationRule` и `AuthorizationRule`. Настройка прав доступа выполняется стандартным для RBAC Kubernetes способом: с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, с указанием в них одной из подготовленных модулем `user-authz` ролей. Для доступа сразу ко всем пространствам имён проекта дополнительно используются ресурсы [ProjectRoleBinding и ClusterProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) модуля `multitenancy-manager`.

> Выдавать доступ можно не только вручную через YAML-манифесты: в веб-интерфейсе Deckhouse Console есть мастер выдачи доступа. Он проводит по шагам (кому выдать доступ → где → с каким уровнем), сам выбирает правильный вид привязки (`RoleBinding`, `ClusterRoleBinding`, `ProjectRoleBinding` или `ClusterProjectRoleBinding`) и позволяет собрать собственную роль из готовых блоков без написания YAML.

Модуль создаёт специальные агрегированные кластерные роли (`ClusterRole`). Используя эти роли в `RoleBinding` или `ClusterRoleBinding` можно решать следующие задачи:

- Управлять доступом к модулям определённой [подсистеме](#подсистемы-ролевой-модели) применения.

  Например, чтобы дать возможность пользователю, выполняющему функции сетевого администратора, настраивать *сетевые* модули (например, `cni-cilium`, `ingress-nginx`, `istio` и т. д.), можно использовать в `ClusterRoleBinding` роль `d8:subsystem:networking:manager`.
- Управлять доступом к *пользовательским* ресурсам модулей в рамках пространства имён.

  Например, использование роли `d8:namespace:manager` в `RoleBinding`, позволит удалять/создавать/редактировать ресурс [PodLoggingConfig](../log-shipper/cr.html#podloggingconfig) в пространстве имён, но не даст доступа к cluster-wide-ресурсам [ClusterLoggingConfig](../log-shipper/cr.html#clusterloggingconfig) и [ClusterLogDestination](../log-shipper/cr.html#clusterlogdestination) модуля `log-shipper`, а также не даст возможность настраивать сам модуль `log-shipper`.

### Области действия ролей

Каждая роль действует в одной из четырёх областей. Область определяет, *где* работают выданные права и *каким ресурсом* роль назначается:

| Область | Формат имени роли | Для кого | Чем назначается |
|---------|-------------------|----------|-----------------|
| Пространство имён | `d8:namespace:<уровень>` | Пользователи приложений (разработчики) | `RoleBinding` в конкретном пространстве имён |
| Проект | `d8:project:<уровень>` | Команды, работающие с [проектами](../multitenancy-manager/) | Только [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) или [ClusterProjectRoleBinding](../multitenancy-manager/cr.html#clusterprojectrolebinding) |
| Подсистема | `d8:subsystem:<подсистема>:<уровень>` | Администраторы части платформы | `ClusterRoleBinding` |
| Вся платформа | `d8:system:<уровень>` | Администраторы платформы | `ClusterRoleBinding` |

Уровни доступа образуют лестницу: каждый следующий уровень включает все права предыдущего.

- Для областей «пространство имён» и «проект» уровней пять: `viewer` → `user` → `manager` → `admin` → `superadmin`.
- Для областей «подсистема» и «вся платформа» уровней три: `viewer` → `manager` → `superadmin`. Уровней `user` и `admin` здесь нет: на системном уровне нет «пользовательских» ресурсов, которыми можно было бы пользоваться, не администрируя их.

Роли, создаваемые модулем, делятся на следующие классы:

- [Namespace-роли](#namespace-роли) — для назначения прав пользователям (например, разработчикам приложений) **в конкретном пространстве имён**.
- [Проектные роли](#проектные-роли) — для назначения прав **сразу во всех пространствах имён проекта**.
- [Системные и подсистемные роли](#системные-и-подсистемные-роли) — для назначения прав администраторам.

{: #rolebinding-car .anchored}

{% alert level="warning" %}
Обратите внимание на особенности настройки комбинированного доступа и совместного использования RoleBinding и ClusterAuthorizationRule (CAR) для одного и того же пользователя.

Если в кластере включён режим мультитенантности (параметр [`enableMultiTenancy: true`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)) и для указанного в RoleBinding пользователя или его группы существует ClusterAuthorizationRule (CAR) с правилами для другого неймспейса, отличного от целевого (указанного в RoleBinding), правила из ClusterRole, указанного в RoleBinding, работать не будут.

Это связано с особенностями работы вебхука модуля `user-authz`. Он проверяет принадлежность запроса к разрешённым неймспейсам на уровне группы. Если группа пользователя привязана к CAR с селектором только на определенный неймспейс, все запросы в неймспейсы, не указанные в CAR, будут отвергнуты, независимо от наличия RoleBinding с этими неймспейсами для пользователя.

Рекомендуется не использовать RoleBinding для пользователя совместно с CAR. Если требуется комбинированный доступ, используйте AuthorizationRule вместо ClusterAuthorizationRule.
{% endalert %}

<div style="height: 0;" id="use-роли"></div>

### Namespace-роли

{% alert level="warning" %}
Namespace-роль можно использовать только в ресурсе `RoleBinding`.
{% endalert %}

Namespace-роли предназначены для назначения прав пользователю **в конкретном пространстве имён**. Под пользователями понимаются, например, разработчики, которые используют настроенный администратором кластер для развёртывания своих приложений. Таким пользователям не нужно управлять модулями DKP или кластером, но им нужно иметь возможность, например, создавать свои Ingress-ресурсы, настраивать аутентификацию приложений и сбор логов с приложений.

Namespace-роль определяет права на доступ к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes (`Pod`, `Deployment`, `Secret`, `ConfigMap` и т. п.).

Модуль создаёт следующие namespace-роли:

- `d8:namespace:viewer` — позволяет в конкретном пространстве имён просматривать стандартные ресурсы Kubernetes (кроме секретов и ресурсов RBAC), журналы подов и метрики, а также выполнять аутентификацию в кластере;
- `d8:namespace:user` — дополнительно к роли `d8:namespace:viewer` позволяет в конкретном пространстве имён просматривать секреты и ресурсы RBAC, подключаться к подам (`kubectl exec`, `kubectl attach`), удалять поды (но не создавать или изменять их), выполнять `kubectl port-forward` и `kubectl proxy`, изменять количество реплик контроллеров;
- `d8:namespace:manager` — дополнительно к роли `d8:namespace:user` позволяет в конкретном пространстве имён управлять ресурсами модулей (например, `Certificate`, `PodLoggingConfig` и т. п.) и стандартными namespaced-ресурсами Kubernetes (`Pod`, `Deployment`, `ConfigMap`, `Secret`, `Service`, `Ingress`, `NetworkPolicy`, `CronJob` и т. п.);
- `d8:namespace:admin` — дополнительно к роли `d8:namespace:manager` позволяет в конкретном пространстве имён управлять ресурсами `ResourceQuota`, `LimitRange`, `ServiceAccount`, `Role`, `RoleBinding`;
- `d8:namespace:superadmin` — дополнительно к роли `d8:namespace:admin` позволяет выполнять опасные с точки зрения безопасности операции: выпускать токены ServiceAccount'ов и выполнять запросы от их имени, а также управлять [системными ресурсами, размещёнными в пространстве имён](#ограничения-уровня-admin-и-права-superadmin) (например, подами Dex или подами/PVC виртуальных машин).

Подробное разделение прав между `admin` и `superadmin` описано [ниже](#ограничения-уровня-admin-и-права-superadmin).

### Проектные роли

{% alert level="warning" %}
Проектную роль нельзя назначить через `ClusterRoleBinding` — попытка будет отклонена. Для назначения роли на весь проект используйте [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) или [ClusterProjectRoleBinding](../multitenancy-manager/cr.html#clusterprojectrolebinding); допускается также обычный `RoleBinding` в одном из пространств имён проекта — тогда роль действует только в нём.
{% endalert %}

Проектные роли (`d8:project:<уровень>`) предназначены для работы с [проектами](../multitenancy-manager/) — изолированными окружениями, которые могут включать несколько пространств имён. Уровни те же, что у namespace-ролей: `viewer`, `user`, `manager`, `admin`, `superadmin`.

Каждая проектная роль включает все права namespace-роли того же уровня и дополнительно даёт права на управление самим проектом:

- `d8:project:viewer` — права `d8:namespace:viewer` плюс просмотр ресурсов [ProjectNamespace](../multitenancy-manager/cr.html#projectnamespace) и [ProjectRoleBinding](../multitenancy-manager/cr.html#projectrolebinding) проекта;
- `d8:project:manager` — права `d8:namespace:manager` плюс управление дополнительными пространствами имён проекта (ресурсы `ProjectNamespace`);
- `d8:project:admin` — права `d8:namespace:admin` плюс управление доступом к проекту (ресурсы `ProjectRoleBinding`) и право привязывать встроенные роли `d8:project:*` и `d8:namespace:*` (кроме уровня `superadmin`) другим пользователям в рамках проекта;
- `d8:project:superadmin` — аналогично соотношению `d8:namespace:superadmin` и `d8:namespace:admin`.

Назначенная через `ProjectRoleBinding` роль автоматически действует во **всех** пространствах имён проекта — как в основном, так и в дополнительных, включая созданные позже.

<div style="height: 0;" id="manage-роли"></div>

### Системные и подсистемные роли

{% alert level="warning" %}
Системные и подсистемные роли не дают доступа к пространству имён пользовательских приложений.

Они определяют доступ только к системным пространствам имён (начинающимся с `d8-` или `kube-`), и только к тем из них, в которых работают модули соответствующей подсистемы роли.
{% endalert %}

Системные (`d8:system:*`) и подсистемные (`d8:subsystem:*`) роли предназначены для назначения прав на управление всей платформой или её частью ([подсистемой](#подсистемы-ролевой-модели)), но не самими приложениями пользователей. С помощью подсистемной роли можно, например, дать возможность администратору безопасности управлять модулями, ответственными за функции безопасности кластера. Тогда администратор безопасности сможет настраивать аутентификацию, авторизацию, политики безопасности и т. п., но не сможет управлять остальными функциями кластера (например, настройками сети и мониторинга) и изменять настройки в пространстве имён приложений пользователей.

Системная/подсистемная роль определяет права на доступ:

- к cluster-wide-ресурсам Kubernetes;
- к управлению модулями DKP (ресурсы `moduleConfig`) в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми модулями DKP для роли `d8:system:*`;
- к управлению cluster-wide-ресурсами модулей DKP в рамках [подсистемы](#подсистемы-ролевой-модели) роли или всеми ресурсами модулей DKP для роли `d8:system:*`;
- к системным пространствам имён (начинающимся с `d8-` или `kube-`), в которых работают модули [подсистемы](#подсистемы-ролевой-модели) роли, или ко всем системным пространствам имён для роли `d8:system:*`.
  
Формат названия системной роли — `d8:system:<ACCESS_LEVEL>`, подсистемной — `d8:subsystem:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:

- `SUBSYSTEM` — подсистема роли ([список подсистем](#подсистемы-ролевой-модели));
- `ACCESS_LEVEL` — уровень доступа.

  Примеры ролей:
  
  - `d8:system:viewer` — доступ на просмотр конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:system:manager` — аналогично роли `d8:system:viewer`, только доступ на уровне `admin`, т. е. просмотр/создание/изменение/удаление конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:subsystem:observability:viewer` — доступ на просмотр конфигурации модулей DKP (ресурсы `moduleConfig`) из подсистемы `observability`, их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в системных пространствах имён `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

Модуль предоставляет три уровня доступа для администратора:

- `viewer` — позволяет просматривать стандартные ресурсы Kubernetes, конфигурацию модулей (ресурсы `moduleConfig`), cluster-wide-ресурсы модулей и namespaced-ресурсы модулей в пространстве имен модуля;
- `manager` — дополнительно к уровню `viewer` позволяет управлять стандартными ресурсами Kubernetes, конфигурацией модулей (ресурсы `moduleConfig`), cluster-wide-ресурсами модулей и namespaced-ресурсами модулей в пространстве имен модуля;
- `superadmin` — дополнительно к уровню `manager` позволяет управлять системными ресурсами модулей подсистемы;

### Подсистемы ролевой модели

Каждый модуль DKP принадлежит определённой подсистеме. Для каждой подсистемы существует набор ролей с разными уровнями доступа. Роли обновляются автоматически при включении или отключении модуля.

Например, для подсистемы `networking` существуют следующие подсистемные роли, которые можно использовать в `ClusterRoleBinding`:

- `d8:subsystem:networking:viewer`
- `d8:subsystem:networking:manager`
- `d8:subsystem:networking:superadmin`

Область действия роли зависит от того, к какой подсистеме она принадлежит:

- Область действия ролей `d8:system:*` — все системные (начинающиеся с `d8-` или `kube-`) неймспейсы кластера.
- Область действия ролей подсистем — неймспейсы, в которых работают модули подсистемы (подробнее — в таблице состава подсистем), а также все cluster-wide объекты модулей подсистемы.

Таблица состава подсистем ролевой модели.

{% include rbac/rbac-subsystems-list.liquid %}

### Как устроены роли: агрегация и capabilities

Ни одна встроенная роль не содержит списка прав напрямую. Права описываются в отдельных небольших кластерных ролях — **capabilities**. Каждая capability отвечает за один вид действий (например, «просмотр логов», «управление квотами», «подключение к подам») и содержит конкретные RBAC-правила. Роль (`d8:namespace:admin`, `d8:system:viewer` и т. д.) — это пустая `ClusterRole` с правилом агрегации (`aggregationRule`): Kubernetes автоматически собирает в неё правила из всех capabilities с подходящими лейблами.

Принадлежность объектов к ролевой модели задаётся лейблами `rbac.deckhouse.io/*` — например, лейбл `rbac.deckhouse.io/aggregate-to-namespace-as: admin` включает capability в роль `d8:namespace:admin` (и, за счёт лестницы уровней, во все уровни выше). Полный перечень лейблов и аннотаций — в [справочнике ниже](#справочник-лейблов-и-аннотаций-ролей).

Такое устройство даёт два практических следствия:

- Модули DKP расширяют роли автоматически: при включении модуля его capabilities добавляются в соответствующие встроенные роли, при выключении — удаляются. Список прав роли всегда соответствует набору включённых модулей.
- Вы можете собирать собственные роли из готовых capabilities, не описывая RBAC-правила вручную. Как это сделать — [в FAQ](faq.html#как-расширить-роли-или-создать-новую).

Имена встроенных ролей и capabilities начинаются с префикса `d8:`. Это пространство имён зарезервировано: создать собственную `ClusterRole` с именем `d8:*` нельзя — исключение составляет только префикс `d8:custom:*`, выделенный для пользовательских ролей и capabilities. Лейблы `rbac.deckhouse.io/kind: role` и `rbac.deckhouse.io/kind: capability` также зарезервированы за встроенными объектами — для собственных используйте `custom-role` и `custom-capability`.

### Справочник лейблов и аннотаций ролей

Все лейблы, которые ролевая модель использует на объектах `ClusterRole`:

| Лейбл | Где встречается | Назначение |
|-------|-----------------|------------|
| `rbac.deckhouse.io/kind` | Все объекты ролевой модели | Тип объекта: `role` или `capability` — встроенные (зарезервированы), `custom-role` или `custom-capability` — пользовательские. Объекты без этого лейбла ролевой моделью не обрабатываются |
| `rbac.deckhouse.io/scope` | Роли и capabilities | Область действия: `namespace`, `project`, `subsystem`, `system` |
| `rbac.deckhouse.io/subsystem` | Подсистемные объекты | Имя подсистемы (например, `networking`) — только при `scope: subsystem` |
| `rbac.deckhouse.io/aggregate-to-<область>-as: <уровень>` | Capabilities и роли младших уровней | Правило агрегации: включает объект в роль указанной области и уровня. `<область>` — `system`, `namespace`, `project` или имя подсистемы; `<уровень>` — `viewer`, `user`, `manager`, `admin`, `superadmin`. Именно эти лейблы указываются в селекторах `aggregationRule` |
| `rbac.deckhouse.io/capability` | Capabilities | Глобально уникальное имя capability (например, `namespace-capability.kubernetes.view_logs`) — по нему capability адресно включается в [собственную роль](faq.html#создание-собственной-namespace--или-проектной-роли) |
| `rbac.deckhouse.io/use-role: <уровень>` | Системные и подсистемные роли | Какой уровень namespace-роли автоматически выдаётся обладателю этой роли в системных пространствах имён её подсистемы. У встроенных ролей: `viewer` → `viewer`, `manager` → `admin`, `superadmin` → `superadmin`. Выдача выполняется автоматически создаваемыми `RoleBinding` (лейбл `rbac.deckhouse.io/automated: "true"`) |
| `rbac.deckhouse.io/namespace: <пространство имён>` | Capabilities | Дополнительное пространство имён, в котором обладателям системной/подсистемной роли будет автоматически создан `RoleBinding` ([пример в FAQ](faq.html#расширение-подсистемных-ролей-с-добавлением-нового-пространства-имён)) |
| `rbac.deckhouse.io/delegatable: "true"` | Роли `d8:namespace:*`, `d8:project:*` и пользовательские | Роль можно указывать в `RoleBinding` [внутри пространств имён проектов](../multitenancy-manager/usage.html#какие-роли-доступны-в-rolebinding-внутри-проекта). Ставьте на собственные роли, которые должны быть доступны в проектах |
| `rbac.deckhouse.io/deprecated: "true"` | [Устаревшие роли-псевдонимы](#устаревшие-имена-ролей) | Роль устарела и будет удалена; переведите привязки на новую роль |
| `module` | Встроенные объекты | Имя модуля DKP, которому принадлежит объект. Удобен для выборки в селекторах агрегации (например, все capabilities одного модуля) |
| `heritage: deckhouse` | Встроенные объекты | Признак объекта платформы. Устанавливать на собственные объекты нельзя |

Аннотации на объектах `ClusterRole`:

| Аннотация | Кто ставит | Назначение |
|-----------|-----------|------------|
| `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` | Платформа | [Отображаемые название и описание](#отображаемые-названия-ролей) на русском |
| `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` | Платформа | Отображаемые название и описание на английском |
| `custom.meta.deckhouse.io/title`, `custom.meta.deckhouse.io/description` | Администратор | Переопределение отображаемого названия/описания; единственное разрешённое изменение встроенных ролей |
| `rbac.deckhouse.io/bindable-only-via` | Платформа | Список видов привязок, которыми можно назначить роль (у проектных ролей — `ProjectRoleBinding,ClusterProjectRoleBinding`) |
| `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` | Администратор | Запрещает выдавать роль в проектах: существующие привязки продолжают работать, новые не создаются ([подробнее](../multitenancy-manager/usage.html#предоставление-доступа-внутри-проекта)) |
| `rbac.deckhouse.io/deprecated-replaced-by` | Платформа | На устаревших ролях-псевдонимах: имя роли, на которую нужно перейти |

### Ограничения уровня admin и права superadmin

Ролевая модель сознательно разводит два уровня администрирования:

- **`admin`** — повседневный администратор. Управляет ресурсами, квотами и доступом в своей области, но не может выполнять операции, которые позволяют выйти за её пределы или нарушить работу компонентов платформы.
- **`superadmin`** — «аварийный» администратор. Обладает всеми правами `admin` и дополнительно может выполнять опасные операции. Выдавайте этот уровень осознанно и только тем, кому он действительно необходим.

Что запрещено уровню `admin` и разрешено только уровню `superadmin`:

- **Выпуск токенов ServiceAccount'ов** (`kubectl create token`) **и выполнение запросов от имени ServiceAccount** (`kubectl --as system:serviceaccount:...`). Токен ServiceAccount'а — это готовая учётная запись: завладев токеном служебного аккаунта платформенного компонента, можно получить его права далеко за пределами пространства имён. Поэтому `admin` управляет самими объектами `ServiceAccount` (создание, удаление), но не может выпускать для них токены и действовать от их имени.
- **Изменение и удаление системных ресурсов в пользовательских пространствах имён.** Некоторые компоненты платформы размещают свои объекты (например, поды Dex-аутентификатора или поды и диски виртуальных машин) прямо в пространствах имён приложений. Такие объекты помечены лейблом `deckhouse.io/system-resource: "true"`. Изменять и удалять их может только `superadmin`; для остальных пользователей эти операции отклоняются на уровне API-сервера с пояснением.
- **Подключение к системным подам** — `kubectl exec`, `kubectl attach` и `kubectl port-forward` в под с лейблом `deckhouse.io/system-resource: "true"` доступны только `superadmin`. Это защищает от чтения чужих секретов и вмешательства в работу платформенных компонентов изнутри их подов.

При этом права `superadmin` тоже не безграничны:

- Ресурсы, созданные из [шаблона проекта](../multitenancy-manager/) (лейбл `heritage: multitenancy-manager`), не может изменить **никто**, включая `superadmin`, — они управляются исключительно контроллером проектов. Чтобы изменить такой ресурс, измените шаблон проекта или сам проект.
- Роль назначается через `RoleBinding` и действует только в том пространстве имён, где выдана: `superadmin` одного пространства имён не получает никаких особых прав в другом.

### Встроенные защиты ролевой модели

Ролевая модель защищена набором проверок на уровне API-сервера. Они не требуют настройки и предотвращают типовые ошибки и попытки повышения привилегий:

- **Нельзя выдать ограниченную по области роль на весь кластер.** `ClusterRoleBinding` на роли `d8:namespace:*`, `d8:project:*` (и их `d8:custom:*`-варианты) отклоняется — иначе роль, рассчитанная на одно пространство имён или проект, действовала бы во всех пространствах имён сразу. Используйте `RoleBinding` в нужном пространстве имён либо `ProjectRoleBinding`/`ClusterProjectRoleBinding` для проекта. `ClusterRoleBinding` допустим только для системных и подсистемных ролей — они кластерные по своей природе.
- **Нельзя выдать capability на весь кластер.** `ClusterRoleBinding` на любую capability (`d8:*-capability:*`, включая пользовательские) отклоняется: capability — строительный блок для ролей, а не самостоятельная роль. В отдельном пространстве имён привязать capability через `RoleBinding` можно.
- **Нельзя получить управление проектами через собственную роль.** Создание `Role` или `ClusterRole`, дающей права на изменение ресурсов управления проектами (`projects`, `projecttemplates`, `projectrolebindings`, `clusterprojectrolebindings`, `projectnamespaces`), отклоняется. Эти права дают только встроенные роли `d8:project:*`. Без этой защиты администратор пространства имён мог бы создать роль с правом создавать `ProjectRoleBinding` и выдать себе доступ ко всему проекту. Роли только на чтение этих ресурсов разрешены.
- **Нельзя смешивать пользовательскую и административную области в одной роли.** Собственная роль не может одновременно агрегировать capabilities областей `namespace`/`project` и областей `system`/`subsystem` — это исключает создание «супер-роли», объединяющей доступ к приложениям и к платформе.
- **Собственные роли не могут содержать прямых RBAC-правил** — только агрегировать capabilities. Права описываются в отдельных пользовательских capabilities, что делает состав любой роли прозрачным. Подробнее — [в FAQ](faq.html#как-расширить-роли-или-создать-новую).

### Отображаемые названия ролей

Каждая встроенная роль и capability имеет локализованные название и описание в аннотациях:

- `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` — на русском;
- `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` — на английском.

Эти аннотации использует, например, веб-интерфейс Deckhouse Console при отображении списка ролей.

Если стандартное название не подходит (например, вы хотите называть роли в терминах, принятых в компании), добавьте на роль аннотации `custom.meta.deckhouse.io/title` и `custom.meta.deckhouse.io/description` — интерфейс покажет их вместо стандартных. Это единственное разрешённое изменение встроенных ролей: изменение их правил, агрегации или лейблов отклоняется.

```shell
d8 k annotate clusterrole d8:namespace:admin \
  custom.meta.deckhouse.io/title='Администратор команды'
```

<div style="height: 0;" id="устаревшие-имена-ролей"></div>

### Устаревшие имена ролей

Прежние имена ролей основной модели (`d8:manage:<подсистема>:<уровень>`, `d8:manage:all:<уровень>` и `d8:use:role:<уровень>`) устарели и будут удалены в следующем релизе. Для обратной совместимости они временно сохранены как роли-псевдонимы: существующие привязки продолжают работать и дают те же права, что и новые роли.

Соответствие имён:

| Устаревшее имя | Новое имя |
|----------------|-----------|
| `d8:manage:all:<уровень>` | `d8:system:<уровень>` |
| `d8:manage:<подсистема>:<уровень>` | `d8:subsystem:<подсистема>:<уровень>` |
| `d8:use:role:<уровень>` | `d8:namespace:<уровень>` |

Обратите внимание: устаревшая роль `d8:use:role:admin` соответствует роли `d8:namespace:admin` и, как и она, [больше не даёт](#ограничения-уровня-admin-и-права-superadmin) права выпускать токены ServiceAccount'ов — теперь для этого нужен уровень `superadmin`.

Переведите существующие `RoleBinding` и `ClusterRoleBinding` на новые имена ролей. Найти привязки, использующие устаревшие имена, можно командой:

```shell
d8 k get clusterrolebindings,rolebindings -A -o json \
  | jq -r '.items[] | select(.roleRef.name | test("^d8:(manage|use):")) | "\(.kind) \(.metadata.namespace // "-") \(.metadata.name) -> \(.roleRef.name)"'
```

<div style="height: 0;" id="текущая-ролевая-модель"></div>

## Устаревшая ролевая модель

Особенности:

- Модуль реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC.
- Настройка прав доступа происходит с помощью [ресурсов](cr.html).
- Управление доступом к инструментам масштабирования (параметр `allowScale` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-allowscale) или [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-allowscale)).
- Управление доступом к форвардингу портов (параметр `portForwarding` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-portforwarding) или [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-portforwarding)).
- Управление списком разрешённых пространств имён в формате labelSelector (параметр `namespaceSelector` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector)).

В модуле, кроме использования RBAC, можно использовать удобный набор высокоуровневых ролей:

- `User` — позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать секреты и выполнять port-forward;
- `PrivilegedUser` — то же самое, что и `User`, но позволяет заходить в контейнеры, читать секреты, а также удалять поды (что обеспечивает возможность перезагрузки);
- `Editor` — то же самое, что и `PrivilegedUser`, но предоставляет возможность создавать, изменять и удалять все объекты, которые обычно нужны для прикладных задач;
- `Admin` — то же самое, что и `Editor`, но позволяет удалять служебные объекты (производные ресурсы, например `ReplicaSet`, `certmanager.k8s.io/challenges` и `certmanager.k8s.io/orders`);
- `ClusterEditor` — то же самое, что и `Editor`, но позволяет управлять ограниченным набором `cluster-wide`-объектов, которые могут понадобиться для прикладных задач (`ClusterXXXMetric`, `KeepalivedInstance`, `DaemonSet` и т. д). Роль для работы оператора кластера;
- `ClusterAdmin` — то же самое, что и `ClusterEditor` + `Admin`, но позволяет управлять служебными `cluster-wide`-объектами (производные ресурсы, например `MachineSets`, `Machines`, `OpenstackInstanceClasses` и т. п., а также `ClusterAuthorizationRule`, `ClusterRoleBindings` и `ClusterRole`). Роль для работы администратора кластера. **Важно**, что `ClusterAdmin`, поскольку он уполномочен редактировать `ClusterRoleBindings`, может **сам себе расширить полномочия**;
- `SuperAdmin` — разрешены любые действия с любыми объектами, при этом ограничения `namespaceSelector` и `limitNamespaces` продолжат работать.

{% alert level="warning" %}
Режим multi-tenancy (авторизация по пространству имён) в данный момент реализован по временной схеме и **не гарантирует безопасность!**
{% endalert %}

В случае, если в [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule)-ресурсе используется `namespaceSelector`, параметры `limitNamespaces` и `allowAccessToSystemNamespace` не учитываются.

Если вебхук, который реализовывает систему авторизации, по какой-то причине будет недоступен, опции `allowAccessToSystemNamespaces`, `namespaceSelector` и `limitNamespaces` в custom resource перестанут применяться и пользователи будут иметь доступ во все пространства имён. После восстановления доступности вебхука опции продолжат работать.

### Список доступа для каждой роли модуля по умолчанию

Каждая следующая роль наследует права предыдущих ролей. В блоке роли показаны только права, которые она добавляет.

Список ниже включает:

- стандартные права устаревшей ролевой модели (права k8s);
- права, создаваемые встроенными модулями Deckhouse.

В нем отсутствуют права [модулей из источника](/products/kubernetes-platform/documentation/v1/architecture/module-development/run/#источник-модулей).

Модули из источника при включении в кластере создают права на предоставляемые ими ресурсы. При выключении модуля из источника созданные им права удаляются.

Для просмотра прав, созданных модулями из источника, используйте [команду](#get_rules).

Сокращения для `verbs`:
<!-- start user-authz roles placeholder -->
* read - `get`, `list`, `watch`
* read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
* write - `create`, `delete`, `deletecollection`, `patch`, `update`

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - apps/deployments
    - apps/replicasets
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificaterequests
    - cert-manager.io/certificates
    - cert-manager.io/clusterissuers
    - cert-manager.io/issuers
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - config.gatekeeper.sh/configs
    - configmaps
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/deckhousereleases
    - deckhouse.io/deschedulers
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/nodegroups
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
    - deckhouse.io/projects
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/securitypolicyexceptions
    - deckhouse.io/vcdaffinityrules
    - deckhouse.io/vcdinstanceclasses
    - deckhouse.io/vsphereinstanceclasses
    - deckhouse.io/yandexinstanceclasses
    - deckhouse.io/zvirtinstanceclasses
    - discovery.k8s.io/endpointslices
    - endpoints
    - events
    - events.k8s.io/events
    - expansion.gatekeeper.sh/expansiontemplate
    - extensions.istio.io/wasmplugins
    - extensions/daemonsets
    - extensions/deployments
    - extensions/ingresses
    - extensions/replicasets
    - extensions/replicationcontrollers
    - externaldata.gatekeeper.sh/providers
    - gateway.networking.k8s.io/backendtlspolicies
    - gateway.networking.k8s.io/gatewayclasses
    - gateway.networking.k8s.io/gateways
    - gateway.networking.k8s.io/grpcroutes
    - gateway.networking.k8s.io/httproutes
    - gateway.networking.k8s.io/listenersets
    - gateway.networking.k8s.io/referencegrants
    - gateway.networking.k8s.io/tcproutes
    - gateway.networking.k8s.io/tlsroutes
    - gateway.networking.k8s.io/udproutes
    - infrastructure.cluster.x-k8s.io/deckhouseclusters
    - infrastructure.cluster.x-k8s.io/deckhousemachines
    - infrastructure.cluster.x-k8s.io/deckhousemachinetemplates
    - infrastructure.cluster.x-k8s.io/dynamixclusters
    - infrastructure.cluster.x-k8s.io/dynamixmachines
    - infrastructure.cluster.x-k8s.io/dynamixmachinetemplates
    - infrastructure.cluster.x-k8s.io/huaweicloudclusters
    - infrastructure.cluster.x-k8s.io/huaweicloudmachines
    - infrastructure.cluster.x-k8s.io/huaweicloudmachinetemplates
    - infrastructure.cluster.x-k8s.io/vcdclusters
    - infrastructure.cluster.x-k8s.io/vcdclustertemplates
    - infrastructure.cluster.x-k8s.io/vcdmachines
    - infrastructure.cluster.x-k8s.io/vcdmachinetemplates
    - infrastructure.cluster.x-k8s.io/zvirtclusters
    - infrastructure.cluster.x-k8s.io/zvirtmachines
    - infrastructure.cluster.x-k8s.io/zvirtmachinetemplates
    - limitranges
    - metrics.k8s.io/nodes
    - metrics.k8s.io/pods
    - multitenancy.deckhouse.io/availableclusterresources
    - mutations.gatekeeper.sh/assign
    - mutations.gatekeeper.sh/assignimage
    - mutations.gatekeeper.sh/assignmetadata
    - mutations.gatekeeper.sh/modifyset
    - namespaces
    - network.deckhouse.io/egressgatewaypolicies
    - network.deckhouse.io/egressgateways
    - network.deckhouse.io/metalloadbalancerbgppeers
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/metalloadbalancerconfigurations
    - network.deckhouse.io/metalloadbalancerpools
    - network.deckhouse.io/servicewithhealthchecks
    - networking.istio.io/destinationrules
    - networking.istio.io/gateways
    - networking.istio.io/serviceentries
    - networking.istio.io/sidecars
    - networking.istio.io/virtualservices
    - networking.istio.io/workloadentries
    - networking.istio.io/workloadgroups
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - nodes
    - persistentvolumeclaims
    - persistentvolumes
    - pods
    - pods/log
    - policy/poddisruptionbudgets
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - replicationcontrollers
    - resourcequotas
    - security.istio.io/authorizationpolicies
    - security.istio.io/peerauthentications
    - security.istio.io/requestauthentications
    - serviceaccounts
    - services
    - status.gatekeeper.sh/configpodstatuses
    - status.gatekeeper.sh/connectionpodstatuses
    - status.gatekeeper.sh/constraintpodstatuses
    - status.gatekeeper.sh/constrainttemplatepodstatuses
    - status.gatekeeper.sh/expansiontemplatepodstatuses
    - status.gatekeeper.sh/mutatorpodstatuses
    - status.gatekeeper.sh/providerpodstatuses
    - storage.k8s.io/storageclasses
    - syncset.gatekeeper.sh/syncsets
    - telemetry.istio.io/telemetries
    - templates.gatekeeper.sh/constrainttemplates
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `PrivilegedUser` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`):

```text
create:
    - pods/eviction
create,get:
    - pods/attach
    - pods/exec
delete,deletecollection:
    - pods
read:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Editor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`):

```text
write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificates
    - cert-manager.io/issuers
    - configmaps
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - discovery.k8s.io/endpointslices
    - endpoints
    - extensions/deployments
    - extensions/ingresses
    - gateway.networking.k8s.io/backendtlspolicies
    - gateway.networking.k8s.io/gateways
    - gateway.networking.k8s.io/grpcroutes
    - gateway.networking.k8s.io/httproutes
    - gateway.networking.k8s.io/listenersets
    - gateway.networking.k8s.io/referencegrants
    - gateway.networking.k8s.io/tcproutes
    - gateway.networking.k8s.io/tlsroutes
    - gateway.networking.k8s.io/udproutes
    - network.deckhouse.io/servicewithhealthchecks
    - networking.istio.io/destinationrules
    - networking.istio.io/gateways
    - networking.istio.io/serviceentries
    - networking.istio.io/sidecars
    - networking.istio.io/virtualservices
    - networking.istio.io/workloadentries
    - networking.istio.io/workloadgroups
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - persistentvolumeclaims
    - policy/poddisruptionbudgets
    - secrets
    - security.istio.io/authorizationpolicies
    - security.istio.io/peerauthentications
    - security.istio.io/requestauthentications
    - serviceaccounts
    - services
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Admin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,patch,update:
    - pods
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - apps/replicasets
    - cert-manager.io/certificaterequests
    - extensions/replicasets
read:
    - 'deckhouse.io/moduleconfigs (resourceNames: deckhouse)'
read-write:
    - deckhouse.io/authorizationrules
write:
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/deckhousereleases
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/securitypolicyexceptions
    - extensions.istio.io/wasmplugins
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - cert-manager.io/certificaterequests
patch,update:
    - nodes
read:
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
    - 'deckhouse.io/moduleconfigs (resourceNames: deckhouse)'
    - install.istio.io/istiooperators
    - multitenancy.deckhouse.io/grantableclusterresourcedefinitions
    - multitenancy.deckhouse.io/grantableclusterresourcereferences
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - sailoperator.io/istiocnis
    - sailoperator.io/istiorevisions
    - sailoperator.io/istiorevisiontags
    - sailoperator.io/istios
    - sailoperator.io/ztunnels
read-write:
    - deckhouse.io/nodegroupconfigurations
    - deckhouse.io/staticinstances
    - multitenancy.deckhouse.io/clusterresourcegrantpolicies
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - cert-manager.io/clusterissuers
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/deckhousereleases
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/nodegroups
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/securitypolicyexceptions
    - extensions.istio.io/wasmplugins
    - extensions/daemonsets
    - gateway.networking.k8s.io/gatewayclasses
    - network.deckhouse.io/egressgatewaypolicies
    - network.deckhouse.io/egressgateways
    - storage.k8s.io/storageclasses
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterAdmin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):

```text
delete,deletecollection,get,list,patch,update,watch:
    - machine.sapcloud.io/alicloudmachineclasses
    - machine.sapcloud.io/awsmachineclasses
    - machine.sapcloud.io/azuremachineclasses
    - machine.sapcloud.io/gcpmachineclasses
    - machine.sapcloud.io/machinedeployments
    - machine.sapcloud.io/machines
    - machine.sapcloud.io/machinesets
    - machine.sapcloud.io/openstackmachineclasses
    - machine.sapcloud.io/packetmachineclasses
    - machine.sapcloud.io/vspheremachineclasses
    - machine.sapcloud.io/yandexmachineclasses
get,list,patch,update,watch:
    - control-plane.deckhouse.io/controlplanenodes
list:
    - dex.coreos.com/offlinesessionses
    - dex.coreos.com/passwords
patch,update:
    - deckhouse.io/vcdaffinityrules
    - infrastructure.cluster.x-k8s.io/deckhouseclusters
    - infrastructure.cluster.x-k8s.io/deckhousemachines
    - infrastructure.cluster.x-k8s.io/deckhousemachinetemplates
    - infrastructure.cluster.x-k8s.io/dynamixclusters
    - infrastructure.cluster.x-k8s.io/dynamixmachines
    - infrastructure.cluster.x-k8s.io/dynamixmachinetemplates
    - infrastructure.cluster.x-k8s.io/huaweicloudclusters
    - infrastructure.cluster.x-k8s.io/huaweicloudmachines
    - infrastructure.cluster.x-k8s.io/huaweicloudmachinetemplates
    - infrastructure.cluster.x-k8s.io/vcdclusters
    - infrastructure.cluster.x-k8s.io/vcdclustertemplates
    - infrastructure.cluster.x-k8s.io/vcdmachines
    - infrastructure.cluster.x-k8s.io/vcdmachinetemplates
    - infrastructure.cluster.x-k8s.io/zvirtclusters
    - infrastructure.cluster.x-k8s.io/zvirtmachines
    - infrastructure.cluster.x-k8s.io/zvirtmachinetemplates
    - machine.sapcloud.io/machinedeployments/scale
proxy:
    - nodes
read:
    - cluster.x-k8s.io/machinedrainrules
    - control-plane.deckhouse.io/controlplaneoperations
    - infrastructure.cluster.x-k8s.io/deckhousecontrolplanes
    - infrastructure.cluster.x-k8s.io/staticclusters
    - infrastructure.cluster.x-k8s.io/staticmachines
    - nfd.k8s-sigs.io/nodefeaturegroups
    - nfd.k8s-sigs.io/nodefeaturerules
    - nfd.k8s-sigs.io/nodefeatures
read-write:
    - cluster.x-k8s.io/clusters
    - cluster.x-k8s.io/machinedeployments
    - cluster.x-k8s.io/machinehealthchecks
    - cluster.x-k8s.io/machinepools
    - cluster.x-k8s.io/machines
    - cluster.x-k8s.io/machinesets
    - deckhouse.io/clusterauthorizationrules
    - deckhouse.io/dexproviderchecks
    - deckhouse.io/dexproviders
    - deckhouse.io/groups
    - deckhouse.io/nodeusers
    - deckhouse.io/sshcredentials
    - deckhouse.io/useroperations
    - deckhouse.io/users
    - infrastructure.cluster.x-k8s.io/staticmachinetemplates
    - nodes/configz
    - nodes/healthz
    - nodes/log
    - nodes/metrics
    - nodes/pods
    - nodes/proxy
    - nodes/stats
write:
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - cluster.x-k8s.io/machinedeployments/scale
    - config.gatekeeper.sh/configs
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/deschedulers
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
    - deckhouse.io/projects
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/vcdinstanceclasses
    - deckhouse.io/vsphereinstanceclasses
    - deckhouse.io/yandexinstanceclasses
    - deckhouse.io/zvirtinstanceclasses
    - expansion.gatekeeper.sh/expansiontemplate
    - externaldata.gatekeeper.sh/providers
    - install.istio.io/istiooperators
    - limitranges
    - mutations.gatekeeper.sh/assign
    - mutations.gatekeeper.sh/assignimage
    - mutations.gatekeeper.sh/assignmetadata
    - mutations.gatekeeper.sh/modifyset
    - namespaces
    - network.deckhouse.io/metalloadbalancerbgppeers
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/metalloadbalancerconfigurations
    - network.deckhouse.io/metalloadbalancerpools
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - resourcequotas
    - sailoperator.io/istiocnis
    - sailoperator.io/istiorevisions
    - sailoperator.io/istiorevisiontags
    - sailoperator.io/istios
    - sailoperator.io/ztunnels
    - status.gatekeeper.sh/configpodstatuses
    - status.gatekeeper.sh/connectionpodstatuses
    - status.gatekeeper.sh/constraintpodstatuses
    - status.gatekeeper.sh/constrainttemplatepodstatuses
    - status.gatekeeper.sh/expansiontemplatepodstatuses
    - status.gatekeeper.sh/mutatorpodstatuses
    - status.gatekeeper.sh/providerpodstatuses
    - syncset.gatekeeper.sh/syncsets
    - templates.gatekeeper.sh/constrainttemplates
```
<!-- end user-authz roles placeholder -->

{: #get_rules .anchored}

Вы можете получить дополнительный список правил доступа для роли модуля из кластера ([существующие пользовательские правила](usage.html#настройка-прав-высокоуровневых-ролей) и нестандартные правила из других модулей Deckhouse):

```bash
D8_ROLE_NAME=Editor
kubectl get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```
