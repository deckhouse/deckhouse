---
title: "Модуль multitenancy-manager"
search: multitenancy
---

Модуль позволяет создавать изолированные окружения внутри одного kubernetes кластера на основе [user-authz](../../modules/140-user-authz) модуля и ресурсов kubernetes (`NetworkPolicy`, `LimitRange`, `ResourceQuota` и др.)

Вся настройка происходит с помощью [Custom Resources](cr.html).

## Возможности модуля

- Управление доступом пользователей и групп на базе механизма RBAC Kubernetes (на основе модуля [user-authz](../../modules/140-user-authz))
- Управление уровнем изоляции конкретных окружений
- Создание шаблонов для нескоьких окружений и кастомизациях параметрами по OpenAPI спецификации
- Полная совместимость с `helm` в темплейтах ресурсов
