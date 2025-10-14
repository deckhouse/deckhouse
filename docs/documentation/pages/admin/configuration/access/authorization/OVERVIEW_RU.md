---
title: "Обзор"
permalink: ru/admin/configuration/access/authorization/
lang: ru
---

В Deckhouse Kubernetes Platform авторизация реализована на основе стандартного механизма Role-Based Access Control (RBAC) Kubernetes. Это позволяет гибко управлять правами доступа для различных пользователей, групп и сервисных аккаунтов, обеспечивая безопасность и контроль над операциями в кластере.

Платформа поддерживает две ролевые модели:

- [Текущая](../authorization/rbac-current.html). Подсистема сквозной авторизации расширяет стандартный RBAC-механизм за счёт пользовательских ресурсов — [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule).
- [Экспериментальная](../authorization/rbac-experimental.html). Эта модель также предполагает использование стандартного механизма RBAC. Доступ настраивается с помощью создания ресурсов [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) или [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/).

Обе модели поддерживаются модулем [`user-authz`](/modules/user-authz/). Выбор модели зависит от требований безопасности и сценариев использования.

## Кому и когда выдаются права

Есть два сценария выдачи прав в Deckhouse Kubernetes Platform:

- Выдача прав пользователям для работы через консольные клиенты, веб-интерфейсы и другие инструменты для администрирования, разработки и управления кластером.
- Выдача прав сервисным аккаунтам для автоматизации задач, таких как развёртывание приложений и их обновление (чаще всего при помощи подхода IaC). Примерами таких сервисов могут быть CI/CD-системы, системы мониторинга и другие.

При успешном прохождении аутентификации пользователи и сервисные аккаунты получают права доступа к ресурсам кластера на основе настроек авторизации.

### Аутентификация пользователей

В Deckhouse Kubernetes Platform поддерживается несколько способов аутентификации пользователей. Подробнее о них можно узнать в разделе [Аутентификация пользователей](../authentication/).

### Аутентификация сервисных аккаунтов

Сервисные аккаунты (ServiceAccount) в Kubernetes — это специальные учётные записи, которые используются для автоматизации задач и взаимодействия с API кластеров. Они позволяют приложениям и сервисам безопасно взаимодействовать с Kubernetes API.
В Deckhouse Kubernetes Platform сервисные аккаунты для внешних по отношению к кластеру сервисов создаются для единообразия в пространстве имён `d8-service-accounts`.

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
