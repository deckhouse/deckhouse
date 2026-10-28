---
title: "Модуль user-authz: примеры конфигурации"
description: "Примеры выдачи доступа ролями и объектами модуля user-authz: администраторы, неймспейсы, проекты, ClusterAuthorizationRule, ServiceAccount'ы и клиентские сертификаты."
---

## Пример назначения прав администратору кластера

{% alert level="info" %}
Пример использует [гранулярную ролевую модель](./#гранулярная-ролевая-модель).
{% endalert %}

Для назначения прав администратору кластера используйте роль `d8:system:manager` в ClusterRoleBinding.

Пример назначения прав администратору кластера (User `jane.doe@example.com`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-admin-jane
subjects:
- kind: User
  name: jane.doe@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:system:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Права, которые получит пользователь" %}
Пользователь получит доступ к cluster-wide-ресурсам, к конфигурации модулей Deckhouse Platform и к неймспейсам включённых модулей Deckhouse Platform. Доступа к неймспейсам пользовательских приложений у него не будет.

Пользователю будут доступны следующие права:
- Просмотр, изменение, удаление и создание ресурсов Kubernetes и модулей Deckhouse Platform.
- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов `moduleConfig`).
- Выполнение следующих команд к подам и сервисам:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}

## Пример назначения прав сетевому администратору

{% alert level="info" %}
Пример использует [гранулярную ролевую модель](./#гранулярная-ролевая-модель).
{% endalert %}

Для назначения прав сетевому администратору на управление сетевой подсистемой кластера используйте роль `d8:subsystem:network:manager` в ClusterRoleBinding.

Пример назначения прав сетевому администратору (User `jane.doe@example.com`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: network-admin-jane
subjects:
- kind: User
  name: jane.doe@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:subsystem:network:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Список прав, которые получит пользователь" %}
Права, которые получит пользователь, будут ограничены следующим списком неймспейсов модулей Deckhouse Platform из подсистемы `network` (фактический список зависит от списка включённых в кластере модулей):
- `d8-cni-cilium`;
- `d8-cni-flannel`;
- `d8-cni-simple-bridge`;
- `d8-ingress-nginx`;
- `d8-istio`;
- `d8-keepalived`;
- `d8-metallb`;
- `d8-monitoring`;
- `d8-network-gateway`;
- `d8-openvpn`;
- `d8-service-with-healthchecks`;
- `d8-static-routing-manager`;
- `d8-system`;
- `kube-system`.

Пользователю будут доступны следующие права:
- Просмотр, изменение, удаление и создание *стандартных* ресурсов Kubernetes в неймспейсах модулей из подсистемы `network`.

  Пример ресурсов, которыми сможет управлять пользователь (список не полный):
  - `Certificate`;
  - `CertificateRequest`;
  - `ConfigMap`;
  - `ControllerRevision`;
  - `CronJob`;
  - `DaemonSet`;
  - `Deployment`;
  - `Event`;
  - `HorizontalPodAutoscaler`;
  - `Ingress`;
  - `Issuer`;
  - `Job`;
  - `Lease`;
  - `LimitRange`;
  - `NetworkPolicy`;
  - `PersistentVolumeClaim`;
  - `Pod`;
  - `PodDisruptionBudget`;
  - `ReplicaSet`;
  - `ReplicationController`;
  - `ResourceQuota`;
  - `Role`;
  - `RoleBinding`;
  - `Secret`;
  - `Service`;
  - `ServiceAccount`;
  - `StatefulSet`;
  - `VerticalPodAutoscaler`;
  - `VolumeSnapshot`.

- Просмотр, изменение, удаление и создание ресурсов в неймспейсах модулей из подсистемы `network`.

  Список ресурсов, которыми сможет управлять пользователь:
  - `EgressGateway`;
  - `EgressGatewayPolicy`;
  - `FlowSchema`;
  - `IngressClass`;
  - `IngressIstioController`;
  - `IngressNginxController`;
  - `IPRuleSet`;
  - `IstioFederation`;
  - `IstioMulticluster`;
  - `KeepalivedInstance`;
  - `RoutingTable`.

- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов moduleConfig) из подсистемы `network`.

  Список модулей, которыми сможет управлять пользователь:
  - `cilium-hubble`;
  - `cni-cilium`;
  - `cni-flannel`;
  - `cni-simple-bridge`;
  - `ingress-nginx`;
  - `istio`;
  - `keepalived`;
  - `kube-dns`;
  - `kube-proxy`;
  - `metallb`;
  - `monitoring-ping`;
  - `network-gateway`;
  - `network-policy-engine`;
  - `node-local-dns`;
  - `openvpn`;
  - `sdn`;
  - `service-with-healthchecks`;
  - `static-routing-manager`.

- Выполнение следующих команд к подам и сервисам в неймспейсах модулей из подсистемы `network`:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}

