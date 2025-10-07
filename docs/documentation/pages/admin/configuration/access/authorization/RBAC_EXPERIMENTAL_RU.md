---
title: "Экспериментальная модель авторизации"
permalink: ru/admin/configuration/access/authorization/rbac-experimental.html
lang: ru
---

Экспериментальная ролевая модель построена на принципе агрегации: она объединяет низкоуровневые роли в более крупные, охватывающие типовые задачи. Это упрощает расширение модели за счёт добавления собственных ролей.

Для реализации экспериментальной ролевой модели в кластере должен быть включен модуль [`user-authz`](/modules/user-authz/).
Модуль создает набор кластерных ролей (ClusterRole), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %} С версии Deckhouse Kubernetes Platform (DKP) v1.64 в модуле реализована экспериментальная модель ролевого доступа. Текущая модель ролевого доступа продолжит работать, но в будущем перестанет поддерживаться.

Функциональность экспериментальной и текущей моделей несовместимы. Автоматическая конвертация ресурсов невозможна. {% endalert %}

<!-- Перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/user-authz/#%D1%8D%D0%BA%D1%81%D0%BF%D0%B5%D1%80%D0%B8%D0%BC%D0%B5%D0%BD%D1%82%D0%B0%D0%BB%D1%8C%D0%BD%D0%B0%D1%8F-%D1%80%D0%BE%D0%BB%D0%B5%D0%B2%D0%B0%D1%8F-%D0%BC%D0%BE%D0%B4%D0%B5%D0%BB%D1%8C -->

