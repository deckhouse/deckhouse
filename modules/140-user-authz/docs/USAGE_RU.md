---
title: "Модуль user-authz: примеры конфигурации"
---

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
  # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition)
  allowAccessToSystemNamespaces: false
  # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition)
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
* Остальные пользователи, чей учёт ведёт не сам Kubernetes, а некоторый внешний софт, который настраивает администратор кластера – существует множество механизмов аутентификации и, соответственно, множество способов заводить пользователей. В настоящий момент поддерживается два способа аутентификации:
  * Через модуль [user-authn](../../modules/150-user-authn/).
  * С помощью сертификатов.

При выпуске сертификата для аутентификации, нужно указать в нем имя (`CN=<имя>`), необходимое количество групп (`O=<группа>`) и подписать его с помощью корневого CA кластера. Именно этим механизмом вы аутентифицируетесь в кластере, когда например используете kubectl на bastion-узле.

### Создание ServiceAccount для сервера и предоставление ему доступа

Может быть необходимо выдать постоянный доступ к Kubernetes API для сервера, например, чтобы CI-система могла выкладывать приложения в кластер.

1. Создайте `ServiceAccount` в namespace `d8-service-accounts` (имя можно изменить):

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

2. Дайте необходимые `ServiceAccount` права (используя custom resource [ClusterAuthorizationRule](cr.html#clusterauthorizationrule)):

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
     # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition)
     allowAccessToSystemNamespaces: true      
   EOF
   ```

   Если в конфигурации Deckhouse включен режим multitenancy (доступно только в версии Enterprise Edition), то, чтобы дать SA доступ в системные namespace'ы укажите `allowAccessToSystemNamespaces: true`.

3. Сгенерируйте `kube-config`, подставив свои значения переменных в начале:

   ```shell
   cluster_name=my-cluster
   user_name=gitlab-runner-deploy.my-cluster
   context_name=${cluster_name}-${user_name}
   file_name=kube.config
   ```

   * Секция `cluster`:
     * Если есть доступ напрямую до API-сервера, то используйте его IP:
       1. Получите CA кластера Kubernetes:

          ```shell
          kubectl get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
          ```

       2. Сгенерируйте секцию с IP API-сервера:

          ```shell
          kubectl config set-cluster $cluster_name --embed-certs=true \
            --server=https://$(kubectl get ep kubernetes -o json | jq -rc '.subsets[0] | "\(.addresses[0].ip):\(.ports[0].port)"') \
            --certificate-authority=/tmp/ca.crt \
            --kubeconfig=$file_name
          ```

     * Если прямого доступа до API-сервера нет, то [включите](../../modules/150-user-authn/configuration.html#параметры) `publishAPI` с `whitelistSourceRanges`. Либо через отдельный Ingress-controller укажите адреса, только с которых будут идти запросы: при помощи опции `ingressClass` с конечным списком `SourceRange` укажите в настройках контроллера список CIDR в параметре `acceptRequestsFrom`.

     * Если используется непубличный CA:

       1. Получите его из Secret'а с сертификатом для домена `api.%s`:

          ```shell
          kubectl -n d8-user-authn get secrets -o json \
            $(kubectl -n d8-user-authn get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
            | jq -rc '.data."ca.crt" // .data."tls.crt"' \
            | base64 -d > /tmp/ca.crt
          ```

       2. И сгенерируйте секцию с внешним доменом и CA:

          ```shell
          kubectl config set-cluster $cluster_name --embed-certs=true \
            --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
            --certificate-authority=/tmp/ca.crt \
            --kubeconfig=$file_name
          ```

     * Если CA публичный, просто сгенерируйте секцию с внешним доменом:

       ```shell
       kubectl config set-cluster $cluster_name \
         --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
         --kubeconfig=$file_name
       ```

   * Секция `user` с токеном из Secret'а `ServiceAccount`:

     ```shell
     kubectl config set-credentials $user_name \
       --token=$(kubectl -n d8-service-accounts get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
       --kubeconfig=$file_name
     ```

   * Секция `context` для связи:

     ```shell
     kubectl config set-context $context_name \
       --cluster=$cluster_name --user=$user_name \
       --kubeconfig=$file_name
     ```

   * Установите контекст по умолчанию для только что созданного kubeconfig файла:

     ```shell
     kubectl config use-context $context_name --kubeconfig=$file_name
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

### Настройка `kube-apiserver` для работы в режиме multi-tenancy

Режим multi-tenancy, позволяющий ограничивать доступ к namespace, включается параметром [enableMultiTenancy](configuration.html#parameters-enablemultitenancy) модуля.

Работа в режиме multi-tenancy требует включения [плагина авторизации Webhook](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) и выполнения настройки `kube-apiserver`. Все необходимые для работы режима multi-tenancy действия **выполняются автоматически** модулем [control-plane-manager](../../modules/040-control-plane-manager/), никаких ручных действий не требуется.

Изменения манифеста `kube-apiserver`, которые произойдут после включения режима multi-tenancy:

* Исправление аргумента `--authorization-mode`. Перед методом RBAC добавится метод Webhook (например — `--authorization-mode=Node,Webhook,RBAC`).
* Добавление аргумента `--authorization-webhook-config-file=/etc/kubernetes/authorization-webhook-config.yaml`.
* Добавление `volumeMounts`:

  ```yaml
  - name: authorization-webhook-config
    mountPath: /etc/kubernetes/authorization-webhook-config.yaml
    readOnly: true
  ```

* Добавление `volumes`:

  ```yaml
  - name: authorization-webhook-config
    hostPath:
      path: /etc/kubernetes/authorization-webhook-config.yaml
      type: FileOrCreate
  ```

## Как проверить, что у пользователя есть доступ?

Необходимо выполнить следующую команду, в которой будут указаны:

* `resourceAttributes` (как в RBAC) — к чему мы проверяем доступ
* `user` — имя пользователя
* `groups` — группы пользователя

> При совместном использовании с модулем `user-authn`, группы и имя пользователя можно посмотреть в логах Dex — `kubectl -n d8-user-authn logs -l app=dex` (видны только при авторизации)

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

Если в кластере включен режим **multitenancy**, то нужно выполнить еще одну проверку, чтобы убедиться, что у пользователя есть доступ в namespace:

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

Сообщение `allowed: false` значит что webhook не блокирует запрос. В случае блокировки запроса webhook'ом вы получите, например, следующее сообщение:

```json
{
  "allowed": false,
  "denied": true,
  "reason": "making cluster scoped requests for namespaced resources are not allowed"
}
```

## Настройка прав высокоуровневых ролей

Если требуется добавить прав для определённой [высокоуровневой роли](./#ролевая-модель), то достаточно создать ClusterRole с аннотацией `user-authz.deckhouse.io/access-level: <AccessLevel>`.

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
