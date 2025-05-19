---
title: "Экспериментальная модель авторизации"
permalink: ru/admin/access/authorization-rbac-experimental.html
lang: ru
---

Экспериментальная ролевая модель построена на принципе агрегации: она объединяет мелкие роли в более крупные, что упрощает расширение модели за счёт собственных ролей.

Для реализации экспериментальной ролевой модели в кластере должен быть включен модуль [user-authz](../../reference/mc/user-authz/).
Модуль создает набор кластерных ролей (`ClusterRole`), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %} С версии Deckhouse Kubernetes Platform v1.64 в модуле реализована экспериментальная модель ролевого доступа. Текущая модель ролевого доступа продолжит работать, но в будущем перестанет поддерживаться.

Функциональность экспериментальной и текущей моделей ролевого доступа несовместимы. Автоматическая конвертация ресурсов невозможна. {% endalert %}

<!-- Перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/user-authz/#%D1%8D%D0%BA%D1%81%D0%BF%D0%B5%D1%80%D0%B8%D0%BC%D0%B5%D0%BD%D1%82%D0%B0%D0%BB%D1%8C%D0%BD%D0%B0%D1%8F-%D1%80%D0%BE%D0%BB%D0%B5%D0%B2%D0%B0%D1%8F-%D0%BC%D0%BE%D0%B4%D0%B5%D0%BB%D1%8C -->

В отличие [от текущей ролевой модели](../access/authorization-rbac-current.html) DKP, экспериментальная ролевая модель не использует ресурсы `ClusterAuthorizationRule` и `AuthorizationRule`. Настройка прав доступа выполняется стандартным для RBAC Kubernetes способом: с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, с указанием в них одной из ролей, подготовленных модулем `user-authz`.

Модуль создаёт специальные агрегированные кластерные роли (`ClusterRole`). Используя эти роли в `RoleBinding` или `ClusterRoleBinding` можно решать следующие задачи:

