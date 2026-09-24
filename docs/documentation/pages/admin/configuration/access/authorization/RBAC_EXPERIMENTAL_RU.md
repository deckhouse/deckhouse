---
title: "Гранулярная модель авторизации"
permalink: ru/admin/configuration/access/authorization/rbac-experimental.html
description: "Настройка гранулярной модели RBAC-авторизации в Deckhouse Platform: области действия ролей, уровни доступа, проектные роли"
lang: ru
---

Гранулярная ролевая модель построена на принципе агрегации: она объединяет низкоуровневые роли в более крупные, охватывающие типовые задачи. Это упрощает расширение модели за счёт добавления собственных ролей.

Для использования гранулярной ролевой модели в кластере должен быть включён модуль [`user-authz`](/modules/user-authz/).
Модуль создаёт набор специальных агрегированных кластерных ролей (ClusterRole), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %}
В модуле реализованы две ролевые модели: гранулярная (описанная на этой странице, рекомендуется к использованию) и [упрощённая](rbac-current.html), построенная на ресурсах ClusterAuthorizationRule и AuthorizationRule (поддержка будет прекращена в будущих релизах).

Модели не совместимы по ресурсам — автоматическая конвертация невозможна, — но могут использоваться одновременно: права из обеих моделей суммируются.
{% endalert %}

