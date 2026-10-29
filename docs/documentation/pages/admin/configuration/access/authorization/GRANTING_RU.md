---
title: "Выдача прав пользователям и сервисным аккаунтам"
permalink: ru/admin/configuration/access/authorization/granting.html
description: "Настройка RBAC для пользователей и сервисных аккаунтов в Deckhouse Platform. Настройка привязки ролей и кластерных ролей для безопасного контроля доступа."
lang: ru
---

Для выдачи прав в Deckhouse Platform (DP) указывается блок `subjects`. Его формат и состав зависят от типа ресурса:

- в ресурсах [AuthorizationRule и ClusterAuthorizationRule](#предоставление-прав-с-помощью-authorizationrule-и-clusterauthorizationrule-упрощённая-ролевая-модель) (упрощённая модель), а также [ProjectRoleBinding и ClusterProjectRoleBinding](#предоставление-прав-с-помощью-clusterrolebinding-и-rolebinding-гранулярная-ролевая-модель) модуля `multitenancy-manager` (гранулярная модель, уровень [проекта](../../../../glossary.html#project)) — используется [блок `subjects`](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-subjects), показанный ниже;
- в стандартных ресурсах Kubernetes [RoleBinding и ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) (гранулярная модель, уровень неймспейса, [подсистемы](../../../../glossary.html#subsystem) или всей платформы) формат отличается: для `kind: User` и `kind: Group` в элементе `subjects` дополнительно обязательно поле `apiGroup: rbac.authorization.k8s.io` (для `kind: ServiceAccount` оно не требуется). Подробнее — в примерах в разделе [«Предоставление прав с помощью ClusterRoleBinding и RoleBinding»](#предоставление-прав-с-помощью-clusterrolebinding-и-rolebinding-гранулярная-ролевая-модель) ниже.

Для пользователя он указывается в формате:

```yaml
subjects:
- kind: User
  name: <email пользователя>
```

{% alert level="warning" %}
В случае использования [модуля `user-authn`](/modules/user-authn/) и [статических пользователей](../../../../glossary.html#static-user), указывайте в `subjects` именно email пользователя, а не имя [ресурса User](/modules/user-authn/cr.html#user).
{% endalert %}

или

```yaml
subjects:
- kind: Group
  name: <группа, в которой состоит пользователь>
```

Для сервисного аккаунта блок `subjects` указывается в формате:

```yaml
subjects:
- kind: ServiceAccount
  name: <имя сервисного аккаунта>
  namespace: <неймспейс, в котором создан сервисный аккаунт>
```

## Предоставление прав с помощью AuthorizationRule и ClusterAuthorizationRule (упрощённая ролевая модель)

В упрощённой [ролевой модели](../../../../glossary.html#rbac) Deckhouse Platform для предоставления прав пользователям можно использовать ресурсы [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) и [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule).

{% alert level="warning" %}
Поддержка упрощённой ролевой модели будет прекращена в будущих релизах. Для новых ролей используйте [гранулярную ролевую модель](#предоставление-прав-с-помощью-clusterrolebinding-и-rolebinding-гранулярная-ролевая-модель) ниже — её можно использовать одновременно с упрощённой моделью, так как права из обеих моделей суммируются.
{% endalert %}

Объектами AuthorizationRule и ClusterAuthorizationRule можно управлять и без написания YAML-манифестов — командой [`d8 iam access`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-access): `d8 iam access grant` создаёт их, а `d8 iam access revoke` удаляет. Например, чтобы выдать пользователю уровень доступа `Admin` в неймспейсе `dev`, выполните:

```shell
d8 iam access grant user anton --access-level Admin -n dev
```

Другие примеры приведены в [документации модуля `user-authz`](/modules/user-authz/usage.html#управление-правами-через-cli).

### Предоставление прав пользователю в рамках одного неймспейса

Если нужно предоставить права пользователю в рамках одного неймспейса, используйте ресурс [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Он действует в рамках одного неймспейса.
Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AuthorizationRule
metadata:
  name: dev-access
  namespace: dev-namespace
spec:
  subjects:
  - kind: User
    name: dev-user@example.com
  accessLevel: Admin
  portForwarding: true
```

### Предоставление прав пользователю во всех неймспейсах

Если нужно предоставить права пользователю во всех неймспейсах (например, для предоставления прав администратора), используйте ресурс [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule). Он действует во всем кластере.

{% alert level="info" %}
В режиме multitenancy ClusterAuthorizationRule без [`namespaceSelector`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector) не предоставляет доступ в [системные неймспейсы](../../../../glossary.html#system-namespace): для них нужен [`namespaceSelector.matchAny: true`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector-matchany) или устаревший [`allowAccessToSystemNamespaces: true`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-allowaccesstosystemnamespaces).
{% endalert %}

При необходимости можно ограничить область действия прав, предоставляемых с помощью [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule), одним или несколькими неймспейсами. Для этого в его манифесте укажите соответствующие ограничения (но, если позволяет возможность, рекомендуемый вариант для этого — использование [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule)). Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin-access
spec:
  subjects:
  - kind: User
    name: dev-user@example.com
  # Опция доступна только при включённом параметре enableMultiTenancy
  # модуля user-authz (редакции BE, SE, SE+, Ultimate/EE, Certified Core, Certified Pro и CSE).
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
  accessLevel: Admin
  portForwarding: true
```

{% alert level="warning" %}
Параметры `namespaceSelector` и `limitNamespaces` ограничивают только доступ к объектам в неймспейсах. Субъект с уровнем доступа `ClusterAdmin` или `SuperAdmin` сохраняет кластерные права, в том числе на создание ClusterRoleBinding и ClusterAuthorizationRule, поэтому эти параметры не ограничивают права, которые он может выдать на весь кластер. Выдавайте эти уровни только тем, кому можно предоставить доступ ко всему кластеру.
{% endalert %}

## Предоставление прав с помощью ClusterRoleBinding и RoleBinding (гранулярная ролевая модель)

При использовании гранулярной [ролевой модели](../../../../glossary.html#rbac) в Deckhouse Platform для предоставления прав пользователям можно использовать ресурсы [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) и [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/), а для выдачи доступа сразу ко всем неймспейсам [проекта](../../../../glossary.html#project) — ресурсы [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) модуля `multitenancy-manager`.

Роли гранулярной модели действуют в одной из четырёх областей — неймспейс, проект, [подсистема](../../../../glossary.html#subsystem) или вся платформа, — каждая со своим форматом имени роли и уровнями доступа. Подробнее — в разделе [«Области действия ролей»](rbac-experimental.html#области-действия-ролей).

{% alert level="info" %}
Выдать доступ можно и без написания YAML-манифестов — с помощью мастера выдачи доступа в [веб-интерфейсе Deckhouse](../../../../user/web/ui.html). Мастер проводит по шагам (кому выдать доступ → где → с каким уровнем), сам выбирает вид привязки (RoleBinding, ClusterRoleBinding, ProjectRoleBinding или ClusterProjectRoleBinding) и позволяет собрать собственную роль из готовых capabilities.
{% endalert %}

### Назначение прав администратору кластера (гранулярная ролевая модель)

Для назначения прав администратору кластера используйте [системную роль](../authorization/rbac-experimental.html#системные-и-подсистемные-роли) `d8:system:manager` в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/).

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
Пользователь получит доступ к cluster-wide-ресурсам, к конфигурации модулей DP и к неймспейсам включённых модулей DP. Доступа к неймспейсам пользовательских приложений у него не будет.

Пользователю будут доступны следующие права:

- Просмотр, изменение, удаление и создание cluster-wide-ресурсов Kubernetes и cluster-wide-ресурсов модулей DP.
- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов ModuleConfig).
- Просмотр объектов, логов, метрик и событий в неймспейсах модулей DP без доступа к секретам.

Роль `d8:system:manager` не позволяет изменять объекты в неймспейсах модулей DP, читать там секреты и выполнять `kubectl exec`, `kubectl attach` и `kubectl port-forward` к их подам. Для этих операций нужна роль `d8:system:superadmin`. Подробнее — в разделе [«Доступ системных и подсистемных ролей в неймспейсы модулей»](rbac-experimental.html#доступ-системных-и-подсистемных-ролей-в-неймспейсы-модулей).

{% endofftopic %}

### Назначение прав сетевому администратору (гранулярная ролевая модель)

Чтобы предоставить администратору права на управление сетевой подсистемой кластера, назначьте ему [роль подсистемы](../authorization/rbac-experimental.html#системные-и-подсистемные-роли) `d8:subsystem:network:manager` в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/).

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
Права, которые получит пользователь, будут ограничены следующим списком неймспейсов модулей DP из подсистемы `network` (фактический список зависит от списка включённых в кластере модулей):

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

- Просмотр объектов, логов, метрик и событий в неймспейсах модулей из подсистемы `network` без доступа к секретам.

- Просмотр, изменение, удаление и создание следующих ресурсов модулей из подсистемы `network`:

  Список ресурсов, которыми сможет управлять пользователь:

  - EgressGateway;
  - EgressGatewayPolicy;
  - FlowSchema;
  - IngressClass;
  - IngressIstioController;
  - IngressNginxController;
  - IPRuleSet;
  - IstioFederation;
  - IstioMulticluster;
  - KeepalivedInstance;
  - RoutingTable.

- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов ModuleConfig) из подсистемы `network`.

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

Роль `d8:subsystem:network:manager` не позволяет изменять объекты в неймспейсах модулей, читать там секреты и выполнять `kubectl exec`, `kubectl attach` и `kubectl port-forward` к их подам. Для этих операций нужна роль `d8:subsystem:network:superadmin`. Подробнее — в разделе [«Доступ системных и подсистемных ролей в неймспейсы модулей»](rbac-experimental.html#доступ-системных-и-подсистемных-ролей-в-неймспейсы-модулей).

{% endofftopic %}

### Назначение административных прав пользователю в рамках неймспейса (гранулярная ролевая модель)

Чтобы назначить/ограничить права пользователя конкретными неймспейсами, используйте в [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) [namespace-роль](../authorization/rbac-experimental.html#namespace-роли) с соответствующим уровнем доступа.

Например, для назначения прав на управление ресурсами приложений в рамках неймспейса, но без возможности настройки модулей DP, используйте роль `d8:namespace:admin` в [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) в соответствующем неймспейсе. Чтобы выдать такой же доступ сразу во всех неймспейсах проекта, вместо этого используйте [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) с аналогичной ролью `d8:project:admin`.

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

- Просмотр, изменение, удаление и создание ресурсов Kubernetes. Например, следующих ресурсов:
  - Certificate;
  - CertificateRequest;
  - ConfigMap;
  - ControllerRevision;
  - CronJob;
  - DaemonSet;
  - Deployment;
  - Event;
  - HorizontalPodAutoscaler;
  - Ingress;
  - Issuer;
  - Job;
  - Lease;
  - LimitRange;
  - NetworkPolicy;
  - PersistentVolumeClaim;
  - Pod;
  - PodDisruptionBudget;
  - ReplicaSet;
  - ReplicationController;
  - ResourceQuota;
  - Role;
  - RoleBinding;
  - Secret;
  - Service;
  - ServiceAccount;
  - StatefulSet;
  - VerticalPodAutoscaler;
  - VolumeSnapshot.
- Просмотр, изменение, удаление и создание следующих ресурсов модулей DP:
  - DexAuthenticator;
  - DexClient;
  - PodLoggingConfig.
- Выполнение следующих команд к подам и сервисам:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}
