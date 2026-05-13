{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Чтобы развернуть Deckhouse Kubernetes Platform на DVP, выполните предварительную настройку в системе виртуализации. Для этого создайте пользователя (ServiceAccount), назначьте ему права и получите kubeconfig.

1. Создайте пользователя (ServiceAccount и токен), выполнив следующую команду:

   ```bash
   d8 k create -f -<<EOF
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: sa-demo
     namespace: default
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: sa-demo-token
     namespace: default
     annotations:
       kubernetes.io/service-account.name: sa-demo
   type: kubernetes.io/service-account-token
   EOF
   ```

1. Назначьте созданному пользователю роль. Для этого выполните:

   ```bash
   d8 k create -f -<<EOF
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: sa-demo-rb
     namespace: default
   subjects:
     - kind: ServiceAccount
       name: sa-demo
       namespace: default
   roleRef:
     kind: ClusterRole
     name: d8:use:role:manager
     apiGroup: rbac.authorization.k8s.io
   EOF
   ```

1. Включите выдачу kubeconfig через API. Откройте настройки модуля `user-authn` (создайте ресурс [ModuleConfig](../../../documentation/v1/reference/api/cr.html#moduleconfig) `user-authn`, если его нет):

   ```shell
   d8 k edit mc user-authn
   ```

1. Добавьте следующую секцию в блок `settings` и сохраните изменения:

   ```yaml
   publishAPI:
     enabled: true
   ```

1. Сгенерируйте kubeconfig, который будет использоваться в файле первичной конфигурации кластера на следующем шаге:

   ```bash
   cat <<EOF > kubeconfig
   apiVersion: v1
   clusters:
     - cluster:
       server: https://<KUBE-APISERVER-URL>   # Замените на реальный адрес API-сервера кластера.
     name: <CLUSTER-NAME>                     # Замените на имя кластера.
   contexts:
     - context:
       cluster: <CLUSTER-NAME>                # Замените на имя кластера.
       user: sa-demo
       namespace: default
     name: sa-demo-context
   current-context: sa-demo-context
   kind: Config
   preferences: {}
   users:
     - name: sa-demo
     user:
       token: $(d8 k get secret sa-demo-token -n default -o json | jq -rc .data.token | base64 -d)
   EOF
   ```

   Закодируйте сгенерированный kubeconfig в кодировке Base64 (он указывается в файле первичной конфигурации в таком виде):

   ```bash
   base64 kubeconfig | tr -d '\n'
   ```