В отличие от [упрощённой ролевой модели](rbac-current.html), гранулярная не использует ресурсы [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Права доступа настраиваются стандартным для RBAC Kubernetes способом: с помощью создания ресурсов [RoleBinding или ClusterRoleBinding](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#rolebinding-and-clusterrolebinding), с указанием в них одной из подготовленных модулем `user-authz` ролей. Для доступа сразу ко всем неймспейсам проекта дополнительно используются ресурсы [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) модуля `multitenancy-manager`.

{% alert level="info" %}
Выдавать доступ можно не только вручную через YAML-манифесты: в [веб-интерфейсе Deckhouse Platform](/products/kubernetes-platform/documentation/v1/user/web/ui.html) есть мастер выдачи доступа. Он проводит по шагам (кому выдать доступ → где → с каким уровнем), сам выбирает правильный вид привязки (RoleBinding, ClusterRoleBinding, ProjectRoleBinding или ClusterProjectRoleBinding) и позволяет собрать собственную роль из готовых блоков без написания YAML.
{% endalert %}

Модуль создаёт специальные агрегированные кластерные роли (ClusterRole). Используя эти роли в RoleBinding или ClusterRoleBinding, можно решать следующие задачи:

- Управлять доступом к модулям определённой [подсистемы](#подсистемы-ролевой-модели).

  Например, чтобы дать возможность пользователю, выполняющему функции сетевого администратора, настраивать *сетевые* модули (например, [`cni-cilium`](/modules/cni-cilium/), [`ingress-nginx`](/modules/ingress-nginx/), [`istio`](/modules/istio/) и т. д.), можно использовать в ClusterRoleBinding роль `d8:subsystem:networking:manager`.
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
| Проект | `d8:project:<уровень>` | Команды, работающие с [проектами](/modules/multitenancy-manager/) | Только [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) или [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) |
| Подсистема | `d8:subsystem:<подсистема>:<уровень>` | Администраторы части платформы | ClusterRoleBinding |
| Вся платформа | `d8:system:<уровень>` | Администраторы платформы | ClusterRoleBinding |

### Уровни доступа

В каждой области действия для ролей предусмотрено несколько уровней доступа:

- Для областей «неймспейс» и «проект» уровней пять: `viewer` → `user` → `manager` → `admin` → `superadmin`.
- Для областей «подсистема» и «вся платформа» уровней три: `viewer` → `manager` → `superadmin`. Уровней `user` и `admin` здесь нет: на системном уровне нет «пользовательских» ресурсов, которыми можно было бы пользоваться, не администрируя их.

Уровни доступа образуют иерархию с кумулятивным принципом: каждый последующий уровень наследует все права предыдущего и расширяет их. Например, уровень `manager` в области действия «неймспейс» включает набор прав уровня `user`, который, в свою очередь, включает набор прав уровня `viewer`.

## Namespace-роли

{% alert level="warning" %}
Namespace-роль можно использовать только в ресурсе RoleBinding.
{% endalert %}

Namespace-роли предназначены для назначения прав пользователю **в конкретном неймспейсе**. Под пользователями понимаются, например, разработчики, которые используют настроенный администратором кластер для развёртывания своих приложений. Таким пользователям не нужно управлять модулями DP или кластером, но им нужно иметь возможность, например, создавать свои Ingress-ресурсы, настраивать аутентификацию приложений и сбор логов с приложений.

Namespace-роль определяет права на доступ к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes (Pod, Deployment, Secret, ConfigMap и т. п.).

В DP создаются следующие namespace-роли:

| Роль | Разрешённые действия | Ограничения |
|------|------------------------|--------------|
| `d8:namespace:viewer` | Просмотр стандартных ресурсов Kubernetes (кроме секретов и ресурсов RBAC), логов подов и метрик, аутентификация в кластере | Нет доступа к секретам, `exec`, изменению ресурсов |
| `d8:namespace:user` | Всё, что даёт `viewer`, плюс: просмотр секретов, `kubectl exec`/`attach`, удаление подов (без создания/изменения), `kubectl port-forward`/`proxy`, изменение числа реплик контроллеров | Нельзя создавать/редактировать объекты |
| `d8:namespace:manager` | Всё, что даёт `user`, плюс: управление ресурсами модулей (например, Certificate, PodLoggingConfig) и стандартными namespaced-ресурсами Kubernetes (Pod, Deployment, ConfigMap, Secret, Service, Ingress, NetworkPolicy, CronJob и т. п.) | Нет доступа к квотам и RBAC |
| `d8:namespace:admin` | Всё, что даёт `manager`, плюс: управление ResourceQuota, LimitRange, ServiceAccount, Role, RoleBinding | Полный доступ в неймспейсе, кроме операций, закреплённых за `superadmin` |
| `d8:namespace:superadmin` | Всё, что даёт `admin`, плюс опасные с точки зрения безопасности операции: выпуск токенов ServiceAccount и запросы от их имени, управление [системными ресурсами в неймспейсе](#ограничения-уровня-admin-и-права-superadmin) (например, подами Dex или подами/PVC виртуальных машин) | — |

Подробное разделение прав между `admin` и `superadmin` описано в разделе [«Ограничения уровня admin и права superadmin»](#ограничения-уровня-admin-и-права-superadmin).

### Автоматический доступ к справочным cluster-wide-ресурсам

Работа в неймспейсе требует чтения некоторых cluster-wide-«справочников»: например, чтобы указать в манифесте `storageClassName` или `ingressClassName`, нужно видеть список StorageClass'ов и IngressClass'ов. Поэтому каждый субъект, получивший RoleBinding на любую роль `d8:namespace:*`, автоматически получает и доступ на чтение таких справочных ресурсов (StorageClass, IngressClass, PriorityClass, RuntimeClass, VolumeSnapshotClass, ClusterLogDestination и т. п.).

Технически это выглядит как автоматически создаваемый ClusterRoleBinding на роль [`d8:dict`](#глобальные-справочники-ресурсов) с лейблом `rbac.deckhouse.io/dict: "true"`. Такие объекты управляются платформой: они появляются при выдаче первой namespace-привязки субъекту и удаляются, когда у субъекта не остаётся ни одной, — редактировать их вручную не нужно.

## Проектные роли

{% alert level="warning" %}
Проектную роль нельзя назначить через ClusterRoleBinding — попытка будет отклонена. Для назначения роли на весь проект используйте [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) или [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding). Допускается также обычный RoleBinding в одном из неймспейсов проекта — тогда роль действует только в нём.
{% endalert %}

Проектные роли (`d8:project:<уровень>`) предназначены для работы с [проектами](../../../multitenancy/project-management.html) — изолированными окружениями, которые могут включать несколько неймспейсов. Для проектных ролей используются те же уровни доступа, что для namespace-ролей: `viewer`, `user`, `manager`, `admin`, `superadmin`.

Каждая проектная роль включает все права namespace-роли того же уровня и дополнительно даёт права на управление самим проектом:

- `d8:project:viewer` — права `d8:namespace:viewer` плюс просмотр ресурсов ProjectNamespace и ProjectRoleBinding проекта;
- `d8:project:manager` — права `d8:namespace:manager` плюс управление дополнительными неймспейсами проекта (ресурсы ProjectNamespace);
- `d8:project:admin` — права `d8:namespace:admin` плюс управление доступом к проекту (ресурсы ProjectRoleBinding) и право привязывать встроенные роли `d8:project:*` и `d8:namespace:*` (кроме уровня `superadmin`) другим пользователям в рамках проекта;
- `d8:project:superadmin` — аналогично соотношению `d8:namespace:superadmin` и `d8:namespace:admin`.

Назначенная через ProjectRoleBinding роль автоматически действует во **всех** неймспейсах проекта — как в основном, так и в дополнительных, включая созданные позже.

## Системные и подсистемные роли

{% alert level="warning" %}
Системные и подсистемные роли не дают доступа к неймспейсам пользовательских приложений.

Они определяют доступ только к системным неймспейсам (начинающимся с `d8-` или `kube-`), и только к тем из них, в которых работают модули соответствующей подсистемы роли.

Системная или подсистемная роль сама по себе не даёт права раздавать доступ другим людям: создание User или Group на email, на котором уже есть грант, или запись ClusterAuthorizationRule — это раздача ролей, и запрос пройдёт, только если у запрашивающего уже есть покрывающие права или ему явно разрешено назначать эти роли.
{% endalert %}

Системные (`d8:system:*`) и подсистемные (`d8:subsystem:*`) роли предназначены для назначения прав на управление всей платформой или её частью ([подсистемой](#подсистемы-ролевой-модели)), но не самими приложениями пользователей. С помощью подсистемной роли можно, например, дать возможность администратору безопасности управлять модулями, ответственными за функции безопасности кластера. Тогда администратор безопасности сможет настраивать аутентификацию, авторизацию, политики безопасности и т. п., но не сможет управлять остальными функциями кластера (например, настройками сети и мониторинга) и изменять настройки в неймспейсах приложений пользователей.

{% alert level="warning" %}
Системная/подсистемная роль ограничивает, к каким модулям и неймспейсам обращается субъект, но не ограничивает привилегии, которые он может получить через доступные ему модули. Это особенно важно учитывать для подсистемы `security`: право управлять аутентификацией и авторизацией равносильно полному контролю над кластером — субъект, который может управлять модулем `user-authn`, способен зарегистрировать провайдер идентификации или сбросить учётные данные любого локального пользователя, а субъект, который может управлять модулем `user-authz`, способен изменить правила авторизации. В обоих случаях он может получить идентичность с любыми привилегиями, включая администратора кластера, поэтому при планировании доступа считайте роль подсистемы `security` равной роли администратора кластера.
{% endalert %}

Системная/подсистемная роль определяет права на доступ:

- к cluster-wide-ресурсам Kubernetes;
- к управлению модулями DP (ресурсы ModuleConfig) в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми модулями DP для роли `d8:system:*`;
- к управлению cluster-wide-ресурсами модулей DP в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми ресурсами модулей DP для роли `d8:system:*`;
- к системным неймспейсам (начинающимся с `d8-` или `kube-`), в которых работают модули [подсистемы](#подсистемы-ролевой-модели) роли, или ко всем системным неймспейсам для роли `d8:system:*`.

Формат названия системной роли — `d8:system:<ACCESS_LEVEL>`, подсистемной — `d8:subsystem:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:

- `SUBSYSTEM` — подсистема роли ([список подсистем](#подсистемы-ролевой-модели));
- `ACCESS_LEVEL` — уровень доступа.

Примеры ролей:

- `d8:system:viewer` — доступ на просмотр конфигурации всех модулей DP (ресурсы ModuleConfig), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) во всех системных неймспейсах (начинающихся с `d8-` или `kube-`);
- `d8:system:manager` — аналогично роли `d8:system:viewer`, только доступ на уровне `admin`, т. е. просмотр/создание/изменение/удаление конфигурации всех модулей DP (ресурсы ModuleConfig), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes во всех системных неймспейсах;
- `d8:subsystem:observability:viewer` — доступ на просмотр конфигурации модулей DP (ресурсы ModuleConfig) из подсистемы `observability`, их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в системных неймспейсах `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

В DP существует три уровня доступа для системных и подсистемных ролей:

- `viewer` — позволяет просматривать стандартные ресурсы Kubernetes, конфигурацию модулей (ресурсы ModuleConfig), cluster-wide-ресурсы модулей и namespaced-ресурсы модулей в неймспейсе модуля;
- `manager` — дополнительно к уровню `viewer` позволяет управлять стандартными ресурсами Kubernetes, конфигурацией модулей (ресурсы ModuleConfig), cluster-wide-ресурсами модулей и namespaced-ресурсами модулей в неймспейсе модуля;
- `superadmin` — дополнительно к уровню `manager` позволяет управлять системными ресурсами модулей подсистемы.

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

ClusterRoleBinding для `d8:dict` создаётся **автоматически**, когда RoleBinding ссылается на роль `d8:namespace:*` или `d8:project:*` (гранулярная модель) или на роль `user-authz:*` (упрощённая модель). RoleBinding, которые `multitenancy-manager` создаёт из ProjectRoleBinding и ClusterProjectRoleBinding, тоже учитываются, поэтому привязку получают и держатели проектных ролей. Это означает, что пользователи проекта могут просматривать справочные ресурсы без какой-либо ручной настройки. Привязка появляется вместе с RoleBinding и удаляется при его удалении. Привязку создаёт и управляет ею исключительно компонент `user-authz-controller` модуля. Самостоятельно её создавать не нужно.

{% alert level="info" %}
Роль `d8:dict` независима от механизма управления доступом к cluster-wide-ресурсам `multitenancy-manager`: она предоставляет права **только на чтение**
справочных ресурсов для их обнаружения. Механизм контролирует, **какие значения ресурсов**
проект может фактически использовать при создании объектов.
{% endalert %}

## Подсистемы ролевой модели

Каждый модуль DP принадлежит определённой подсистеме. Для каждой подсистемы существует набор ролей с разными уровнями доступа. Роли обновляются автоматически при включении или отключении модуля.

Например, для подсистемы `networking` существуют следующие подсистемные роли, которые можно использовать в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/):

- `d8:subsystem:networking:viewer`;
- `d8:subsystem:networking:manager`;
- `d8:subsystem:networking:superadmin`.

Область действия роли зависит от того, к какой подсистеме она принадлежит:

- Область действия ролей `d8:system:*` — все системные (начинающиеся с `d8-` или `kube-`) неймспейсы кластера.
- Область действия ролей подсистем — неймспейсы, в которых работают модули подсистемы (подробнее — в таблице состава подсистем ниже), а также все cluster-wide-объекты модулей подсистемы.

Таблица состава подсистем ролевой модели.

{% include rbac/rbac-subsystems-list.liquid %}

## Как устроены роли: агрегация и capabilities

Ни одна встроенная роль не содержит списка прав напрямую. Права описываются в отдельных небольших кластерных ролях — **capabilities**. Каждая capability отвечает за один вид действий (например, «просмотр логов», «управление квотами», «подключение к подам») и содержит конкретные RBAC-правила. Роль (`d8:namespace:admin`, `d8:system:viewer` и т. д.) — это пустая ClusterRole с правилом агрегации (`aggregationRule`): Kubernetes автоматически собирает в неё правила из всех capabilities с подходящими лейблами.

Такое устройство даёт два практических следствия:

- Модули DP расширяют роли автоматически: при включении модуля его capabilities добавляются в соответствующие встроенные роли, при выключении — удаляются. Список прав роли всегда соответствует набору включённых модулей.
- Вы можете собирать собственные роли из готовых capabilities, не описывая RBAC-правила вручную — как показано ниже.

Имена встроенных ролей и capabilities начинаются с префикса `d8:`. Этот неймспейс зарезервирован: создать собственную ClusterRole с именем `d8:*` нельзя — исключение составляет только префикс `d8:custom:*`, выделенный для пользовательских ролей и capabilities.

## Создание новой роли подсистемы

Если, например, текущие подсистемы не подходят под ролевое распределение в компании, требуется создать новую [подсистему](./#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `deckhouse`, подсистемы `kubernetes` и модуля `user-authn`.

Для решения этой задачи создайте следующую роль:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
    rbac.deckhouse.io/aggregate-to-system-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
```

В начале указаны лейблы для новой роли:

- показывает, какую namespace-роль хук должен использовать при создании RoleBinding в неймспейсах модулей:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- показывает, что роль является кастомной (кастомные роли не определяют собственных правил, а только агрегируют capabilities):

  ```yaml
  rbac.deckhouse.io/kind: custom-role
  ```

  > Этот лейбл обязателен.

- показывает, что роль является ролью подсистемы, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/scope: subsystem
  ```

- указывает подсистему, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/subsystem: mycustom
  ```

- позволяет роли `d8:system:manager` агрегировать эту роль в себя:

  ```yaml
  rbac.deckhouse.io/aggregate-to-system-as: manager
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль менеджера из подсистемы `deckhouse`:

  ```yaml
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- агрегирует все системные (scope `system`) capabilities модуля `user-authn`:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

{% alert level="info" %}

Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей».](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

{% endalert %}

Таким образом роль получает права от подсистем `deckhouse`, `kubernetes` и от модуля `user-authn`.

Особенности:

- кастомные роли и capabilities должны иметь префикс имени `d8:custom:` (остальной неймспейс `d8:` зарезервировано за встроенными объектами DP). Имя должно согласовываться с объявленной областью: подсистемная роль — `d8:custom:<подсистема>:<имя>` (сегмент — сама подсистема, как в примере выше), namespace- или проектная роль — `d8:custom:namespace:<имя>` и `d8:custom:project:<имя>`, capability — `d8:custom:<область>-capability:<имя>`. Имя, расходящееся с лейблом `rbac.deckhouse.io/scope`, будет отклонено;
- RoleBinding с namespace-ролью (`d8:namespace:<уровень>`) будут созданы в неймспейсах модулей агрегированных подсистем, уровень задаётся лейблом `rbac.deckhouse.io/use-role`.

## Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для manage-роли) CRD-объект — MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом. Выполните следующие шаги:

1. Дополните роль новым селектором:

   ```yaml
   rbac.deckhouse.io/aggregate-to-mycustom-as: manager
   ```

   Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей».](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

   Этот селектор позволит агрегировать capabilities к новой подсистеме через указание этого лейбла. После добавления нового селектора роль будет выглядеть так:

    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: d8:custom:mycustom:manager
      labels:
        rbac.deckhouse.io/use-role: admin
        rbac.deckhouse.io/kind: custom-role
        rbac.deckhouse.io/scope: subsystem
        rbac.deckhouse.io/subsystem: mycustom
        rbac.deckhouse.io/aggregate-to-system-as: manager
    aggregationRule:
      clusterRoleSelectors:
        - matchLabels:
            rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
        - matchLabels:
            rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
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

   Capability добавит свои права в роль подсистемы, дав права на просмотр нового объекта.

Особенности:

- кастомные capabilities должны иметь префикс имени `d8:custom:`. Требования к остальной части имени нет, но для читаемости лучше использовать этот стиль.

## Расширение существующих подсистемных ролей

Если необходимо расширить существующую роль, выполните те же шаги, что и в пункте выше, но измените лейблы и название роли. Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей».](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

Пример для расширения роли менеджера из подсистемы `deckhouse` (`d8:subsystem:deckhouse:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: deckhouse
    rbac.deckhouse.io/capability: "custom.subsystem-capability.deckhouse.superresource_view"
  name: d8:custom:subsystem-capability:deckhouse:superresource:view
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

Таким образом новая capability расширит роль `d8:subsystem:deckhouse:manager`.

## Расширение подсистемных ролей с добавлением нового неймспейса

Если необходимо добавить новый неймспейс, добавьте в роль лейбл:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей».](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

Этот лейбл сообщает хуку, что в этом неймспейсе нужно создать RoleBinding с namespace-ролью:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: deckhouse
     rbac.deckhouse.io/namespace: namespace
   name: d8:custom:subsystem-capability:deckhouse:superresource:view
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

Хук отслеживает ClusterRoleBinding и при создании привязки анализирует все системные и подсистемные роли, чтобы найти все объединенные в них capabilities с помощью проверки правила агрегации. Затем он берёт неймспейс из лейбла `rbac.deckhouse.io/namespace` и создает RoleBinding с namespace-ролью в этом неймспейсе.

Хук отслеживает только объекты с лейблом `rbac.deckhouse.io/scope: system` или `subsystem`. Capability без этого лейбла всё так же отдаёт свои правила роли через агрегацию, но её лейбл `rbac.deckhouse.io/namespace` не будет прочитан, и RoleBinding в неймспейсе не появится.

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

Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей».](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

## Создание собственной namespace- или проектной роли

Иногда встроенная иерархия уровней не подходит: например, нужна роль «разработчик» — просмотр всего неймспейса плюс чтение логов, но без права менять квоты или RBAC. Такая роль собирается из готовых capabilities, без написания RBAC-правил вручную.

Правила для собственных ролей:

- имя должно начинаться с `d8:custom:` (например, `d8:custom:namespace:developer`);
- роль должна иметь лейбл `rbac.deckhouse.io/kind: custom-role`;
- namespace- или проектная роль, которую будут выдавать через RoleBinding, должна также иметь `rbac.deckhouse.io/delegatable: "true"`. Каждый пользовательский неймспейс — это проект, и RoleBinding в нём принимают только роли с этим лейблом. На системные и подсистемные роли лейбл ставить нельзя — вебхук отклонит такую роль;
- роль **не может содержать собственных правил** (`rules`) — только агрегировать capabilities через `aggregationRule`. Права описываются в отдельных capabilities — так состав роли всегда прозрачен;
- нельзя в одной роли агрегировать capabilities пользовательских областей (`namespace`, `project`) вместе с административными (`system`, подсистемы) — такая роль будет отклонена.

Подробнее о лейблах и аннотациях ролей — в [«Справочнике лейблов и аннотаций ролей».](/modules/user-authz/#справочник-лейблов-и-аннотаций-ролей).

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

Если готовой capability с нужными правами нет, создайте собственную (`custom-capability` может содержать правила) и добавьте в `aggregationRule` роли селектор по её лейблу `rbac.deckhouse.io/capability` (в примере ниже — `matchLabels: {rbac.deckhouse.io/capability: "custom.logs-reader"}`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace-capability:logs-reader
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/capability: "custom.logs-reader"
rules:
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get", "list"]
```

Чтобы получить список всех доступных capabilities и их уникальных имён, используйте команду:

```shell
d8 k get clusterroles -l rbac.deckhouse.io/kind=capability \
  -o custom-columns='NAME:.metadata.name,CAPABILITY:.metadata.labels.rbac\.deckhouse\.io/capability'
```

Созданная роль назначается так же, как и встроенная: через RoleBinding в неймспейсе или через [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) на весь проект (для проектных ролей используйте `rbac.deckhouse.io/scope: project` и агрегируйте `aggregate-to-project-as`). Для RoleBinding в неймспейсе проекта на роли обязателен лейбл `delegatable`; ProjectRoleBinding его не требует (у него свой список допустимых префиксов). Назначить роль через ClusterRoleBinding нельзя — как и встроенные роли этих областей.

{% alert level="info" %}

Собрать такую роль можно и без YAML — мастером выдачи доступа в веб-интерфейсе Deckhouse Console: он показывает доступные capabilities, собирает из них роль и сразу создаёт нужную привязку.

{% endalert %}

## Ограничения уровня admin и права superadmin

В ролевой модели предусмотрено два уровня администрирования:

- `admin` — повседневный администратор. Управляет ресурсами, квотами и доступом в своей области, но не может выполнять операции, которые позволяют выйти за её пределы или нарушить работу компонентов платформы.
- `superadmin` — «аварийный» администратор. Обладает всеми правами `admin` и дополнительно может выполнять опасные операции. Выдавайте этот уровень осознанно и только тем, кому он действительно необходим.

Что запрещено уровню `admin` и разрешено только уровню `superadmin`:

- **Выпуск токенов ServiceAccount'ов** (`kubectl create token`) **и выполнение запросов от имени ServiceAccount** (`kubectl --as system:serviceaccount:...`). Токен ServiceAccount'а — это готовая учётная запись: завладев токеном служебного аккаунта платформенного компонента, можно получить его права далеко за пределами неймспейса. Поэтому `admin` управляет самими объектами `ServiceAccount` (создание, удаление), но не может выпускать для них токены и действовать от их имени.
- **Изменение и удаление системных ресурсов в пользовательских неймспейсах.** Некоторые компоненты платформы размещают свои объекты (например, поды Dex-аутентификатора или поды и диски виртуальных машин) прямо в неймспейсах приложений. Такие объекты помечены лейблом `deckhouse.io/system-resource: "true"`. Изменять и удалять их может только `superadmin`.
- **Подключение к системным подам** — `kubectl exec`, `kubectl attach` и `kubectl port-forward` в под с лейблом `deckhouse.io/system-resource: "true"` доступны только `superadmin`.

Перечисленные ограничения не действуют на администраторов кластера: у тех, кому выдана стандартная роль Kubernetes `cluster-admin` или уровень доступа `SuperAdmin` [упрощённой модели](rbac-current.html), а также у участников групп `system:masters`, `kubeadm:cluster-admins`, `superadmins` и `system:sudousers`, аварийный доступ сохраняется независимо от этих ограничений.

Для `superadmin` также есть ограничения: ресурсы, созданные из [шаблона проекта](/modules/multitenancy-manager/) (лейбл `heritage: multitenancy-manager`), не может изменить **никто**, включая `superadmin` и администратора кластера, — они управляются исключительно контроллером проектов. Роль назначается через RoleBinding и действует только в том неймспейсе, где выдана: `superadmin` одного неймспейса не получает никаких особых прав в другом.

## Встроенные механизмы защиты ролевой модели

Ролевая модель защищена набором проверок на уровне API-сервера. Они не требуют настройки и предотвращают типовые ошибки и попытки повышения привилегий:

- **Нельзя выдать ограниченную по области роль на весь кластер.** ClusterRoleBinding на роли `d8:namespace:*`, `d8:project:*` (и их `d8:custom:*`-варианты) отклоняется — иначе роль, рассчитанная на один неймспейс или проект, действовала бы во всех неймспейсах сразу. Используйте RoleBinding в нужном неймспейсе либо ProjectRoleBinding/ClusterProjectRoleBinding для проекта.
- **Нельзя выдать capability на весь кластер.** ClusterRoleBinding на любую capability отклоняется: capability — «строительный блок» для ролей, а не самостоятельная роль. В отдельном неймспейсе привязать capability через RoleBinding можно.
- **Нельзя получить управление проектами через собственную роль.** Создание Role или ClusterRole, дающей права на изменение ресурсов управления проектами (`projects`, `projecttemplates`, `projectrolebindings`, `clusterprojectrolebindings`, `projectnamespaces`), отклоняется — эти права дают только встроенные роли `d8:project:*`.
- **Нельзя смешивать пользовательскую и административную области в одной роли.** Собственная роль не может одновременно агрегировать capabilities областей `namespace`/`project` и областей `system`/`subsystem`.
- **Собственные роли не могут содержать прямых RBAC-правил** — только агрегировать capabilities.

## Отображаемые названия ролей

Каждая встроенная роль и capability имеет локализованные название и описание в аннотациях:

- `ru.meta.deckhouse.io/title`, `ru.meta.deckhouse.io/description` — на русском;
- `en.meta.deckhouse.io/title`, `en.meta.deckhouse.io/description` — на английском.

Эти аннотации использует, например, веб-интерфейс Deckhouse Console при отображении списка ролей.

Если стандартное название не подходит (например, вы хотите называть роли в терминах, принятых в компании), добавьте на роль аннотации `custom.meta.deckhouse.io/title` и `custom.meta.deckhouse.io/description` — интерфейс покажет их вместо стандартных. Пример:

```shell
d8 k annotate clusterrole d8:namespace:admin \
  custom.meta.deckhouse.io/title='Администратор команды'
```

Это единственное разрешённое изменение встроенных ролей: изменение их правил, агрегации или лейблов отклоняется.

## Устаревшие имена ролей

Прежние имена ролей гранулярной модели (`d8:manage:<подсистема>:<уровень>`, `d8:manage:all:<уровень>` и `d8:use:role:<уровень>`) устарели и будут удалены в одном из следующих релизов. Для обратной совместимости они временно сохранены как роли-псевдонимы: привязка к псевдониму агрегирует capabilities **новой** роли. Это не те же права, что до миграции: например, устаревшая роль `d8:use:role:admin` больше не даёт выпуск токена ServiceAccount и impersonate (это ушло на `superadmin`).

Соответствие имён:

| Устаревшее имя | Новое имя |
|-----------------|-----------|
| `d8:manage:all:<уровень>` | `d8:system:<уровень>` |
| `d8:manage:<подсистема>:<уровень>` | `d8:subsystem:<подсистема>:<уровень>` |
| `d8:use:role:<уровень>` | `d8:namespace:<уровень>` |
| `d8:use:role:<уровень>:kubernetes` | `d8:namespace:<уровень>` (полная namespace-роль, не kubernetes-only) |

Пока в кластере остаются привязки к устаревшим именам, срабатывают алерты `D8UserAuthzDeprecatedRBACv2RoleInUse` и `D8UserAuthzDeprecatedRBACv2CapabilityInUse`. Переведите существующие RoleBinding и ClusterRoleBinding на новые имена ролей. Найти привязки, использующие устаревшие имена, можно с помощью команды:

```bash
d8 k get clusterrolebindings,rolebindings -A -o json \
  | jq -r '.items[] | select(.roleRef.name | test("^d8:(manage|use):")) | "\(.kind) \(.metadata.namespace // "-") \(.metadata.name) -> \(.roleRef.name)"'
```

## Получение аналога ролей ClusterAdmin и SuperAdmin упрощённой модели

В гранулярной модели нет ролей, объединяющих права в одну сущность, как [`ClusterAdmin` и `SuperAdmin`](rbac-current.html) в упрощённой модели. В ней разделяется управление платформой (системные роли) и доступ к приложениям (namespace- и проектные роли). Аналог собирается из **двух привязок**: ClusterRoleBinding на системную роль и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) на проектную роль (она действует во всех проектах, включая создаваемые позже).

Примерное соответствие уровней:

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
# Платформа: конфигурация модулей DP, cluster-wide-ресурсы, системные неймспейсы.
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

Особенности:

- При включённом [автоматическом создании проектов](/modules/multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects) каждый пользовательский неймспейс является проектом, поэтому пара «системная роль + ClusterProjectRoleBinding» покрывает и платформу, и все пользовательские неймспейсы. Не покрывается только неймспейс `default` — он не относится ни к проектам, ни к системным.
- Создать собственную роль «со всеми правами» (`apiGroups: ["*"], resources: ["*"], verbs: ["*"]`) не получится: такая роль даёт, в том числе права на управление проектами и будет отклонена [встроенной защитой](./#встроенные-защиты-ролевой-модели). Если нужен именно неограниченный доступ ко всему API (вне ролевой модели платформы), используйте ClusterRoleBinding на встроенную роль Kubernetes `cluster-admin` — назначить её может только тот, у кого такие права уже есть.