## Пример назначения административных прав пользователю в рамках неймспейса

{% alert level="info" %}
Пример использует [гранулярную ролевую модель](./#гранулярная-ролевая-модель).
{% endalert %}

Для назначения прав на управление ресурсами приложений в неймспейсе (включая некоторые namespaced-ресурсы модулей Deckhouse Platform, например DexAuthenticator) используйте роль `d8:namespace:admin` в RoleBinding в соответствующем неймспейсе. Кластерная настройка модулей в эту роль не входит.

Пример назначения прав разработчику приложений (User `app-developer`) в неймспейсе `myapp`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: myapp-developer
  namespace: myapp
subjects:
- kind: User
  name: app-developer@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:namespace:admin
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Список прав, которые получит пользователь" %}
В рамках неймспейса `myapp` пользователю будут доступны следующие права:

- Просмотр, изменение, удаление и создание ресурсов Kubernetes. Например, следующих ресурсов (список не полный):
  - `Certificate`;
  - `CertificateRequest`;
  - `ConfigMap`;
  - `ControllerRevision`;
  - `CronJob`;
  - `DaemonSet`;
  - `Deployment`;
  - `Event`;
  - `HorizontalPodAutoscaler`;
  - `Ingress`;
  - `Issuer`;
  - `Job`;
  - `Lease`;
  - `LimitRange`;
  - `NetworkPolicy`;
  - `PersistentVolumeClaim`;
  - `Pod`;
  - `PodDisruptionBudget`;
  - `ReplicaSet`;
  - `ReplicationController`;
  - `ResourceQuota`;
  - `Role`;
  - `RoleBinding`;
  - `Secret`;
  - `Service`;
  - `ServiceAccount`;
  - `StatefulSet`;
  - `VerticalPodAutoscaler`;
  - `VolumeSnapshot`.
- Просмотр, изменение, удаление и создание следующих ресурсов модулей Deckhouse Platform:
  - `DexAuthenticator`;
  - `DexClient`;
  - `PodLoggingConfig`.
- Выполнение следующих команд к подам и сервисам:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.

Пользователю **не** будут доступны операции, зарезервированные за уровнем `superadmin` (подробнее — в разделе [«Ограничения уровня admin и права superadmin»](./#ограничения-уровня-admin-и-права-superadmin)): выпуск токенов ServiceAccount'ов, выполнение запросов от имени ServiceAccount'ов, изменение системных ресурсов платформы в этом неймспейсе и подключение к системным подам.
{% endofftopic %}

## Пример назначения прав на все неймспейсы проекта

{% alert level="info" %}
Пример использует [гранулярную ролевую модель](./#гранулярная-ролевая-модель) и модуль [multitenancy-manager](/modules/multitenancy-manager/).
{% endalert %}

Если неймспейсы объединены в [проект](/modules/multitenancy-manager/), роль можно выдать сразу на весь проект — она автоматически будет действовать во всех его неймспейсах, включая создаваемые позже. Для этого вместо RoleBinding используйте [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) в основном неймспейсе проекта:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: ProjectRoleBinding
metadata:
  name: team-developers
  namespace: my-project
spec:
  subjects:
    - kind: Group
      name: developers
  roleRef:
    kind: ClusterRole
    name: d8:project:user
```

Подробнее о привязках ролей в проектах — [в документации модуля multitenancy-manager](/modules/multitenancy-manager/usage.html#предоставление-доступа-внутри-проекта).

## Пример ClusterAuthorizationRule

{% alert level="info" %}
Пример использует [упрощённую ролевую модель](./#упрощённая-ролевая-модель).
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test-rule
spec:
  subjects:
  - kind: User
    name: some@example.com
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: ci
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  # Опция доступна только при включенном режиме enableMultiTenancy.
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: stage
        operator: In
        values:
        - test
        - review
      matchLabels:
        team: frontend
```

Субъект типа `User` или `Group` сопоставляется по имени с идентичностью пользователя из токена. При этом локально управляемая идентичность не отличается от полученной от внешнего провайдера аутентификации. По этой причине нельзя создать ресурс [User](/modules/user-authn/cr.html#user) со значением `spec.email` или [Group](/modules/user-authn/cr.html#group) со значением `spec.name`, совпадающим с субъектом существующего правила ClusterAuthorizationRule или AuthorizationRule. Это предотвращает незаметное предоставление прав идентичности.

Если совпадение сделано намеренно, например, если правило ClusterAuthorizationRule написано заранее, установите на User или Group аннотацию `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"`. Аннотация только подтверждает совпадение имени. Она не позволяет назначить роли, которые запрашивающий не покрывает или которые вне его диапазона can-assign.

Удаление ClusterAuthorizationRule проходит ту же проверку can-assign, что создание и обновление. Снять грант можно только если эти роли можно назначить. Обновление проверяется и по прежним, и по новым грантам: снизить `SuperAdmin` до `User` можно только если `SuperAdmin` можно назначить.

Для субъектов типа `User` указывайте email в нижнем регистре. Он записывается в токен в нижнем регистре, поэтому, например, субъект `Admin@Example.com` не совпадёт с `admin@example.com`. Имена субъектов типа `Group` сравниваются точно, поскольку имена групп не приводятся к нижнему регистру.

Ограничение действует при создании User или Group и при изменении их `spec.email` или `spec.name`. Удаление User или Group, чьё имя всё ещё получает права от правила, разрешено с предупреждением: имя остаётся в правиле, и повторное создание объекта возвращает права. Ограничение не запрещает добавлять существующих пользователей и группы в ClusterAuthorizationRule или AuthorizationRule. Если соответствующий User или Group уже существует, после добавления его в правило права будут предоставлены сразу.

## Управление правами через CLI

{% alert level="info" %}
Команда `d8 iam access` использует [упрощённую ролевую модель](./#упрощённая-ролевая-модель).
{% endalert %}

Команда [`d8 iam access`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-access) позволяет управлять правилами авторизации без написания YAML-манифестов. `d8 iam access grant` создаёт объекты [ClusterAuthorizationRule](cr.html#clusterauthorizationrule) и [AuthorizationRule](cr.html#authorizationrule), а `d8 iam access revoke` удаляет только объекты, созданные командой `grant`.

Команда работает с базовой ролевой моделью и только с пользователями и группами. Роли [гранулярной модели](./#гранулярная-ролевая-модель) выдавайте объектами RoleBinding и ClusterRoleBinding. Доступ для ServiceAccount выдавайте манифестом, см. [Создание ServiceAccount для сервера и предоставление ему доступа](#создание-serviceaccount-для-сервера-и-предоставление-ему-доступа). Команды `d8 iam get user` и `d8 iam list users` учитывают только объекты ClusterAuthorizationRule и AuthorizationRule. Чтобы увидеть весь доступ субъекта, используйте [SubjectAccessReport](faq.html#как-узнать-что-разрешено-конкретному-пользователю-группе-или-serviceaccountу).

Доступные уровни доступа: `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin`, `SuperAdmin`.

Примеры назначения прав с помощью команды `d8 iam access`:

Назначить роль Admin пользователю в неймспейсе `dev`:

```shell
d8 iam access grant user anton --access-level Admin -n dev
```

Назначить пользователю роль Admin в нескольких неймспейсах:

```shell
d8 iam access grant user anton --access-level Admin -n dev -n stage
```

Назначить пользователю кластерную роль (без системных неймспейсов):

```shell
d8 iam access grant user anton --access-level ClusterAdmin --scope cluster
```

Назначить пользователю кластерную роль во всех неймспейсах (включая системные):

```shell
d8 iam access grant user anton --access-level ClusterAdmin --scope all-namespaces
```

Назначить членам группы роль по селектору лейблов неймспейсов:

```shell
d8 iam access grant group admins --access-level Editor --scope labels=team=platform,tier=prod
```

`--scope cluster` исключает системные неймспейсы только в [режиме multi-tenancy](configuration.html#parameters-enablemultitenancy); без него правило выдаёт уровень доступа во всех неймспейсах, включая системные. `--scope labels=...` и `--scope all-namespaces` требуют режима multi-tenancy: без него правило отклоняется. Правило, созданное с `-n`, — это AuthorizationRule, оно действует только в своём неймспейсе.

Назначить членам группы роль с дополнительными возможностями:

```shell
d8 iam access grant group admins --access-level Editor -n dev --port-forwarding --allow-scale
```

Предпросмотр манифеста без применения:

```shell
d8 iam access grant user anton --access-level Admin -n dev --dry-run -o yaml
```

Отозвать права пользователя в неймспейсе:

```shell
d8 iam access revoke user anton --access-level Admin -n dev
```

Отозвать права пользователя на уровне кластера:

```shell
d8 iam access revoke user anton --access-level ClusterAdmin --scope cluster
```

Уровень доступа и область должны совпадать с исходной выдачей.

Посмотреть все правила доступа в кластере:

```shell
d8 iam list rules
```

Посмотреть детальную информацию о конкретном правиле. ClusterAuthorizationRule указывается как `CAR/<NAME>`, AuthorizationRule — как `AR/<NAMESPACE>/<NAME>`:

```shell
d8 iam get rule CAR/<NAME>
d8 iam get rule AR/<NAMESPACE>/<NAME>
```

## Пример выдачи прав на все неймспейсы

{% alert level="info" %}
Пример относится к [упрощённой ролевой модели](./#упрощённая-ролевая-модель).
{% endalert %}

В режиме [multi-tenancy](configuration.html#parameters-enablemultitenancy) (`userAuthz.enableMultiTenancy`) ограничение доступа по неймспейсам задаётся полями ресурса [ClusterAuthorizationRule](cr.html#clusterauthorizationrule-v1-spec-namespaceselector).

Варианты настройки:

- **Разрешить доступ во все неймспейсы, включая системные** (`kube-*`, `d8-*`, `default`). Чтобы разрешить доступ во все неймспейсы, включая системные,  укажите `spec.namespaceSelector.matchAny: true`. Если параметр `namespaceSelector` задан, значения устаревших параметров `limitNamespaces` и `allowAccessToSystemNamespaces` **игнорируются**. Доступ в системные неймспейсы с уровнем `Admin` или `ClusterAdmin` равен администратору кластера: в этих неймспейсах живут ServiceAccount платформы (см. [Доступ системных и подсистемных ролей в неймспейсы модулей](./#доступ-системных-и-подсистемных-ролей-в-неймспейсы-модулей)).

  Пример:

  ```yaml
  apiVersion: deckhouse.io/v1
  kind: ClusterAuthorizationRule
  metadata:
    name: all-namespaces-including-system
  spec:
    subjects:
    - kind: User
      name: user@example.com
    accessLevel: Editor
    namespaceSelector:
      matchAny: true
  ```

- **Разрешить доступ во все неймспейсы, кроме системных**. Чтобы разрешить пользователю доступ во все неймспейсы, кроме системных, при создании ClusterAuthorizationRule не указывайте ни `namespaceSelector`, ни устаревшие параметры `limitNamespaces` и `allowAccessToSystemNamespaces`. Перечень системных неймспейсов — в [описании полей CR](cr.html#clusterauthorizationrule-v1-spec-namespaceselector).

{% alert level="info" %}
Уровень доступа `SuperAdmin` **не снимает** ограничения по неймспейсам, заданные в параметрах `namespaceSelector` и `limitNamespaces`. При необходимости предоставления доступа ко всем неймспейсам задайте область явно, в том числе через [`namespaceSelector.matchAny`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector).
{% endalert %}

Если одному субъекту соответствует несколько ресурсов `ClusterAuthorizationRule`, набор разрешённых неймспейсов **объединяется**; эффективный `accessLevel` — **самый сильный** среди всех подходящих правил. Подробнее — в [FAQ](faq.html#что-если-два-clusterauthorizationrules-подходят-для-одного-пользователя).

{% alert level="warning" %}
Ограничения по неймспейсам из ClusterAuthorizationRule применяет вебхук авторизации. API-сервер вызывает его до RBAC с `failurePolicy: Deny`, поэтому, если вебхук недоступен, запросы, которые он должен проверить, **отклоняются**. Вебхук не проверяет компоненты control plane, kubelet и ServiceAccount'ы неймспейса `kube-system` и неймспейсов, имена которых начинаются с `d8-`. Ограничения по неймспейсам к ним не применяются. Подробнее — в [описании модуля](./#упрощённая-ролевая-модель).
{% endalert %}

{% offtopic title="Гранулярная ролевая модель" %}
В гранулярной модели назначайте роли `d8:namespace:*` через RoleBinding в каждом неймспейсе или используйте [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) для всех неймспейсов проекта. Роли `d8:system:*` / `d8:subsystem:*` не распространяются на неймспейсы пользовательских приложений — только на системные (`d8-*`, `kube-*`) в рамках подсистемы.
{% endofftopic %}

## Создание пользователя

В Kubernetes есть две категории пользователей:

* ServiceAccount'ы, учёт которых ведёт сам Kubernetes через API.
* Остальные пользователи и группы, учёт которых ведёт не сам Kubernetes, а некоторый внешний софт, который настраивает администратор кластера, — существует множество механизмов аутентификации и, соответственно, множество способов заводить пользователей. В настоящий момент поддерживаются способы аутентификации:
  * Через модуль `user-authn`. Модуль поддерживает следующие внешние провайдеры и протоколы аутентификации: GitHub, GitLab, Bitbucket Cloud, Atlassian Crowd, LDAP, OIDC, SAML, а также локальные пользователи (объект [User](/modules/user-authn/cr.html#user)). Подробнее — в документации модуля [`user-authn`](/modules/user-authn/).
  * С помощью [сертификатов](#создание-пользователя-с-помощью-клиентского-сертификата).

При выпуске сертификата для аутентификации нужно указать в нём имя (`CN=<имя>`), необходимое количество групп (`O=<группа>`) и подписать его с помощью корневого CA-кластера. Именно этим механизмом вы аутентифицируетесь в кластере, когда, например, используете kubectl на master-узле.

### Создание ServiceAccount для сервера и предоставление ему доступа

{% alert level="info" %}
Пример использует [упрощённую ролевую модель](./#упрощённая-ролевая-модель).
{% endalert %}

Создание ServiceAccount с доступом к Kubernetes API может потребоваться, например, при настройке развёртывания приложений через CI-системы.  

1. Создайте неймспейс для CI, например `ci`, и ServiceAccount в нём:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: v1
   kind: Namespace
   metadata:
     name: ci
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: gitlab-runner-deploy
     namespace: ci
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: gitlab-runner-deploy-token
     namespace: ci
     annotations:
       kubernetes.io/service-account.name: gitlab-runner-deploy
   type: kubernetes.io/service-account-token
   EOF
   ```

1. Выдайте ServiceAccount'у необходимые права с помощью кастомного ресурса [ClusterAuthorizationRule](cr.html#clusterauthorizationrule):

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: ClusterAuthorizationRule
   metadata:
     name: gitlab-runner-deploy
   spec:
     subjects:
     - kind: ServiceAccount
       name: gitlab-runner-deploy
       namespace: ci
     accessLevel: Editor
   EOF
   ```

   Если включён режим multi-tenancy (параметр [`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy)), ограничьте доступные ServiceAccount'у неймспейсы параметром [`namespaceSelector`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector).

   {% alert level="warning" %}
   Не создавайте ServiceAccount'ы для внешних систем в `kube-system` и в неймспейсах, имена которых начинаются с `d8-`. Вебхук авторизации не проверяет ServiceAccount'ы этих неймспейсов, поэтому ограничения ClusterAuthorizationRule по неймспейсам (`namespaceSelector`, `limitNamespaces`, `allowAccessToSystemNamespaces`) к ним не применяются. Их уровень доступа действует во всех неймспейсах кластера, включая системные.
   {% endalert %}

1. Определите значения переменных (они будут использоваться далее), выполнив следующие команды (**подставьте свои значения**):

   ```shell
   export CLUSTER_NAME=my-cluster
   export USER_NAME=gitlab-runner-deploy.my-cluster
   export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
   export FILE_NAME=kube.config
   ```

1. Сгенерируйте секцию `cluster` в файле конфигурации kubectl:

   Используйте один из следующих вариантов доступа к API-серверу кластера:

   * Если есть прямой доступ до API-сервера:
     1. Получите сертификат CA-кластера Kubernetes:

        ```shell
        d8 k get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
        ```

     1. Сгенерируйте секцию `cluster` (используется IP-адрес API-сервера для доступа):

        ```shell
        d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(d8 k get endpointslices -l kubernetes.io/service-name=kubernetes -o json | jq -rc '.items[0] | "\(.endpoints[0].addresses[0]):\(.ports[0].port)"') \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * Если прямого доступа до API-сервера нет, используйте один следующих вариантов:
      * включите доступ к API-серверу через Ingress-контроллер (параметр [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) модуля `control-plane-manager`) и укажите адреса, с которых будут идти запросы (параметр [whitelistSourceRanges](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi-ingress-whitelistsourceranges));
      * укажите адреса, с которых будут идти запросы, в отдельном Ingress-контроллере (параметр [acceptRequestsFrom](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom)).

   * Если используется непубличный CA:

     1. Получите сертификат CA из секрета Ingress `kubernetes-api` в неймспейсе `kube-system`. Это сертификат домена `api.%s`:

        ```shell
        d8 k -n kube-system get secrets -o json \
          $(d8 k -n kube-system get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     2. Сгенерируйте секцию `cluster` (используется внешний домен и CA для доступа):

        ```shell
        d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(d8 k -n kube-system get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * Если используется публичный CA. Сгенерируйте секцию `cluster` (используется внешний домен для доступа):

     ```shell
     d8 k config set-cluster $CLUSTER_NAME \
       --server=https://$(d8 k -n kube-system get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
       --kubeconfig=$FILE_NAME
     ```

   * Если API-сервер опубликован через Gateway API, вместо Ingress в кластере есть HTTPRoute `kubernetes-api` в неймспейсе `kube-system`. В командах выше берите хост из него вместо `.spec.rules[].host`:

     ```shell
     d8 k -n kube-system get httproute kubernetes-api -o jsonpath='{.spec.hostnames[0]}'
     ```

     Имя Secret с сертификатом CA берите из ListenerSet `control-plane-manager` вместо `.spec.tls[0].secretName`:

     ```shell
     d8 k -n kube-system get listenerset control-plane-manager -o jsonpath='{.spec.listeners[?(@.name=="api")].tls.certificateRefs[0].name}'
     ```

1. Сгенерируйте секцию `user` с токеном из секрета ServiceAccount в файле конфигурации kubectl:

   ```shell
   d8 k config set-credentials $USER_NAME \
     --token=$(d8 k -n ci get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
     --kubeconfig=$FILE_NAME
   ```

1. Сгенерируйте контекст в файле конфигурации kubectl:

   ```shell
   d8 k config set-context $CONTEXT_NAME \
     --cluster=$CLUSTER_NAME --user=$USER_NAME \
     --kubeconfig=$FILE_NAME
   ```

1. Установите сгенерированный контекст как используемый по умолчанию в файле конфигурации kubectl:

   ```shell
   d8 k config use-context $CONTEXT_NAME --kubeconfig=$FILE_NAME
   ```

### Создание пользователя с помощью клиентского сертификата

{% alert level="info" %}
Этот способ рекомендуется использовать для системных нужд (аутентификация kubelet'ов, компонентов control plane и пр.). Если нужно создать «обычного» пользователя (например, с доступом через консоль, `kubectl` и т. д.), используйте генерацию [kubeconfig](/modules/user-authn/faq.html#как-сгенерировать-kubeconfig-для-доступа-к-kubernetes-api).
{% endalert %}

При создании пользователя с помощью клиентского сертификата можно использовать [OpenSSL](#создание-пользователя-с-помощью-сертификата-выпущенного-через-openssl) или [Kubernetes API (объект CertificateSigningRequest)](#создание-пользователя-с-помощью-сертификата-выпущенного-через-kubernetes-api).

{% alert level="warning" %}
Сертификаты, выпущенные любым из этих способов, отозвать нельзя.
В случае компрометации сертификата потребуется убрать все права этого пользователя (это может быть сложно, если пользователь добавлен в какие-нибудь группы: придётся также удалять все соответствующие группы).
{% endalert %}

#### Создание пользователя с помощью сертификата, выпущенного через OpenSSL

{% alert level="warning" %}
При использовании этого способа учитывайте риски безопасности.

`ca.crt` и `ca.key` не должны покидать master-узел: подписывайте CSR только на нём.

При подписании CSR вне master-узла есть риск компрометации корневого сертификата кластера.
{% endalert %}

Особенности этого способа:

- Клиентский сертификат должен подписываться на master-узле, чтобы не допустить компрометации кластерного сертификата.
- Необходим доступ к CA-ключу кластера (`ca.key`). Подписывать сертификаты может только администратор кластера.

Чтобы создать пользователя с помощью клиентского сертификата, выпущенного через OpenSSL, выполните следующие шаги:

1. Убедитесь, что у вас есть доступ к master-узлу. Файлы CA кластера `ca.crt` и `ca.key` находятся на нём в каталоге `/etc/kubernetes/pki/`.
1. Сгенерируйте ключ пользователя:

    ```shell
    openssl ecparam -name prime256v1 -genkey -out myuser.key
    ```

1. Создайте CSR, указав в нём имя пользователя `myuser`, который состоит в группах `mygroup1` и `mygroup2`:

    ```shell
    openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
    ```

1. Загрузите созданный на предыдущем шаге CSR (в этом примере — `myuser.csr`) на master-узел и подпишите его корневым сертификатом кластера. Пример команды для подписания CSR на master-узле (убедитесь, что в команде указаны верные для вашего случая пути к `myuser.csr`, `ca.crt` и `ca.key`):

    ```shell
    openssl x509 -req -in myuser.csr -CA /etc/kubernetes/pki/ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out myuser.crt -days 10
    ```

Полученный сертификат можно указывать в конфигурационном файле:

```shell
cat << EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $(cat /etc/kubernetes/pki/ca.crt | base64 -w0)
    server: https://<хост кластера>:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: myuser
  name: myuser@kubernetes
current-context: myuser@kubernetes
kind: Config
preferences: {}
users:
- name: myuser
  user:
    client-certificate-data: $(cat myuser.crt | base64 -w0)
    client-key-data: $(cat myuser.key | base64 -w0)
EOF
```

#### Создание пользователя с помощью сертификата, выпущенного через Kubernetes API

Это более безопасный способ, так как для подписания сертификата используется специальный API Kubernetes.

Особенности этого способа:

- Подписание сертификата через Kubernetes API: CSR отправляется на подпись через API и прямой доступ к `ca.key` не требуется.
- Выпускать клиентские сертификаты может не только администратор кластера. Право на создание CSR и их подписание можно назначить определённому пользователю.

Чтобы создать пользователя с помощью клиентского сертификата, выпущенного через Kubernetes API, выполните следующие шаги:

1. Сгенерируйте ключ пользователя:

    ```shell
    openssl ecparam -name prime256v1 -genkey -out myuser.key
    ```

1. Создайте CSR, указав в нём имя пользователя `myuser`, который состоит в группах `mygroup1` и `mygroup2`:

    ```shell
    openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
    ```

1. Создайте манифест объекта CertificateSigningRequest и сохраните его в файл (в этом примере — `csr.yaml`):

    {% alert level="info" %}
    В поле `request` замените `<CSR_BASE64>` содержимым `myuser.csr` в Base64 без переносов строк, например выводом команды `base64 < myuser.csr | tr -d '\n'`.
    {% endalert %}

    ```yaml
    apiVersion: certificates.k8s.io/v1
    kind: CertificateSigningRequest
    metadata:
      name: demo-client-cert
    spec:
      request: <CSR_BASE64>
      signerName: "kubernetes.io/kube-apiserver-client"
      expirationSeconds: 7200
      usages:
      - "digital signature"
      - "client auth"
    ```
  
1. Примените манифест, чтобы создать запрос на подпись сертификата:
  
    ```shell
    d8 k apply -f csr.yaml
    ```

1. Убедитесь, что сертификат подтверждён и выпущен:

    ```shell
    d8 k get csr demo-client-cert
    ```

    Если сертификат подтверждён, в колонке `CONDITION` у него будет значение `Approved,Issued`. Пример вывода:

    ```console
    NAME               AGE     SIGNERNAME                            REQUESTOR          REQUESTEDDURATION   CONDITION
    demo-client-cert   8m24s   kubernetes.io/kube-apiserver-client   kubernetes-admin   120m                Approved,Issued
    ```

    Если сертификат не подтвердился автоматически, подтвердите его:

    ```shell
    d8 k certificate approve demo-client-cert
    ```

    После этого убедитесь, что сертификат подтверждён.

1. Извлеките закодированный сертификат из CSR с именем `demo-client-cert`, декодируйте его из Base64 и сохраните в файл (в этом примере — `myuser.crt`):

    ```shell
    d8 k get csr demo-client-cert -ojsonpath="{.status.certificate}" | base64 -d > myuser.crt
    ```

Полученный сертификат необходимо указать в конфигурационном файле:

```shell
cat << EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $(cat /etc/kubernetes/pki/ca.crt | base64 -w0)
    server: https://<хост кластера>:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: myuser
  name: myuser@kubernetes
current-context: myuser@kubernetes
kind: Config
preferences: {}
users:
- name: myuser
  user:
    client-certificate-data: $(cat myuser.crt | base64 -w0)
    client-key-data: $(cat myuser.key | base64 -w0)
EOF
```

#### Предоставление доступа созданному пользователю

{% alert level="info" %}
Пример использует [упрощённую ролевую модель](./#упрощённая-ролевая-модель).
{% endalert %}

Для предоставления доступа созданному пользователю создайте `ClusterAuthorizationRule`.

Пример `ClusterAuthorizationRule`:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: myuser
spec:
  subjects:
  - kind: User
    name: myuser
  accessLevel: PrivilegedUser
  portForwarding: true
```

## Настройка kube-apiserver для работы в режиме multi-tenancy

Режим multi-tenancy, позволяющий ограничивать доступ к неймспейсам, включается параметром [enableMultiTenancy](configuration.html#parameters-enablemultitenancy) модуля.

Работа в режиме multi-tenancy требует включения [плагина авторизации Webhook](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) и выполнения настройки `kube-apiserver`. Все необходимые для работы режима multi-tenancy действия **выполняются автоматически** модулем [control-plane-manager](/modules/control-plane-manager/), никаких ручных действий не требуется.

Режим multi-tenancy доступен в редакциях, указанных в [описании параметра](configuration.html#parameters-enablemultitenancy). API-сервер настраивается автоматически, пока параметр [`controlPlaneConfigurator.enabled`](configuration.html#parameters-controlplaneconfigurator-enabled) равен `true`.

Изменения манифеста `kube-apiserver`, которые произойдут после включения режима multi-tenancy:

* флаг `--authorization-mode=Node,RBAC` заменяется на structured authorization configuration (`--authorization-config=/etc/kubernetes/deckhouse/extra-files/authorization-config.yaml`) с авторизаторами `Node`, `Webhook` и `RBAC`;
* создаётся `kubeconfig` для вебхука авторизации по пути `/etc/kubernetes/deckhouse/extra-files/webhook-config.yaml`. Оба файла находятся в каталоге `/etc/kubernetes/deckhouse/extra-files`, который уже смонтирован в `kube-apiserver`.

## Как проверить, что у пользователя есть доступ?

Необходимо выполнить следующую команду, в которой будут указаны:

* `resourceAttributes` (как в RBAC) — к чему мы проверяем доступ;
* `user` — имя пользователя;
* `groups` — группы пользователя.

{% alert level="info" %}
При совместном использовании с модулем `user-authn` группы и имя пользователя можно посмотреть в логах Dex (`d8 k -n d8-user-authn logs -l app=dex`). Они появляются в логах только после входа пользователя.
{% endalert %}

```shell
cat  <<EOF | 2>&1 d8 k  create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "dev",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "user@example.com",
    "groups": [
      "developers"
    ]
  }
}
EOF
```

В результате увидим, есть ли доступ и на основании какой роли:

```json
{
  "allowed": true,
  "reason": "RBAC: allowed by RoleBinding \"<BINDING_NAME>/dev\" of ClusterRole \"<ROLE_NAME>\" to User \"user@example.com\""
}
```

Ответ API-сервера уже учитывает решение вебхука авторизации. Чтобы получить ответ только вебхука, выполните на master-узле от имени root следующую команду:

```shell
cat  <<EOF | 2>&1 d8 k --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "dev",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "user@example.com",
    "groups": [
      "developers"
    ]
  }
}
EOF
```

```json
{
  "allowed": false
}
```

Сообщение `allowed: false` значит, что вебхук не блокирует запрос. В случае блокировки запроса вебхуком вы получите, например, следующее сообщение:

```json
{
  "allowed": false,
  "denied": true,
  "reason": "either you have no access to the namespace or the namespace does not exist"
}
```

## Настройка прав высокоуровневых ролей

Если требуется добавить прав для определённой [высокоуровневой роли](./#упрощённая-ролевая-модель), достаточно создать ClusterRole с аннотацией `user-authz.deckhouse.io/access-level: <AccessLevel>`.

Модуль автоматически выставляет на такие ClusterRole лейбл `user-authz.deckhouse.io/access-level` и включает их правила в агрегированную ClusterRole `user-authz:<access-level>:custom`. Лейбл служебный: модуль приводит его в соответствие с аннотацией и снимает, если аннотации нет, поэтому задавать его вручную не нужно. В журнале аудита доступ, выданный через аннотированную ClusterRole, приписывается этой агрегированной роли.

Аннотация принимает значения `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor` и `ClusterAdmin`. Другие значения, включая `SuperAdmin`, игнорируются. В имени агрегированной роли уровень записывается через дефис, например `user-authz:cluster-editor:custom`. Правила уровня также входят в агрегированные роли всех более высоких уровней.

Пример:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: Editor
  name: user-editor
rules:
- apiGroups:
  - kuma.io
  resources:
  - trafficroutes
  - trafficroutes/finalizers
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - flagger.app
  resources:
  - canaries
  - canaries/status
  - metrictemplates
  - metrictemplates/status
  - alertproviders
  - alertproviders/status
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
```
