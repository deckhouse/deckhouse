---
title: "Доступ для CI/CD"
permalink: ru/admin/configuration/access/authorization/ci_cd.html
description: "Настройка доступа CI/CD к кластеру Kubernetes в Deckhouse Kubernetes Platform. Настройка ServiceAccount, генерация kubeconfig и конфигурация доступа для CI/CD."
lang: ru
---

Для получения доступа к API-кластера Kubernetes для CI/CD-систем, таких как GitLab Runner, Jenkins и других, необходимо создать ServiceAccount, настроить права доступа и сгенерировать файл конфигурации kubeconfig. Этот файл будет использоваться для подключения к API-кластера.

Чтобы настроить доступ к API-кластера Kubernetes для CI/CD-системы, выполните следующие шаги:

1. Создайте ServiceAccount в пространстве имён `d8-service-accounts`:

   ```shell
   d8 k create -f - <<EOF
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

1. Назначьте необходимые для ServiceAccount права согласно инструкциям в разделе [Выдача прав пользователям и сервисным аккаунтам](../authorization/granting.html).

   Для текущей ролевой модели:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterAuthorizationRule
   metadata:
     name: gitlab-admin-access
   spec:
     subjects:
     - kind: ServiceAccount
       name: gitlab-runner-deploy
       namespace: d8-service-accounts
     accessLevel: SuperAdmin
     portForwarding: true
   ```

   Для экспериментальной ролевой модели:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: gitlab-admin-access
   subjects:
   - kind: ServiceAccount
     name: gitlab-runner-deploy
     namespace: d8-service-accounts
   roleRef:
     kind: ClusterRole
     name: d8:manage:all:manager
     apiGroup: rbac.authorization.k8s.io
    ```

1. Определите значения переменных (они будут использоваться далее), выполнив следующие команды (**подставьте свои значения**):

   ```shell
   export CLUSTER_NAME=my-cluster
   export USER_NAME=gitlab-runner-deploy.my-cluster
   export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
   export FILE_NAME=kube.config
   ```

1. Сгенерируйте секцию `cluster` в файле конфигурации `kubectl`. Используйте один из следующих вариантов доступа к API-серверу кластера:

   - Если есть прямой доступ к API-серверу:
     - Получите сертификат CA-кластера Kubernetes:

       ```shell
       d8 k get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
       ```

     - Сгенерируйте секцию `cluster` (используется IP-адрес API-сервера для доступа):

       ```shell
       d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
         --server=https://$(d8 k get ep kubernetes -o json | jq -rc '.subsets[0] | "\(.addresses[0].ip):\(.ports[0].port)"') \
         --certificate-authority=/tmp/ca.crt \
         --kubeconfig=$FILE_NAME
       ```

   - Если прямого доступа к API-серверу нет, используйте один следующих вариантов:
     - включите доступ к API-серверу через Ingress-контроллер (параметр [`publishAPI`](/modules/user-authn/configuration.html#parameters-publishapi)) и укажите адреса, с которых будут идти запросы (параметр [`whitelistSourceRanges`](/modules/user-authn/configuration.html#parameters-publishapi-whitelistsourceranges));
     - укажите адреса, с которых будут идти запросы, в отдельном Ingress-контроллере (параметр [`acceptRequestsFrom`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom)).

   - **Если используется непубличный CA:**

     - Получите сертификат CA из секрета с сертификатом, который используется для домена `api.%s`:

       ```shell
       d8 k -n d8-user-authn get secrets -o json \
         $(d8 k -n d8-user-authn get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
         | jq -rc '.data."ca.crt" // .data."tls.crt"' \
         | base64 -d > /tmp/ca.crt
       ```

     - Сгенерируйте секцию `cluster` (используется внешний домен и CA для доступа):

       ```shell
       d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
         --server=https://$(d8 k -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
         --certificate-authority=/tmp/ca.crt \
         --kubeconfig=$FILE_NAME
       ```

   - **Если используется публичный CA.** Сгенерируйте секцию `cluster` (используется внешний домен для доступа):

     ```shell
     d8 k config set-cluster $CLUSTER_NAME \
       --server=https://$(d8 k -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
       --kubeconfig=$FILE_NAME
     ```

1. Сгенерируйте секцию `user` с токеном из секрета ServiceAccount в файле конфигурации `kubectl`:

   ```shell
   d8 k config set-credentials $USER_NAME \
     --token=$(d8 k -n d8-service-accounts get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
     --kubeconfig=$FILE_NAME
   ```

1. Сгенерируйте контекст в файле конфигурации `kubectl`:

   ```shell
   d8 k config set-context $CONTEXT_NAME \
     --cluster=$CLUSTER_NAME --user=$USER_NAME \
     --kubeconfig=$FILE_NAME
   ```

1. Установите сгенерированный контекст как используемый по умолчанию в файле конфигурации `kubectl`:

   ```shell
   d8 k config use-context $CONTEXT_NAME --kubeconfig=$FILE_NAME
   ```

Далее можно использовать сгенерированный файл `$FILE_NAME` конфигурации kubeconfig для подключения к API-кластера Kubernetes из CI/CD-системы, такой как GitLab Runner или Jenkins.
