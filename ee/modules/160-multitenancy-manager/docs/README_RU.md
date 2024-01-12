---
title: "Модуль multitenancy-manager"
search: multitenancy
description: Модуль multitenancy-manager в Deckhouse предоставляет возможность создания типовых окружений в кластере Kubernetes c помощью ресурсов (custom resources). Рендеринг шаблонов окружений с помощью `Helm` позволяет использовать в шаблоне окружения любые объекты Kubernetes.    
---

Модуль `multitenancy-manager` позволяет настраивать изолированные окружения в кластере Kubernetes.

С помощью модуля `multitenancy-manager` и созданных на основе заранее подготовленного [шаблона](cr.html#projecttype) пользовательских ресурсов [Project](cr.html#project) в кластере Kubernetes можно создавать изолированные друг от друга среды окружение с настроенными правами доступа для пользователей. Дополнительные сведения можно найти в разделе [Примеры](usage.html).

Например, создание изолированных окружений с помощью модуля `multitenancy-manager` может быть удобно в следующих случаях:

* в процессе CI/CD — для создания окружения разработчика при тестировании или демонстрации работы кода;
* при развертывании приложений с предоставлением разработчику ограниченного доступа в кластер;
* при предоставлении услуг по аренде ресурсов кластера.

## Возможности модуля

* Управление доступом для групп и отдельных пользователей на основе ролей RBAC Kubernetes (модуль [user-authz](../140-user-authz/)).
* Управление уровнем изоляции конкретных окружений.
* Создание шаблонов для нескольких сред окружений и настройка параметров кастомизации по OpenAPI-спецификации.
* Полная совместимость с `Helm` в шаблонах ресурсов.

## Принцип работы

При создании ресурса [Project](cr.html#project) происходит следующее:

* создается `Namespace` с именем из ресурса [Project](cr.html#project);
* создается [AuthorizationRule](../140-user-authz/cr.html#authorizationrule) из приведенных данных в поле [subjects](cr.html#projecttype-v1alpha1-spec-subjects) ресурса [ProjectType](cr.html#projecttype);
* выполняется рендеринг (преобразование?) шаблонов (параметр [resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate) ресурса [ProjectType](cr.html#projecttype)) с помощью [Helm](https://helm.sh/docs/). Значения для рендеринга берутся из параметра [template](cr.html#project-v1alpha1-spec-template) ресурса [Project](cr.html#project). При рендеринге выполняется проверка значений по OpenAPI-спецификации (параметр [openAPI](cr.html#projecttype-v1alpha1-spec-openapi) ресурса [ProjectType](cr.html#projecttype)).

Рендеринг [шаблонов](cr.html#projecttype-v1alpha1-spec-resourcestemplate) выполняется с помощью `Helm`, поэтому в шаблоне можно описать все необходимые объекты Kubernetes, например, `NetworkPolicy`, `LimitRange`, `ResourceQuota` и т. п.
