---
title: "Авторизация"
permalink: ru/admin/configuration/access/authorization/
description: "Настройка авторизации и контроля доступа в Deckhouse Platform на основе RBAC. Управление правами пользователей, ролями и сервисными аккаунтами для безопасного доступа к кластеру."
lang: ru
---

В Deckhouse Platform авторизация реализована на основе стандартного механизма Role-Based Access Control (RBAC) Kubernetes. Это позволяет гибко управлять правами доступа для различных пользователей, групп и сервисных аккаунтов, обеспечивая безопасность и контроль над операциями в кластере.

Платформа поддерживает две [ролевые модели](../../../../glossary.html#rbac):

- [Гранулярная](../authorization/rbac-experimental.html) (рекомендуется к использованию). Права доступа настраиваются стандартным для RBAC Kubernetes способом: с помощью создания ресурсов [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) или [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/), а для доступа сразу ко всем неймспейсам [проекта](../../../../glossary.html#project) — ресурсов [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) модуля `multitenancy-manager`.
- [Упрощённая](../authorization/rbac-current.html). [Подсистема](../../../../glossary.html#subsystem) сквозной авторизации расширяет стандартный RBAC-механизм за счёт пользовательских ресурсов — [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Поддержка этой модели будет прекращена в будущих релизах.

Обе модели поддерживаются модулем [`user-authz`](/modules/user-authz/) и могут использоваться одновременно — права, которые они предоставляют, суммируются (подробнее — в [FAQ](/modules/user-authz/faq.html#можно-ли-использовать-упрощённую-и-гранулярную-ролевые-модели-одновременно) модуля `user-authz`). Выбор модели зависит от требований безопасности и сценариев использования.

## Совместное использование ClusterAuthorizationRule, AuthorizationRule и RBAC

Если в кластере включён режим мультитенантности (параметр [`enableMultiTenancy: true`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)), итоговые права пользователя представляют собой объединение прав, полученных из всех следующих источников:

- **права из ClusterAuthorizationRule** — применяются только внутри неймспейсов, разрешённых параметром `namespaceSelector` (или устаревшим параметром `limitNamespaces`);
- **права из AuthorizationRule** — применяются внутри неймспейса, в котором создан ресурс AuthorizationRule;
- **права из обычных RoleBinding** — применяются внутри неймспейса, в котором создан ресурс RoleBinding;
- **права из обычных ClusterRoleBinding**, не созданных модулем `user-authz` — применяются во всём кластере.

Ограничения по неймспейсам, заданные в ClusterAuthorizationRule, распространяются только на права, выданные этим ClusterAuthorizationRule. Они не отменяют права, выданные через RoleBinding, ClusterRoleBinding или AuthorizationRule в других неймспейсах. При этом права, предоставляемые ClusterAuthorizationRule, на эти неймспейсы не распространяются.

Ограничения по неймспейсам применяет вебхук авторизации, поэтому они не действуют на идентичности, которые вебхук не проверяет: компоненты control plane, kubelet и ServiceAccount'ы неймспейса `kube-system` и неймспейсов, имена которых начинаются с `d8-`. ClusterAuthorizationRule, в котором указан такой ServiceAccount (напрямую или через группу, например `system:serviceaccounts`), даёт его уровень доступа во всех неймспейсах. Создавайте ServiceAccount'ы внешних систем в обычных неймспейсах.

Например, если у пользователя есть ClusterAuthorizationRule с уровнем доступа `accessLevel: Editor`, ограниченный неймспейсом `ns-a`, а также RoleBinding с ролью `view` в неймспейсе `ns-b`, то он получит права уровня `Editor` в неймспейсе `ns-a` и права только на чтение в неймспейсе `ns-b`. Права, предоставленные через RoleBinding, не ограничиваются ClusterAuthorizationRule, а уровень доступа `Editor`, заданный ClusterAuthorizationRule, не распространяется на неймспейс `ns-b`.

## Кому и когда выдаются права

Есть два сценария выдачи прав в Deckhouse Platform:

- Выдача прав пользователям для работы через консольные клиенты, веб-интерфейсы и другие инструменты для администрирования, разработки и управления кластером.
- Выдача прав сервисным аккаунтам для автоматизации задач, таких как развёртывание приложений и их обновление (чаще всего при помощи подхода IaC). Примерами таких сервисов могут быть CI/CD-системы, системы мониторинга и другие.

При успешном прохождении аутентификации пользователи и сервисные аккаунты получают права доступа к ресурсам кластера на основе настроек авторизации.

### Аутентификация пользователей

В Deckhouse Platform поддерживается несколько способов аутентификации пользователей. Подробнее о них можно узнать в разделе [«Аутентификация пользователей»](../authentication/).

### Аутентификация сервисных аккаунтов

Сервисные аккаунты (ServiceAccount) в Kubernetes — это специальные учётные записи, которые используются для автоматизации задач и взаимодействия с API кластеров. Они позволяют приложениям и сервисам безопасно взаимодействовать с Kubernetes API.

{% alert level="warning" %}
Не создавайте ServiceAccount'ы для внешних систем в `kube-system` и в неймспейсах, имена которых начинаются с `d8-`. Вебхук авторизации не проверяет ServiceAccount'ы этих неймспейсов, поэтому ограничения ClusterAuthorizationRule по неймспейсам (`namespaceSelector`, `limitNamespaces`, `allowAccessToSystemNamespaces`) к ним не применяются. Их уровень доступа действует во всех неймспейсах кластера, включая системные.
{% endalert %}

Создание ServiceAccount с доступом к Kubernetes API может потребоваться, например, при настройке развёртывания приложений через CI-системы. Для создания ServiceAccount и предоставления ему доступа выполните следующие шаги:

{% alert level="info" %}
Пример использует упрощённую ролевую модель.
{% endalert %}

1. Создайте неймспейс для CI и ServiceAccount в нём:

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

1. Выдайте ServiceAccount'у необходимые права с помощью кастомного ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule):

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

   Если включён режим мультитенантности (параметр [`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) модуля `user-authz`), ограничьте доступные ServiceAccount'у неймспейсы параметром [`namespaceSelector`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector).

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
     1. Получите сертификат CA кластера Kubernetes:

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

   * Если прямого доступа до API-сервера нет, используйте один из следующих вариантов:
      * включите доступ к API-серверу через [Ingress-контроллер](../../../../glossary.html#ingress-controller) (параметр [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) модуля `control-plane-manager`) и укажите адреса, с которых будут идти запросы (параметр [whitelistSourceRanges](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi-ingress-whitelistsourceranges));
      * укажите адреса, с которых будут идти запросы, в отдельном Ingress-контроллере (параметр [acceptRequestsFrom](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom)).

   * Если используется непубличный CA:

     1. Получите сертификат CA из секрета Ingress `kubernetes-api` в неймспейсе `kube-system`. Это сертификат домена `api.%s`:

        ```shell
        d8 k -n kube-system get secrets -o json \
          $(d8 k -n kube-system get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     1. Сгенерируйте секцию `cluster` (используется внешний домен и CA для доступа):

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

   * Если API-сервер опубликован через Gateway API, вместо Ingress в кластере есть HTTPRoute `kubernetes-api` в неймспейсе `kube-system`. В командах выше используйте хост из него вместо `.spec.rules[].host`:

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
