---
title: "Модуль user-authz: примеры конфигурации"
---

## Пример назначения прав администратору кластера

{% alert level="info" %}
Пример использует [экспериментальную ролевую модель](./#экспериментальная-ролевая-модель).
{% endalert %}

Для назначения прав администратору кластера используйте роль `d8:manage:all:manager` в `ClusterRoleBinding`.

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

## Пример назначения прав сетевому администратору

{% alert level="info" %}
Пример использует [экспериментальную ролевую модель](./#экспериментальная-ролевая-модель).
{% endalert %}

Для назначения прав сетевому администратору на управление сетевой подсистемой кластера используйте роль `d8:manage:networking:manager` в `ClusterRoleBinding`.

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

## Пример назначения административных прав пользователю в рамках пространства имён

{% alert level="info" %}
Пример использует [экспериментальную ролевую модель](./#экспериментальная-ролевая-модель).
{% endalert %}

Для назначения прав на управление ресурсами приложений в рамках пространства имён, но без возможности настройки модулей DKP, используйте роль `d8:use:role:admin` в `RoleBinding` в соответствующем пространстве имён.

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

## Пример `ClusterAuthorizationRule`

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
    namespace: d8-service-accounts
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition).
  allowAccessToSystemNamespaces: false
  # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition).
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

## Создание пользователя

В Kubernetes есть две категории пользователей:

* ServiceAccount'ы, учёт которых ведёт сам Kubernetes через API.
* Остальные пользователи, учёт которых ведёт не сам Kubernetes, а некоторый внешний софт, который настраивает администратор кластера, — существует множество механизмов аутентификации и, соответственно, множество способов заводить пользователей. В настоящий момент поддерживаются два способа аутентификации:
  * через модуль [user-authn](../../modules/user-authn/);
  * с помощью сертификатов.

При выпуске сертификата для аутентификации нужно указать в нём имя (`CN=<имя>`), необходимое количество групп (`O=<группа>`) и подписать его с помощью корневого CA-кластера. Именно этим механизмом вы аутентифицируетесь в кластере, когда, например, используете kubectl на bastion-узле.

### Создание ServiceAccount для сервера и предоставление ему доступа

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

1. Дайте необходимые ServiceAccount-права (используя custom resource [ClusterAuthorizationRule](cr.html#clusterauthorizationrule)):

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

   Если в конфигурации Deckhouse включён режим мультитенантности (параметр [enableMultiTenancy](configuration.html#parameters-enablemultitenancy), доступен только в Enterprise Edition), настройте доступные для ServiceAccount пространства имён (параметр [namespaceSelector](cr.html#clusterauthorizationrule-v1-spec-namespaceselector)).

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
        kubectl get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
        ```

     1. Сгенерируйте секцию `cluster` (используется IP-адрес API-сервера для доступа):

        ```shell
        kubectl config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(kubectl get ep kubernetes -o json | jq -rc '.subsets[0] | "\(.addresses[0].ip):\(.ports[0].port)"') \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * Если прямого доступа до API-сервера нет, используйте один следующих вариантов:
      * включите доступ к API-серверу через Ingress-контроллер (параметр [publishAPI](../user-authn/configuration.html#parameters-publishapi)) и укажите адреса, с которых будут идти запросы (параметр [whitelistSourceRanges](../user-authn/configuration.html#parameters-publishapi-whitelistsourceranges));
      * укажите адреса, с которых будут идти запросы, в отдельном Ingress-контроллере (параметр [acceptRequestsFrom](../ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom)).

   * Если используется непубличный CA:

     1. Получите сертификат CA из секрета с сертификатом, который используется для домена `api.%s`:

        ```shell
        kubectl -n d8-user-authn get secrets -o json \
          $(kubectl -n d8-user-authn get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     2. Сгенерируйте секцию `cluster` (используется внешний домен и CA для доступа):

        ```shell
        kubectl config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * Если используется публичный CA. Сгенерируйте секцию `cluster` (используется внешний домен для доступа):

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

### Создание пользователя с помощью клиентского сертификата

#### Создание пользователя

* Получите корневой сертификат кластера (ca.crt и ca.key).
* Сгенерируйте ключ пользователя:

  ```shell
  openssl genrsa -out myuser.key 2048
  ```

* Создайте CSR, где укажите, что требуется пользователь `myuser`, который состоит в группах `mygroup1` и `mygroup2`:

  ```shell
  openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
  ```

* Подпишите CSR корневым сертификатом кластера:

  ```shell
  openssl x509 -req -in myuser.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out myuser.crt -days 10
  ```

* Теперь полученный сертификат можно указывать в конфиг-файле:

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

#### Предоставление доступа созданному пользователю

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

## Настройка `kube-apiserver` для работы в режиме multi-tenancy

Режим multi-tenancy, позволяющий ограничивать доступ к пространству имён, включается параметром [enableMultiTenancy](configuration.html#parameters-enablemultitenancy) модуля.

Работа в режиме multi-tenancy требует включения [плагина авторизации Webhook](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) и выполнения настройки `kube-apiserver`. Все необходимые для работы режима multi-tenancy действия **выполняются автоматически** модулем [control-plane-manager](../../modules/control-plane-manager/), никаких ручных действий не требуется.

Изменения манифеста `kube-apiserver`, которые произойдут после включения режима multi-tenancy:

* исправление аргумента `--authorization-mode`. Перед методом RBAC добавится метод Webhook (например, `--authorization-mode=Node,Webhook,RBAC`);
* добавление аргумента `--authorization-webhook-config-file=/etc/kubernetes/authorization-webhook-config.yaml`;
* добавление `volumeMounts`:

  ```yaml
  - name: authorization-webhook-config
    mountPath: /etc/kubernetes/authorization-webhook-config.yaml
    readOnly: true
  ```

* добавление `volumes`:

  ```yaml
  - name: authorization-webhook-config
    hostPath:
      path: /etc/kubernetes/authorization-webhook-config.yaml
      type: FileOrCreate
  ```

## Как проверить, что у пользователя есть доступ?

Необходимо выполнить следующую команду, в которой будут указаны:

* `resourceAttributes` (как в RBAC) — к чему мы проверяем доступ;
* `user` — имя пользователя;
* `groups` — группы пользователя.

> При совместном использовании с модулем `user-authn` группы и имя пользователя можно посмотреть в логах Dex — `kubectl -n d8-user-authn logs -l app=dex` (видны только при авторизации).

```shell
cat  <<EOF | 2>&1 kubectl  create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "system:kube-controller-manager",
    "groups": [
      "Admins"
    ]
  }
}
EOF
```

В результате увидим, есть ли доступ и на основании какой роли:

```json
{
  "allowed": true,
  "reason": "RBAC: allowed by ClusterRoleBinding \"system:kube-controller-manager\" of ClusterRole \"system:kube-controller-manager\" to User \"system:kube-controller-manager\""
}
```

Если в кластере включён режим **multi-tenancy**, нужно выполнить ещё одну проверку, чтобы убедиться, что у пользователя есть доступ в пространство имён:

```shell
cat  <<EOF | 2>&1 kubectl --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "system:kube-controller-manager",
    "groups": [
      "Admins"
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
  "reason": "making cluster scoped requests for namespaced resources are not allowed"
}
```

## Настройка прав высокоуровневых ролей

Если требуется добавить прав для определённой [высокоуровневой роли](./#текущая-ролевая-модель), достаточно создать ClusterRole с аннотацией `user-authz.deckhouse.io/access-level: <AccessLevel>`.

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
