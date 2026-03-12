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

1. Включите выдачу kubeconfig через API. Откройте настройки модуля `user-authn` (создайте ресурс ModuleConfig `user-authn`, если его нет):

   ```shell
   d8 k edit mc user-authn
   ```

1. Добавьте следующую секцию в блок `settings` и сохраните изменения:

   ```yaml
   publishAPI:
     enabled: true
   ```

1. Сгенерируйте kubeconfig [в веб-интерфейсе kubeconfigurator](/products/kubernetes-platform/documentation/v1/user/web/kubeconfig.html). Адрес интерфейса зависит от [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) (например, `kubeconfig.kube.my` при шаблоне `%s.kube.my`).