- Управлять доступом к модулям, относящимся к определённой [подсистеме](#подсистемы-ролевой-модели) платформы.

  Например, чтобы дать возможность пользователю, выполняющему функции сетевого администратора, настраивать *сетевые* модули (например, `cni-cilium`, `ingress-nginx`, `istio` и т. д.), можно использовать в `ClusterRoleBinding` роль `d8:manage:networking:manager`.
  
- Управлять доступом к *пользовательским* ресурсам модулей в рамках пространства имён.

  Например, использование роли `d8:use:role:manager` в `RoleBinding`, позволит удалять/создавать/редактировать ресурс [PodLoggingConfig](../../reference/cr/podloggingconfig/) в пространстве имён, но не даст доступа к cluster-wide-ресурсам [ClusterLoggingConfig](../../reference/cr/clusterloggingconfig/) и [ClusterLogDestination](../../reference/cr/clusterlogdestination/) модуля `log-shipper`, а также не даст возможность настраивать сам модуль `log-shipper`.

Роли, создаваемые модулем, делятся на два класса:

- [Use-роли](#use-роли) — для назначения прав пользователям (например, разработчикам приложений) **в конкретном пространстве имён**.
- [Manage-роли](#manage-роли) — для назначения прав администраторам.

## Use-роли

{% alert level="warning" %}
Use-роль можно использовать только в ресурсе `RoleBinding`.
{% endalert %}

Use-роли предназначены для назначения прав пользователю **в конкретном пространстве имён**. Под пользователями понимаются, например, разработчики, которые используют настроенный администратором кластер для развёртывания своих приложений. Таким пользователям не нужно управлять модулями DKP или кластером, но им нужно иметь возможность, например, создавать свои Ingress-ресурсы, настраивать аутентификацию приложений и сбор логов с приложений.

Use-роль определяет права на доступ к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes (`Pod`, `Deployment`, `Secret`, `ConfigMap` и т. п.).

Модуль [user-authz](../../reference/mc/user-authz/) создаёт следующие use-роли:

- `d8:use:role:viewer` — позволяет в конкретном пространстве имён просматривать стандартные ресурсы Kubernetes, кроме секретов и ресурсов RBAC, а также выполнять аутентификацию в кластере;
- `d8:use:role:user` — дополнительно к роли `d8:use:role:viewer` позволяет в конкретном пространстве имён просматривать секреты и ресурсы RBAC, подключаться к подам, удалять поды (но не создавать или изменять их), выполнять `kubectl port-forward` и `kubectl proxy`, изменять количество реплик контроллеров;
- `d8:use:role:manager` — дополнительно к роли `d8:use:role:user` позволяет в конкретном пространстве имён управлять ресурсами модулей (например, `Certificate`, `PodLoggingConfig` и т. п.) и стандартными namespaced-ресурсами Kubernetes (`Pod`, `ConfigMap`, `CronJob` и т. п.);
- `d8:use:role:admin` — дополнительно к роли `d8:use:role:manager` позволяет в конкретном пространстве имён управлять ресурсами `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy`.

Таблица РАЗ

| Роль                 | Описание                                                                                                             |
|----------------------|----------------------------------------------------------------------------------------------------------------------|
| **`d8:use:role:viewer`**  | Просмотр стандартных ресурсов Kubernetes в конкретном пространстве имён, кроме секретов и ресурсов RBAC. Выполнение аутентификации в кластере. |
| **`d8:use:role:user`**    | Все от `d8:use:role:viewer` + просмотр секретов и ресурсов RBAC, подключение к подам, удаление подов, выполнение `kubectl port-forward` и `kubectl proxy`, изменение количества реплик контроллеров в конкретном пространстве имён. |
| **`d8:use:role:manager`** | Все от `d8:use:role:user` + управление ресурсами модулей и стандартными namespaced-ресурсами Kubernetes в конкретном пространстве имён. |
| **`d8:use:role:admin`**   | Все от `d8:use:role:manager` + управление ресурсами `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy` в конкретном пространстве имён. |

Таблица ДВА

| Роль                     | Доступные действия                                                                                     | Ограничения                                  |
|--------------------------|-------------------------------------------------------------------------------------------------------|---------------------------------------------|
| **d8:use:role:viewer**   | Просмотр Pod, Deployment, Service (кроме секретов и RBAC)                                              | Нет доступа к exec, портам, изменению ресурсов |
| **d8:use:role:user**    | + Чтение секретов, `kubectl exec`, `port-forward`, удаление подов, масштабирование реплик             | Не может создавать/редактировать объекты    |
| **d8:use:role:manager** | + Создание/изменение Pod, ConfigMap, Deployment, управление ресурсами модулей (например, Certificate)  | Нет доступа к Quota и RBAC                  |
| **d8:use:role:admin**   | + Управление ServiceAccount, Role, ResourceQuota, NetworkPolicy                                       | Полный доступ в пределах namespace          |

Ключевые отличия ролей:
- `viewer` — только "read-only" (без секретов и RBAC).
- `user` — добавляет доступ к секретам, подам и сетевым функциям (port-forward).
- `manager` — разрешает управлять ресурсами приложений и модулей.
- `admin` — полный контроль над namespace (включая RBAC и квоты).

## Manage-роли

{% alert level="warning" %}
Manage-роль не дает доступа к пространству имён пользовательских приложений.

Manage-роль определяет доступ только к системным пространствам имён (начинающимся с `d8-` или `kube-`), и только к тем из них, в которых работают модули соответствующей подсистемы роли.
{% endalert %}

Manage-роли предназначены для назначения прав на управление всей платформой или её частью ([подсистемой](#подсистемы-ролевой-модели)), но не самими приложениями пользователей. С помощью manage-роли можно, например, дать возможность администратору безопасности управлять модулями, ответственными за функции безопасности кластера. Тогда администратор безопасности сможет настраивать аутентификацию, авторизацию, политики безопасности и т. п., но не сможет управлять остальными функциями кластера (например, настройками сети и мониторинга) и изменять настройки в пространстве имён приложений пользователей.

Manage-роль определяет права на доступ:

- к cluster-wide-ресурсам Kubernetes;
- к управлению модулями DKP (ресурсы `moduleConfig`) в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми модулями DKP для роли `d8:manage:all:*`;
- к управлению cluster-wide-ресурсами модулей DKP в рамках [подсистемы](#подсистемы-ролевой-модели) роли или всеми ресурсами модулей DKP для роли `d8:manage:all:*`;
- к системным пространствам имён (начинающимся с `d8-` или `kube-`), в которых работают модули [подсистемы](#подсистемы-ролевой-модели) роли, или ко всем системным пространствам имён для роли `d8:manage:all:*`.
  
Формат названия manage-роли — `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:

- `SUBSYSTEM` — подсистема роли. Может быть либо одной из подсистем [списка](#подсистемы-ролевой-модели), либо `all` для доступа в рамках всех подсистем;
- `ACCESS_LEVEL` — уровень доступа.

  Примеры manage-ролей:
  
  - `d8:manage:all:viewer` — доступ на просмотр конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:manage:all:manager` — аналогично роли `d8:manage:all:viewer`, только доступ на уровне `admin`, т. е. просмотр/создание/изменение/удаление конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:manage:observability:viewer` — доступ на просмотр конфигурации модулей DKP (ресурсы `moduleConfig`) из подсистемы `observability`, их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в системных пространствах имён `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

Таблица РАЗ

| Роль                             | Описание                                                                                                                |
|----------------------------------|-------------------------------------------------------------------------------------------------------------------------|
| **d8:manage:all:viewer**         | Доступ на просмотр конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide и namespaced-ресурсов, а также стандартных объектов Kubernetes (без секретов и ресурсов RBAC) во всех системных пространствах имён (начинающихся с `d8-` или `kube-`). |
| **d8:manage:all:manager**        | Все от `d8:manage:all:viewer` с доступом уровня `admin`: просмотр, создание, изменение и удаление конфигураций всех модулей DKP, их cluster-wide, namespaced-ресурсов и стандартных объектов Kubernetes во всех системных пространствах имён.            |
| **d8:manage:observability:viewer** | Доступ на просмотр конфигурации модулей DKP из подсистемы `observability` (ресурсы `moduleConfig`), их cluster-wide и namespaced-ресурсов, и стандартных объектов Kubernetes (без секретов и ресурсов RBAC) в системных пространствах имён, связанных с наблюдаемостью.

Таблица ДВА

| Роль                              | Действия       | Модули          | Пространства имён                     | Ограничения       |
|-----------------------------------|---------------|-----------------|---------------------------------------|------------------|
| **d8:manage:all:viewer**         | Чтение        | Все             | `d8-*`, `kube-*`                     | Секреты, RBAC    |
| **d8:manage:all:manager**        | Полный доступ | Все             | `d8-*`, `kube-*`                     | Нет              |
| **d8:manage:observability:viewer** | Чтение        | Только observability | `d8-monitoring`, `d8-okmeter` и др. | Секреты, RBAC    |

Модуль предоставляет два уровня доступа для администратора:

- `viewer` — позволяет просматривать стандартные ресурсы Kubernetes, конфигурацию модулей (ресурсы `moduleConfig`), cluster-wide-ресурсы модулей и namespaced-ресурсы модулей в пространстве имен модуля;
- `manager` — дополнительно к роли `viewer` позволяет управлять стандартными ресурсами Kubernetes, конфигурацией модулей (ресурсы `moduleConfig`), cluster-wide-ресурсами модулей и namespaced-ресурсами модулей в пространстве имен модуля;

## Подсистемы ролевой модели

Каждый модуль DKP принадлежит определённой подсистеме. Для каждой подсистемы существует набор ролей с разными уровнями доступа. Роли обновляются автоматически при включении или отключении модуля.

Например, для подсистемы `networking` существуют следующие manage-роли, которые можно использовать в `ClusterRoleBinding`:

- `d8:manage:networking:viewer`
- `d8:manage:networking:manager`

Подсистема роли ограничивает её действие всеми системными (начинающимися с `d8-` или `kube-`) пространствами имён кластера (подсистема `all`) или теми пространствами имён, в которых работают модули подсистемы (см. таблицу состава подсистем).

Таблица состава подсистем ролевой модели.

{% include rbac/rbac-subsystems-list.liquid %}

## Создание новой роли подсистемы

Предположим, что текущие подсистемы не подходят под ролевое распределение в компании и требуется создать новую [подсистему](#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `deckhouse`, подсистемы `kubernetes` и модуля `user-authn`.

Для решения этой задачи создайте следующую роль:

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

В начале указаны лейблы для новой роли:

- показывает, какую роль хук должен использовать при создании use ролей:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- показывает, что роль должна обрабатываться как manage-роль:

  ```yaml
  rbac.deckhouse.io/kind: manage
  ```

  > Этот лейбл обязателен.

- показывает, что роль является ролью подсистемы, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/level: subsystem
  ```

- указывает подсистему, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/subsystem: custom
  ```

- позволяет `manage:all`-роли агрегировать эту роль в себя:

  ```yaml
  rbac.deckhouse.io/aggregate-to-all-as: manager
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль менеджера из подсистемы `deckhouse`:

  ```yaml
  rbac.deckhouse.io/kind: manage
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- агрегирует все правила от модуля user-authn:

  ```yaml
   rbac.deckhouse.io/kind: manage
   module: user-authn
  ```

Таким образом роль получает права от подсистем `deckhouse`, `kubernetes` и от модуля user-authn.

Особенности:

* ограничений на имя роли нет, но для читаемости лучше использовать этот стиль;
* use-роли будут созданы в пространстве имён агрегированных подсистем и модуля, тип роли выбран лейблом.

## Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для manage-роли) CRD-объект — MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом.

Сначала дополните роль новым селектором:

```yaml
rbac.deckhouse.io/kind: manage
rbac.deckhouse.io/aggregate-to-custom-as: manager
```

Этот селектор позволит агрегировать роли к новой подсистеме через указание этого лейбла. После добавления нового селектора роль будет выглядеть так:

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

 Затем создайте новую роль, в которой следует определить права для нового ресурса. Например, только чтение:

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

Особенности:

* ограничений на имя роли нет, но для читаемости лучше использовать этот стиль.

## Расширение существующих manage subsystem-ролей

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

## Расширение manage subsystem-ролей с добавлением нового пространства имён

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

Хук мониторит `ClusterRoleBinding` и при создании биндинга просматривает все manage-роли, чтобы найти все объединенные в них роли с помощью проверки правила агрегации. Затем он берёт пространство имён из лейбла `rbac.deckhouse.io/namespace` и создает use-роль в этом пространстве имён.

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
