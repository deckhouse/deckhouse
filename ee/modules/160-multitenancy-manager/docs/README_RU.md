---
title: "Модуль multitenancy-manager"
search: multitenancy
description: Модуль multitenancy-manager Deckhouse помогает удобно создавать шаблонизированные окружения в кластере Kubernetes с помощью ресурсов (Custom Resources). Рендеринг шаблонов окружения с помощью Helm позволяет использовать в шаблоне окружения любые объекты Kubernetes.  
---

Модуль позволяет настраивать изолированные окружения в кластере Kubernetes.

По подготовленному [шаблону](cr.html#projecttype) с помощью Custom Resource [Project](cr.html#project) в кластере Kubernetes можно получить одинаковые, изолированные друг от друга окружения с настроенными доступами пользователей (подробнее в разделе [Примеры](usage.html)).

Создание изолированных окружений с помощью модуля `multitenancy-manager` может быть удобно, например, в следующих случаях:
- В рамках процесса CI/CD — для создания окружений разработчика при тестировании или демонстрации работы кода.
- При развертывании приложений, с предоставлением разработчику ограниченного доступа в кластер.
- При предоставлении услуг по аренде ресурсов кластера.

## Возможности модуля

- Управление доступом пользователей и групп на базе механизма RBAC Kubernetes (на основе модуля [user-authz](../140-user-authz/)).
- Управление уровнем изоляции конкретных окружений.
- Создание шаблонов для нескольких окружений и кастомизация параметрами по OpenAPI-спецификации.
- Полная совместимость с `Helm` в темплейтах ресурсов.

## Принцип работы

При создании ресурса [Project](cr.html#project) происходит следующее:
- Создается `Namespace` с именем из ресурса [Project](cr.html#project).
- Создается [AuthorizationRule](../140-user-authz/cr.html#authorizationrule) из приведенных данных в поле [subjects](cr.html#projecttype-v1alpha1-spec-subjects) ресурса [ProjectType](cr.html#projecttype).
- Выполняется рендеринг шаблонов (параметр [resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate) ресурса [ProjectType](cr.html#projecttype)) с помощью [Helm](https://helm.sh/docs/). Значения для рендеринга берутся из параметра [template](cr.html#project-v1alpha1-spec-template) ресурса [Project](cr.html#project). При рендеринге выполняется валидация значений согласно OpenAPI-спецификации (параметр [openAPI](cr.html#projecttype-v1alpha1-spec-openapi) ресурса [ProjectType](cr.html#projecttype)).

Так как рендеринг [шаблонов](cr.html#projecttype-v1alpha1-spec-resourcestemplate) выполняется с помощью `Helm`, в шаблоне можно описать любые необходимые объекты Kubernetes, например `NetworkPolicy`, `LimitRange`, `ResourceQuota` и т.п.