В отличие [от текущей ролевой модели](rbac-current.html) DKP, экспериментальная ролевая модель не использует ресурсы [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Права доступа настраиваются стандартным способом Kubernetes RBAC: через ресурсы [RoleBinding или ClusterRoleBinding](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#rolebinding-and-clusterrolebinding), в которых указывается одна из ролей, созданных модулем `user-authz`.

Модуль создаёт специальные агрегированные кластерные роли (ClusterRole). Используя эти роли в [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) или [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) можно решать следующие задачи:

- Управлять доступом к модулям, относящимся к определённой [подсистеме](#подсистемы-ролевой-модели) платформы.

  Например, чтобы дать возможность пользователю, выполняющему функции сетевого администратора, настраивать *сетевые* модули (например, [`cni-cilium`](/modules/cni-cilium/), [`ingress-nginx`](/modules/ingress-nginx/), [`istio`](/modules/istio/) и т. д.), можно использовать в ClusterRoleBinding роль `d8:manage:networking:manager`.
  
- Управлять доступом к *пользовательским* ресурсам модулей в рамках пространства имён.

  Например, использование роли `d8:use:role:manager` в RoleBinding, позволит удалять/создавать/редактировать ресурс [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) в пространстве имён, но не даст доступ к таким ресурсам на уровне кластера, как [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) и [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) модуля `log-shipper`, а также не даст возможность настраивать сам модуль `log-shipper`.

Роли, создаваемые модулем, делятся на два класса:

- [Use-роли](#use-роли) — для назначения прав пользователям (например, разработчикам приложений) **в конкретном пространстве имён**.
- [Manage-роли](#manage-роли) — для назначения прав администраторам.

## Use-роли

{% alert level="warning" %}
Use-роль можно использовать только в ресурсе [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/).
{% endalert %}

Use-роли предназначены для назначения прав пользователю **в конкретном пространстве имён**. Пользователями считаются, например, разработчики, использующие настроенный администратором кластер для развёртывания своих приложений. Таким пользователям не нужно управлять модулями DKP или кластером, но им нужно иметь возможность, например, создавать свои Ingress-ресурсы, настраивать аутентификацию приложений и сбор логов с приложений.

Use-роль определяет права на доступ к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes (Pod, Deployment, Secret, ConfigMap и т. п.).

Модуль [`user-authz`](/modules/user-authz/) создаёт следующие use-роли:

| Роль                     | Доступные действия                                                                                     | Ограничения доступа                            |
|--------------------------|-------------------------------------------------------------------------------------------------------|------------------------------------------------|
| `d8:use:role:viewer`   | Просмотр Pod, Deployment, Service (кроме секретов и RBAC)                                              | Нет доступа к `exec`, портам, изменению ресурсов |
| `d8:use:role:user`    | Чтение секретов, `kubectl exec`, `port-forward`, удаление подов, масштабирование реплик             | Не может создавать/редактировать объекты       |
| `d8:use:role:manager` | Создание/изменение Pod, ConfigMap, Deployment, управление ресурсами модулей (например, Certificate)  | Нет доступа к Quota и RBAC                     |
| `d8:use:role:admin`   | Управление ServiceAccount, Role, ResourceQuota, NetworkPolicy                                       | Полный доступ в пределах пространства имён             |

Ключевые отличия ролей:

- `viewer` — только `read-only` (без секретов и RBAC);
- `user` — добавляет доступ к секретам, подам и сетевым функциям (port-forward);
- `manager` — разрешает управлять ресурсами приложений и связанными ресурсами модулей;
- `admin` — полный контроль над пространством имён (включая RBAC и квоты).

## Manage-роли

{% alert level="warning" %}
Manage-роли не предоставляют доступ к пространствам имён пользовательских приложений.

Manage-роль определяет доступ только к системным пространствам имён (начинающимся с `d8-` или `kube-`), и только к тем из них, в которых работают модули соответствующей подсистемы роли.
{% endalert %}

Manage-роли предназначены для назначения прав на управление всей платформой или её частью ([подсистемой](#подсистемы-ролевой-модели)), но не самими приложениями пользователей. С помощью manage-роли можно, например, дать возможность администратору безопасности управлять модулями, ответственными за функции безопасности кластера. Такой администратор сможет управлять аутентификацией, авторизацией и политиками безопасности, но не получит доступ к другим частям кластера (например, сетевой подсистеме или мониторингу) и не сможет менять настройки в пространствах имён приложений.

Manage-роль определяет права на доступ:

- к ресурсам Kubernetes на уровне кластера;
- к управлению модулями DKP (ресурсы ModuleConfig) в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми модулями DKP для роли `d8:manage:all:*`;
- к управлению ресурсами модулей DKP на уровне кластера в рамках [подсистемы](#подсистемы-ролевой-модели) роли или всеми ресурсами модулей DKP для роли `d8:manage:all:*`;
- к системным пространствам имён (начинающимся с `d8-` или `kube-`), в которых работают модули [подсистемы](#подсистемы-ролевой-модели) роли, или ко всем системным пространствам имён для роли `d8:manage:all:*`.
  
Формат названия manage-роли — `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:

- `SUBSYSTEM` — подсистема роли. Может быть либо одной из подсистем [списка](#подсистемы-ролевой-модели), либо `all` для доступа в рамках всех подсистем;
- `ACCESS_LEVEL` — уровень доступа.

Примеры manage-ролей:
  
- `d8:manage:all:viewer` — доступ на просмотр конфигурации всех модулей DKP (ресурсы ModuleConfig), их ресурсов на уровне кластера, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
- `d8:manage:all:manager` — аналогично роли `d8:manage:all:viewer`, только доступ на уровне `admin`, т. е. просмотр/создание/изменение/удаление конфигурации всех модулей DKP (ресурсы ModuleConfig), их ресурсов на уровне кластера, их namespaced-ресурсов и стандартных объектов Kubernetes во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
- `d8:manage:observability:viewer` — доступ на просмотр конфигурации модулей DKP (ресурсы ModuleConfig) из подсистемы `observability`, их ресурсов на уровне кластера, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в системных пространствах имён `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

Модуль предоставляет два уровня доступа для администратора:

- `viewer` — позволяет просматривать стандартные ресурсы Kubernetes, конфигурацию модулей (ресурсы ModuleConfig), ресурсы модулей на уровне кластера и namespaced-ресурсы модулей в пространстве имен модуля;
- `manager` — дополнительно к роли `viewer` позволяет управлять стандартными ресурсами Kubernetes, конфигурацией модулей (ресурсы ModuleConfig), ресурсами модулей на уровне кластера и namespaced-ресурсами модулей в пространстве имен модуля.

## Подсистемы ролевой модели

Каждый модуль DKP принадлежит определённой подсистеме. Для каждой подсистемы существует набор ролей с разными уровнями доступа. Роли обновляются автоматически при включении или отключении модуля.

Например, для подсистемы `networking` существуют следующие manage-роли, которые можно использовать в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/):

- `d8:manage:networking:viewer`;
- `d8:manage:networking:manager`.

Подсистема роли ограничивает её действие всеми системными (начинающимися с `d8-` или `kube-`) пространствами имён кластера (подсистема `all`) или теми пространствами имён, в которых работают модули подсистемы (см. таблицу состава подсистем).

### Состав подсистем ролевой модели

{% include rbac/rbac-subsystems-list.liquid %}

## Создание новой роли подсистемы

Если текущие подсистемы не соответствуют требованиям ролевого распределения в компании, можно создать новую [подсистему](#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистем `deckhouse`, `kubernetes` и [модуля `user-authn`](/modules/user-authn/).

Для этого создайте следующую роль:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: subsystem
    rbac.deckhouse.io/subsystem: custom
    rbac.deckhouse.io/aggregate-to-all-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        module: user-authn
rules: []
```

### Описание лейблов

Созданная роль содержит следующие лейблы:

- `rbac.deckhouse.io/use-role: admin` — указывает, какую роль хук должен использовать при создании use-ролей;
- `rbac.deckhouse.io/kind: manage` — определяет, что роль относится к типу `manage`. **Этот лейбл обязателен**;
- `rbac.deckhouse.io/level: subsystem` — указывает, что роль относится к уровню подсистемы и будет обрабатываться соответствующим образом;
- `rbac.deckhouse.io/subsystem: custom` — задаёт имя подсистемы, за которую отвечает эта роль;
- `rbac.deckhouse.io/aggregate-to-all-as: manager` — позволяет `manage:all`-роли агрегировать эту роль как `manager`.

### Описание селекторов агрегации

Секция `aggregationRule` описывает, какие роли и модули агрегируются в данную роль:

- `rbac.deckhouse.io/kind: manage`, `rbac.deckhouse.io/aggregate-to-deckhouse-as: manager` — агрегирует manage-роль из подсистем `deckhouse`, `kubernetes`.
- `rbac.deckhouse.io/kind: manage`, `module: user-authn` — агрегирует все правила из модуля `user-authn`.

Таким образом роль получает права от подсистем `deckhouse`, `kubernetes` и от [модуля `user-authn`](/modules/user-authn/).

{% alert level="info" %}

- Ограничений на имя роли нет, но рекомендуется придерживаться читаемого и унифицированного стиля.
- Use-роли будут автоматически созданы в пространствах имён соответствующих подсистем и модуля.
  Тип роли определяется по лейблу.
  
{% endalert %}

## Расширение пользовательской роли

Допустим, в кластере появился новый кластерный CRD-объект MySuperResource. Чтобы выдать права на взаимодействие с ним, необходимо дополнить существующую manage-роль, описанную выше.

1. Сначала дополните роль новым селектором:

   ```yaml
   rbac.deckhouse.io/kind: manage
   rbac.deckhouse.io/aggregate-to-custom-as: manager
   ```

   Этот селектор позволяет включать роль в агрегирующую роль подсистемы через соответствующий лейбл. После добавления нового селектора роль будет выглядеть так:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: custom:manage:mycustom:manager
     labels:
       rbac.deckhouse.io/use-role: admin
       rbac.deckhouse.io/kind: manage
       rbac.deckhouse.io/level: subsystem
       rbac.deckhouse.io/subsystem: custom
       rbac.deckhouse.io/aggregate-to-all-as: manager
   aggregationRule:
     clusterRoleSelectors:
       - matchLabels:
           rbac.deckhouse.io/kind: manage
           rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
       - matchLabels:
           rbac.deckhouse.io/kind: manage
           rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
       - matchLabels:
           rbac.deckhouse.io/kind: manage
           module: user-authn
       - matchLabels:
           rbac.deckhouse.io/kind: manage
           rbac.deckhouse.io/aggregate-to-custom-as: manager
   rules: []
   ```

1. Создайте новую роль, в которой следует определить права для нового ресурса. Например, только чтение:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     labels:
       rbac.deckhouse.io/aggregate-to-custom-as: manager
       rbac.deckhouse.io/kind: manage
     name: custom:manage:permission:mycustom:superresource:view
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

   Роль дополнит своими правами роль подсистемы, дав права на просмотр нового объекта.

{% alert level="info" %}
Имя роли можно задать произвольно, но для удобства рекомендуется придерживаться принятого стиля.
{% endalert %}

## Расширение существующих manage-subsystem-ролей

Если необходимо расширить существующую роль, выполните те же шаги, что и в пункте выше, но изменив лейблы и название роли.

Пример для расширения роли менеджера из подсистемы `deckhouse`(`d8:manage:deckhouse:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: manage
  name: custom:manage:permission:mycustommodule:superresource:view
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

Таким образом новая роль расширит роль `d8:manage:deckhouse`.

## Расширение manage-subsystem-ролей с добавлением нового пространства имён

Если необходимо добавить новое пространство имён (для создания в нём use-роли с помощью хука), потребуется добавить лишь один лейбл:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

Этот лейбл сообщает хуку, что в этом пространстве имён нужно создать use-роль:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/namespace: namespace
  name: custom:manage:permission:mycustom:superresource:view
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

Хук отслеживает ресурсы [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) и при их создании проверяет manage-роли, чтобы найти все агрегированные роли по правилу `aggregationRule`. Из каждой из них он извлекает пространство имён из лейбла `rbac.deckhouse.io/namespace` и создаёт use-роль в этом пространстве.

## Расширение существующих use-ролей

Если ресурс принадлежит пространству имён, необходимо расширить use-роль вместо manage-роли. Разница лишь в лейблах и имени:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-kubernetes-as: user
    rbac.deckhouse.io/kind: use
  name: custom:use:capability:mycustom:superresource:view
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

Эта роль дополнит роль `d8:use:role:user:kubernetes`.
