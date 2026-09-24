---
title: "Авторизация"
permalink: ru/admin/configuration/access/authorization/
description: "Настройка авторизации и контроля доступа в Deckhouse Platform на основе RBAC. Управление правами пользователей, ролями и сервисными аккаунтами для безопасного доступа к кластеру."
lang: ru
---

В Deckhouse Platform авторизация реализована на основе стандартного механизма Role-Based Access Control (RBAC) Kubernetes. Это позволяет гибко управлять правами доступа для различных пользователей, групп и сервисных аккаунтов, обеспечивая безопасность и контроль над операциями в кластере.

Платформа поддерживает две ролевые модели:

- [Гранулярная](../authorization/rbac-experimental.html) (рекомендуется к использованию). Права доступа настраиваются стандартным для RBAC Kubernetes способом: с помощью создания ресурсов [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) или [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/), а для доступа сразу ко всем неймспейсам проекта — ресурсов [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) и [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) модуля `multitenancy-manager`.
- [Упрощённая](../authorization/rbac-current.html). Подсистема сквозной авторизации расширяет стандартный RBAC-механизм за счёт пользовательских ресурсов — [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Поддержка этой модели будет прекращена в будущих релизах.

Обе модели поддерживаются модулем [`user-authz`](/modules/user-authz/) и могут использоваться одновременно — права, которые они предоставляют, суммируются (подробнее — в [FAQ](/modules/user-authz/faq.html#можно-ли-использовать-упрощённую-и-гранулярную-ролевые-модели-одновременно) модуля `user-authz`). Выбор модели зависит от требований безопасности и сценариев использования.

## Совместное использование ClusterAuthorizationRule, AuthorizationRule и RBAC

Если в кластере включён режим мультитенантности (параметр [`enableMultiTenancy: true`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)), итоговые права пользователя представляют собой объединение прав, полученных из всех следующих источников:

- **права из ClusterAuthorizationRule** — применяются только внутри неймспейсов, разрешённых параметрами `limitNamespaces` или `namespaceSelector`;
- **права из AuthorizationRule** — применяются внутри неймспейса, в котором создан ресурс AuthorizationRule;
- **права из обычных RoleBinding** — применяются внутри неймспейса, в котором создан ресурс RoleBinding;
- **права из обычных ClusterRoleBinding**, не созданных модулем `user-authz` — применяются во всём кластере.

Ограничения по неймспейсам, заданные в ClusterAuthorizationRule, распространяются только на права, выданные этим ClusterAuthorizationRule. Они не отменяют права, выданные через RoleBinding, ClusterRoleBinding или AuthorizationRule в других неймспейсах. При этом права, предоставляемые ClusterAuthorizationRule, на эти неймспейсы не распространяются.

Например, если у пользователя есть ClusterAuthorizationRule с уровнем доступа `accessLevel: Editor`, ограниченный неймспейсом `ns-a`, а также RoleBinding с ролью `view` в неймспейсе `ns-b`, то он получит права уровня `Editor` в неймспейсе `ns-a` и права только на чтение в неймспейсе `ns-b`. Права, предоставленные через RoleBinding, не ограничиваются ClusterAuthorizationRule, а уровень доступа `Editor`, заданный ClusterAuthorizationRule, не распространяется на неймспейс `ns-b`.

Начиная с DP 1.76.5, RoleBinding и ClusterAuthorizationRule можно использовать совместно для одного пользователя. В более ранних версиях DP вебхук модуля `user-authz` отклонял все запросы в неймспейсы, не указанные в ClusterAuthorizationRule пользователя, даже при наличии соответствующих ресурсов RoleBinding.

## Кому и когда выдаются права

Есть два сценария выдачи прав в Deckhouse Platform:

- Выдача прав пользователям для работы через консольные клиенты, веб-интерфейсы и другие инструменты для администрирования, разработки и управления кластером.
- Выдача прав сервисным аккаунтам для автоматизации задач, таких как развёртывание приложений и их обновление (чаще всего при помощи подхода IaC). Примерами таких сервисов могут быть CI/CD-системы, системы мониторинга и другие.

При успешном прохождении аутентификации пользователи и сервисные аккаунты получают права доступа к ресурсам кластера на основе настроек авторизации.

### Аутентификация пользователей

В Deckhouse Platform поддерживается несколько способов аутентификации пользователей. Подробнее о них можно узнать в разделе [Аутентификация пользователей](../authentication/).

### Аутентификация сервисных аккаунтов

Сервисные аккаунты (ServiceAccount) в Kubernetes — это специальные учётные записи, которые используются для автоматизации задач и взаимодействия с API кластеров. Они позволяют приложениям и сервисам безопасно взаимодействовать с Kubernetes API.
В Deckhouse Platform сервисные аккаунты для внешних по отношению к кластеру сервисов создаются для единообразия в неймспейсе `d8-service-accounts`.

Пример манифеста для создания ServiceAccount:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gitlab-runner-deploy
  namespace: d8-service-accounts
```

После создания ServiceAccount необходимо выписать токен для него, чтобы сервис мог аутентифицироваться в кластере. Для этого создаётся секрет, который содержит токен доступа.

Пример манифеста для создания секрета с токеном ServiceAccount:

```yaml
 apiVersion: v1
 kind: Secret
 metadata:
   name: gitlab-runner-deploy-token
   namespace: d8-service-accounts
   annotations:
     kubernetes.io/service-account.name: gitlab-runner-deploy
 type: kubernetes.io/service-account-token
```
