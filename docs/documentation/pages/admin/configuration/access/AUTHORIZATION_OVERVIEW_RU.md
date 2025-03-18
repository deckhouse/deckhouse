---
title: "Общие сведения об авторизации в Deckhouse Kubernetes Platform"
permalink: ru/admin/access/authorization-overview.html
lang: ru
---

В Deckhouse Kubernetes Platform авторизация реализована на основе стандартного механизма RBAC (Role-Based Access Control) Kubernetes. Это позволяет гибко настраивать права доступа для различных пользователей, групп и сервисных аккаунтов, обеспечивая безопасность и контроль над операциями в кластере.

Deckhouse Kubernetes Platform используется две ролевые модели доступа, основанные на базе стандартного механизма RBAC Kubernetes:

- [Текущая](../access/authorization-rbac-current.html). Реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC за счет использования Custom Resources, таких как [ClusterAuthorizationRule](#) и [AuthorizationRule](#).

- [Экспериментальная](../access/authorization-rbac-experimental.html). Эта модель подразумевает использование для настройки доступа стандартного для RBAC Kubernetes способа: с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, в которых используются роли, подготовленные модулем [user-authz](#). Custom Resources [ClusterAuthorizationRule](#) и [AuthorizationRule](#) в экспериментальной модели не используются.

Обе модели реализуются с помощью модуля [user-authz](#).

Выбор ролевой модели зависит от ваших потребностей. В этом разделе представлена информация о настройке авторизации с использованием обеих моделей, а также рекомендации по их применению в различных сценариях.
