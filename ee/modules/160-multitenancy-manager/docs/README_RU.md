---
title: "Модуль multitenancy-manager"
search: multitenancy
description: Модуль multitenancy-manager Deckhouse помогает удобно создавать шаблонизированные проекты в кластере Kubernetes с помощью ресурсов (Custom Resources). Шаблонизация ресурсов проекта с помощью Helm позволяет использовать в проекте любые объекты Kubernetes.
---

Модуль позволяет настраивать изолированные проекты в кластере Kubernetes.

По подготовленному [типу проекта](cr.html#projecttype) с помощью Custom Resource [Project](cr.html#project) в кластере Kubernetes можно получить одинаковые, изолированные друг от друга проекты с настроенными доступами пользователей (подробнее в разделе [Примеры](usage.html)).

Создание изолированных проектов с помощью модуля `multitenancy-manager` может быть удобно, например, в следующих случаях:
- В рамках процесса CI/CD — для создания окружения разработчика при тестировании или демонстрации работы кода.
- При развертывании приложений, с предоставлением разработчику ограниченного доступа в кластер.
- При предоставлении услуг по аренде ресурсов кластера.

## Возможности модуля

- Управление доступом пользователей и групп на базе механизма RBAC Kubernetes (на основе модуля [user-authz](../140-user-authz/)).
- Управление уровнем изоляции конкретных проектов.
- Шаблонизация однотипов проектов и их параметризация по OpenAPI-спецификации.
- Полная совместимость с `Helm` в шаблонах ресурсов.

## Принцип работы

При создании ресурса [Project](cr.html#project) происходит следующее:
- Создается `Namespace` с именем из ресурса [Project](cr.html#project).
- Создается правило авторизации ([AuthorizationRule](../140-user-authz/cr.html#authorizationrule)) для указанных [субъектов](cr.html#projecttype-v1alpha1-spec-subjects) в типе проекта [ProjectType](cr.html#projecttype).
- Выполняется рендеринг шаблонов (параметр [resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate) в типе проекта) с помощью [Helm](https://helm.sh/docs/). Значения для рендеринга берутся из параметра [template](cr.html#project-v1alpha1-spec-template) в ресурсе проекта. При рендеринге выполняется валидация значений согласно OpenAPI-спецификации (параметр [openAPI](cr.html#projecttype-v1alpha1-spec-openapi)).

Так как рендеринг [шаблонов](cr.html#projecttype-v1alpha1-spec-resourcestemplate) выполняется с помощью `Helm`, в шаблоне можно описать любые необходимые объекты Kubernetes, например `NetworkPolicy`, `LimitRange`, `ResourceQuota` и т.п.
