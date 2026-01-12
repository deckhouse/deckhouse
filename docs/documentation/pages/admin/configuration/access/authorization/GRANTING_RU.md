---
title: "Выдача прав пользователям и сервисным аккаунтам"
permalink: ru/admin/configuration/access/authorization/granting.html
description: "Настройка RBAC для пользователей и сервисных аккаунтов в Deckhouse Kubernetes Platform. Настройка привязки ролей и кластерных ролей для безопасного контроля доступа."
lang: ru
---

Для выдачи прав в Deckhouse Kubernetes Platform в пользовательских ресурсах указывается [блок `subjects`](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-subjects).

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
  namespace: <пространство имён, в котором создан сервисный аккаунт>
```

## Предоставление прав с помощью AuthorizationRule и ClusterAuthorizationRule (текущая ролевая модель)

При использовании текущей ролевой модели в Deckhouse Kubernetes Platform для предоставления прав пользователям можно использовать ресурсы [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) и [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule).

### Предоставление прав пользователю в рамках одного пространства имен

Если нужно предоставить права пользователю в рамках одного пространства имен, используйте ресурс [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Он действует в рамках одного пространства имен.
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

### Предоставление прав пользователю во всех пространствах имен

Если нужно предоставить права пользователю во всех пространствах имен, включая системные (например, для предоставления прав администратора), используйте ресурс [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule). Он действует во всем кластере.

При необходимости можно ограничить область действия прав, предоставляемых с помощью [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule), одним или несколькими пространствами имен. Для этого в его манифесте укажите соответствующие ограничения (но, если позволяет возможность, рекомендуемый вариант для этого — использование [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule)). Пример:

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

## Предоставление прав с помощью ClusterRoleBinding и RoleBinding (экспериментальная ролевая модель)

При использовании экспериментальной ролевой модели в Deckhouse Kubernetes Platform для предоставления прав пользователям можно использовать ресурсы [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) и [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/).

### Назначение прав администратору кластера (экспериментальная ролевая модель)

Для назначения прав администратору кластера используйте [manage-роль](../authorization/rbac-experimental.html#manage-роли) `d8:manage:all:manager` в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/).

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
  name: d8:manage:all:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Права, которые получит пользователь" %}
Права, которые получит пользователь, будут ограничены рамками пространств имён, начинающихся с `d8-` или `kube-`.

Пользователю будут доступны следующие права:

- Просмотр, изменение, удаление и создание ресурсов Kubernetes и модулей DKP.
- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов ModuleConfig).
- Выполнение следующих команд к подам и сервисам:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}

### Назначение прав сетевому администратору (экспериментальная ролевая модель)

Для назначения прав сетевому администратору на управление сетевой подсистемой кластера используйте [manage-роль](../authorization/rbac-experimental.html#manage-роли) `d8:manage:networking:manager` в [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/).

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
  name: d8:manage:networking:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Список прав, которые получит пользователь" %}
Права, которые получит пользователь, будут ограничены следующим списком пространств имён модулей DKP из подсистемы `networking` (фактический список зависит от списка включённых в кластере модулей):

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

- Просмотр, изменение, удаление и создание *стандартных* ресурсов Kubernetes в пространстве имён модулей из подсистемы `networking`.

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

- Просмотр, изменение, удаление и создание ресурсов в пространстве имён модулей из подсистемы `networking`.

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

- Выполнение следующих команд к подам и сервисам в пространстве имён модулей из подсистемы `networking`:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}

### Назначение административных прав пользователю в рамках пространства имён (экспериментальная ролевая модель)

Чтобы назначить/ограничить права пользователя конкретными пространствами имён, используйте в [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) [use-роль](../authorization/rbac-experimental.html#use-роли) с соответствующим уровнем доступа.

Например, для назначения прав на управление ресурсами приложений в рамках пространства имён, но без возможности настройки модулей DKP, используйте роль `d8:use:role:admin` в [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) в соответствующем пространстве имён.

Пример назначения прав разработчику приложений (User `app-developer`) в пространстве имён `myapp`:

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
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Список прав, которые получит пользователь" %}
В рамках пространства имён `myapp` пользователю будут доступны следующие права:

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
- Просмотр, изменение, удаление и создание следующих ресурсов модулей DKP:
  - DexAuthenticator;
  - DexClient;
  - PodLoggingConfig.
- Выполнение следующих команд к подам и сервисам:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}
