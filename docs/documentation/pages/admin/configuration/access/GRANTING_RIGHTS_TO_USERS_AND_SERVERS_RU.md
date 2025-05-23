---
title: "Выдача прав пользователям и серверам"
permalink: ru/admin/access/granting-rights-to-users-and-servers.html
lang: ru
---

В Kubernetes существуют две категории пользователей:

- ServiceAccount'ы, учёт которых ведёт сам Kubernetes через API.
- Остальные (статические) пользователи, учёт которых ведёт не сам Kubernetes, а внешний софт, который настраивает администратор кластера. Существует множество механизмов аутентификации и, соответственно, множество способов заводить пользователей. В настоящий момент поддерживаются два способа аутентификации:
  - через модуль [user-authn](../../reference/mc/user-authn/);
  - с помощью сертификатов.

При выпуске сертификата для аутентификации нужно указать в нём имя (`CN=<имя>`), необходимое количество групп (`O=<группа>`) и подписать его с помощью корневого CA-кластера. Именно этот механизм используется для аутентификации в кластере, когда, например, используется kubectl на bastion-узле. Пример выпуска сертификата в разделе [Создание пользователя](#создание-пользователя).

Пример манифеста для создания статического пользователя:

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: new-user
spec:
  email: new-user@example.ru
  password: $2a$10$MRhpW7jfXisdwLMM1bLEJehVojy3xWy0lfUzthSNoqG6mMUl.jLEG
```

Пример манифеста для создания ServiceAccount:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gitlab-runner-deploy-1
  namespace: alpha-project
```

## Предоставление прав с помощью AuthorizationRule и ClusterAuthorizationRule (текущая модель)

При использовании текущей ролевой модели в Deckhouse Kubernetes Platform для предоставления прав пользователям можно использовать ресурсы [AuthorizationRule](../../reference/cr/authorizationrule/) и [ClusterAuthorizationRule](../../reference/cr/clusterauthorizationrule/).

Если вам нужно предоставить права пользователю в рамках одного пространства имен, используйте [AuthorizationRule](../../reference/cr/authorizationrule/). Он действует в рамках одного пространства имен.
Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: AuthorizationRule
metadata:
  name: dev-access
spec:
  subjects:
    - kind: User
      name: dev-user@example.com
```

[ClusterAuthorizationRule](../../reference/cr/clusterauthorizationrule/) действует во всем кластере. Используйте его, если нужно предоставить права пользователю во всех пространствах имен, включая системные (например, для предоставления прав администратора).

При необходимости можно ограничить область действия прав, предоставляемых с помощью [ClusterAuthorizationRule](../../reference/cr/clusterauthorizationrule/), одним или несколькими пространствами имен, указав в манифесте соответствующие ограничения (но, если позволяет возможность, рекомендуемый вариант для этого — использование [AuthorizationRule](../../reference/cr/authorizationrule/)). Пример:

```yaml
apiVersion: deckhouse.io/v1
   kind: ClusterAuthorizationRule
   metadata:
     name: admin-access
   spec:
     subjects:
     - kind: User
       name: dev-user@example.com
       # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition).
       namespaceSelector:
        labelSelector:
          matchLabels:
            env: review
     accessLevel: SuperAdmin
     portForwarding: true
```  

## Создание ServiceAccount для сервера и настройка его прав (текущая модель)

Создание ServiceAccount с доступом к Kubernetes API может потребоваться, например, при настройке развёртывания приложений через CI-системы.  

1. Создайте ServiceAccount, например в пространстве имён `d8-service-accounts`:

   ```shell
   kubectl create -f - <<EOF
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: gitlab-runner-deploy
     namespace: d8-service-accounts
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: gitlab-runner-deploy-token
     namespace: d8-service-accounts
     annotations:
       kubernetes.io/service-account.name: gitlab-runner-deploy
   type: kubernetes.io/service-account-token
   EOF
   ```

1. Назначьте необходимые для ServiceAccount права (используя кастомный ресурс [ClusterAuthorizationRule](../../reference/cr/clusterauthorizationrule/)):

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: ClusterAuthorizationRule
   metadata:
     name: gitlab-runner-deploy
   spec:
     subjects:
     - kind: ServiceAccount
       name: gitlab-runner-deploy
       namespace: d8-service-accounts
     accessLevel: SuperAdmin
     # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition).
     allowAccessToSystemNamespaces: true      
   EOF
   ```

   Если в конфигурации Deckhouse включён режим мультитенантности (параметр [enableMultiTenancy](../../reference/mc/user-authz/#parameters-enablemultitenancy), доступен только в Enterprise Edition), настройте доступные для ServiceAccount пространства имён (параметр [namespaceSelector](../../reference/cr/clusterauthorizationrule/#clusterauthorizationrule-v1-spec-namespaceselector)).

1. Определите значения переменных (они будут использоваться далее), выполнив следующие команды (**подставьте свои значения**):

   ```shell
   export CLUSTER_NAME=my-cluster
   export USER_NAME=gitlab-runner-deploy.my-cluster
   export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
   export FILE_NAME=kube.config
   ```

1. Сгенерируйте секцию `cluster` в файле конфигурации kubectl. Используйте один из следующих вариантов доступа к API-серверу кластера:

   - Если есть прямой доступ к API-серверу:
     - Получите сертификат CA-кластера Kubernetes:

        ```shell
        kubectl get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
        ```

     - Сгенерируйте секцию `cluster` (используется IP-адрес API-сервера для доступа):

        ```shell
        kubectl config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(kubectl get ep kubernetes -o json | jq -rc '.subsets[0] | "\(.addresses[0].ip):\(.ports[0].port)"') \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   - Если прямого доступа к API-серверу нет, используйте один следующих вариантов:
     - включите доступ к API-серверу через Ingress-контроллер (параметр [publishAPI](../../reference/mc/user-authn/#parameters-publishapi)) и укажите адреса, с которых будут идти запросы (параметр [whitelistSourceRanges](../../reference/mc/user-authn/#parameters-publishapi/#parameters-publishapi-whitelistsourceranges));
     - укажите адреса, с которых будут идти запросы, в отдельном Ingress-контроллере (параметр [acceptRequestsFrom](../../reference/cr/ingressnginxcontroller/#ingressnginxcontroller-v1-spec-acceptrequestsfrom)).

   - **Если используется непубличный CA:**

     - Получите сертификат CA из секрета с сертификатом, который используется для домена `api.%s`:

        ```shell
        kubectl -n d8-user-authn get secrets -o json \
          $(kubectl -n d8-user-authn get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     - Сгенерируйте секцию `cluster` (используется внешний домен и CA для доступа):

        ```shell
        kubectl config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   - **Если используется публичный CA.** Сгенерируйте секцию `cluster` (используется внешний домен для доступа):

     ```shell
     kubectl config set-cluster $CLUSTER_NAME \
       --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
       --kubeconfig=$FILE_NAME
     ```

1. Сгенерируйте секцию `user` с токеном из секрета ServiceAccount в файле конфигурации kubectl:

   ```shell
   kubectl config set-credentials $USER_NAME \
     --token=$(kubectl -n d8-service-accounts get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
     --kubeconfig=$FILE_NAME
   ```

1. Сгенерируйте контекст в файле конфигурации kubectl:

   ```shell
   kubectl config set-context $CONTEXT_NAME \
     --cluster=$CLUSTER_NAME --user=$USER_NAME \
     --kubeconfig=$FILE_NAME
   ```

1. Установите сгенерированный контекст как используемый по умолчанию в файле конфигурации kubectl:

   ```shell
   kubectl config use-context $CONTEXT_NAME --kubeconfig=$FILE_NAME
   ```

## Создание пользователя с помощью клиентского сертификата и настройка его прав (текущая модель)

### Создание пользователя

- Получите корневой сертификат кластера (ca.crt и ca.key).
- Сгенерируйте ключ пользователя:

  ```shell
  openssl genrsa -out myuser.key 2048
  ```

- Создайте CSR с указанием пользователя `myuser`, входящего в группы `mygroup1` и `mygroup2`:

  ```shell
  openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
  ```

- Подпишите CSR корневым сертификатом кластера:

  ```shell
  openssl x509 -req -in myuser.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out myuser.crt -days 10
  ```

- Укажите полученный сертификат в файле конфигурации:

  ```shell
  cat << EOF
  apiVersion: v1
  clusters:
  - cluster:
      certificate-authority-data: $(cat ca.crt | base64 -w0)
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

### Предоставление доступа созданному пользователю

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

## Настройка прав высокоуровневых ролей (текущая модель)

Если требуется добавить права для определённой [высокоуровневой роли](../access/authorization-rbac-current.html#высокоуровневые-роли-используемые-для-реализации-модели), создайте ClusterRole с аннотацией `user-authz.deckhouse.io/access-level: <AccessLevel>`.

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

<!-- перенесено из [## Пример назначения прав администратору кластера](https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/user-authz/usage.html#%D0%BF%D1%80%D0%B8%D0%BC%D0%B5%D1%80-%D0%BD%D0%B0%D0%B7%D0%BD%D0%B0%D1%87%D0%B5%D0%BD%D0%B8%D1%8F-%D0%BF%D1%80%D0%B0%D0%B2-%D0%B0%D0%B4%D0%BC%D0%B8%D0%BD%D0%B8%D1%81%D1%82%D1%80%D0%B0%D1%82%D0%BE%D1%80%D1%83-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B0) -->

## Назначение прав администратору кластера (экспериментальная модель)

Для назначения прав администратору кластера используйте [manage-роль](../access/authorization-rbac-experimental.html#manage-роли) `d8:manage:all:manager` в `ClusterRoleBinding`.

Пример назначения прав администратору кластера (User `joe`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-admin-joe
subjects:
- kind: User
  name: joe
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
- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов `moduleConfig`).
- Выполнение следующих команд к подам и сервисам:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}

## Назначение прав сетевому администратору (экспериментальная модель)

Для назначения прав сетевому администратору на управление сетевой подсистемой кластера используйте [manage-роль](../access/authorization-rbac-experimental.html#manage-роли) `d8:manage:networking:manager` в `ClusterRoleBinding`.

Пример назначения прав сетевому администратору (User `joe`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: network-admin-joe
subjects:
- kind: User
  name: joe
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

- Просмотр, изменение, удаление и создание ресурсов в пространстве имён модулей из подсистемы `networking`.

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
  - `RoutingTable`.

- Изменение конфигурации модулей (просмотр, изменение, удаление и создание ресурсов moduleConfig) из подсистемы `networking`.

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

## Назначение административных прав пользователю в рамках пространства имён (экспериментальная модель)

Чтобы назначить/ограничить права пользователя конкретными пространствами имён, используйте в `RoleBinding` [use-роль](../access/authorization-rbac-experimental.html#use-роли) с соответствующим уровнем доступа.

Например, для назначения прав на управление ресурсами приложений в рамках пространства имён, но без возможности настройки модулей DKP, используйте роль `d8:use:role:admin` в `RoleBinding` в соответствующем пространстве имён.

Пример назначения прав разработчику приложений (User `app-developer`) в пространстве имён `myapp`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: myapp-developer
  namespace: myapp
subjects:
- kind: User
  name: app-developer
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Список прав, которые получит пользователь" %}
В рамках пространства имён `myapp` пользователю будут доступны следующие права:

- Просмотр, изменение, удаление и создание ресурсов Kubernetes. Например, следующих ресурсов:
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
- Просмотр, изменение, удаление и создание следующих ресурсов модулей DKP:
  - `DexAuthenticator`;
  - `DexClient`;
  - `PodLogginConfig`.
- Выполнение следующих команд к подам и сервисам:
  - `kubectl attach`;
  - `kubectl exec`;
  - `kubectl port-forward`;
  - `kubectl proxy`.
{% endofftopic %}
