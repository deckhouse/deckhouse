---
title: "Выдача прав пользователям и сервисным аккаунтам"
permalink: ru/admin/configuration/access/authorization/granting.html
description: "Настройка RBAC для пользователей и сервисных аккаунтов в Deckhouse Platform. Настройка привязки ролей и кластерных ролей для безопасного контроля доступа."
lang: ru
---

Для выдачи прав в Deckhouse Platform в пользовательских ресурсах указывается блок `subjects`. Формат этого блока зависит от типа ресурса:

- в ресурсах [AuthorizationRule и ClusterAuthorizationRule](#предоставление-прав-с-помощью-authorizationrule-и-clusterauthorizationrule-упрощённая-ролевая-модель) (упрощённая модель), а также [ProjectRoleBinding и ClusterProjectRoleBinding](#предоставление-прав-с-помощью-clusterrolebinding-и-rolebinding-гранулярная-ролевая-модель) модуля `multitenancy-manager` (гранулярная модель, уровень проекта) — используется [блок `subjects`](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-subjects), показанный ниже;
- в стандартных ресурсах Kubernetes [RoleBinding и ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) (гранулярная модель, уровень неймспейса, подсистемы или всей платформы) формат отличается: для `kind: User` и `kind: Group` дополнительно обязательно поле `apiGroup: rbac.authorization.k8s.io` (для `kind: ServiceAccount` оно не требуется). Подробнее — в примера в разделе [«Предоставление прав с помощью ClusterRoleBinding и RoleBinding»](#предоставление-прав-с-помощью-clusterrolebinding-и-rolebinding-гранулярная-ролевая-модель) ниже.

Для пользователя он указывается в формате:

```yaml
subjects:
- kind: User
  name: <email пользователя>
```

{% alert level="warning" %}
В случае использования [модуля `user-authn`](/modules/user-authn/) и статических пользователей, указывайте в `subjects` именно email пользователя, а не имя [ресурса User](/modules/user-authn/cr.html#user).
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

При использовании упрощённой ролевой модели в Deckhouse Platform для предоставления прав пользователям можно использовать ресурсы [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) и [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule).

{% alert level="warning" %}
Поддержка упрощённой ролевой модели будет прекращена в будущих релизах. Для новых ролей используйте [гранулярную ролевую модель](#предоставление-прав-с-помощью-clusterrolebinding-и-rolebinding-гранулярная-ролевая-модель) ниже — её можно использовать одновременно с упрощённой моделью, так как права из обеих моделей суммируются.
{% endalert %}

### Предоставление прав пользователю в рамках одного неймспейса

Если нужно предоставить права пользователю в рамках одного неймспейса, используйте ресурс [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Он действует в рамках одного неймспейса.
Пример:

```yaml
apiVersion: deckhouse.io/v1
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

Если нужно предоставить права пользователю во всех неймспейсах, включая системные (например, для предоставления прав администратора), используйте ресурс [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule). Он действует во всем кластере.

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
  # Опция доступна только при включенном режиме enableMultiTenancy 
  # в модуле user-authz (версия Enterprise Edition).
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
  accessLevel: SuperAdmin
  portForwarding: true
```  

## Предоставление прав с помощью ClusterRoleBinding и RoleBinding (гранулярная ролевая модель)

При использовании гранулярной ролевой модели в Deckhouse Platform для предоставления прав пользователям можно использовать ресурсы [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) и [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/), а для выдачи доступа сразу ко всем неймспейсам проекта — ресурсы [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) модуля `multitenancy-manager`.

Роли гранулярной модели действуют в одной из четырёх областей — неймспейс, проект, подсистема или вся платформа, — каждая со своим форматом имени роли и уровнями доступа. Подробнее — в разделе [«Области действия ролей»](rbac-experimental.html#области-действия-ролей).

### Назначение прав администратору кластера (гранулярная ролевая модель)

Для назначения прав администратору кластера используйте [системную роль](../authorization/rbac-experimental.html#системные-и-подсистемные-роли) `d8:system:manager` в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/).

Пример назначения прав администратору кластера (User `jane`):

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
Права, которые получит пользователь, будут ограничены рамками неймспейсов, начинающихся с `d8-` или `kube-`.

Пользователю будут доступны следующие права:

- Просмотр, изменение, удаление и создание ресурсов Kubernetes и модулей DP.
- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов ModuleConfig).
- Выполнение следующих команд к подам и сервисам:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}

### Назначение прав сетевому администратору (гранулярная ролевая модель)

Для назначения прав сетевому администратору на управление сетевой подсистемой кластера используйте [роль подсистемы](../authorization/rbac-experimental.html#системные-и-подсистемные-роли) `d8:subsystem:networking:manager` в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/).

Пример назначения прав сетевому администратору (User `jane`):

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
  name: d8:subsystem:networking:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Список прав, которые получит пользователь" %}
Права, которые получит пользователь, будут ограничены следующим списком неймспейсов модулей DP из подсистемы `networking` (фактический список зависит от списка включённых в кластере модулей):

- `d8-cni-cilium`;
- `d8-cni-flannel`;
- `d8-cni-simple-bridge`;
- `d8-ingress-nginx`;
- `d8-istio`;
- `d8-metallb`;
- `d8-network-gateway`;
- `d8-openvpn`;
- `d8-static-routing-manager`;
- `d8-system`;
- `kube-system`.

Пользователю будут доступны следующие права:

- Просмотр, изменение, удаление и создание *стандартных* ресурсов Kubernetes в неймспейсе модулей из подсистемы `networking`.

  Пример ресурсов, которыми сможет управлять пользователь (список не полный):
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

- Просмотр, изменение, удаление и создание ресурсов в неймспейсе модулей из подсистемы `networking`.

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
  - RoutingTable.

- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов ModuleConfig) из подсистемы `networking`.

  Список модулей, которыми сможет управлять пользователь:
  - `cilium-hubble`;
  - `cni-cilium`;
  - `cni-flannel`;
  - `cni-simple-bridge`;
  - `flow-schema`;
  - `ingress-nginx`;
  - `istio`;
  - `kube-dns`;
  - `kube-proxy`;
  - `metallb`;
  - `network-gateway`;
  - `network-policy-engine`;
  - `node-local-dns`;
  - `openvpn`;
  - `static-routing-manager`.

- Выполнение следующих команд к подам и сервисам в неймспейсе модулей из подсистемы `networking`:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
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
