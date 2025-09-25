---
title: Мультитенантность
permalink: ru/virtualization-platform/documentation/architecture//multitenancy/
lang: ru
---

## Внутренняя логика работы

### Создание проекта

Для создания проекта используются следующие кастомные ресурсы:

* [ProjectTemplate](/products/virtualization-platform/reference/cr/project.html) — описывает шаблон проекта. Задается список ресурсов, которые будут созданы в проекте, а также схему параметров, которые можно передать при создании проекта;
* [Project](/products/virtualization-platform/reference/cr/projecttemplate.html) — описывает конкретный проект.

При создании Project из определенного ProjectTemplate происходит следующее:

1. Переданные [параметры](/products/virtualization-platform/reference/cr/project.html#project-v1alpha2-spec-parameters) валидируются по OpenAPI-спецификации (параметр [openAPI](/products/virtualization-platform/reference/cr/projecttemplate.html#projecttemplate-v1alpha1-spec-parametersschema) ресурса [ProjectTemplate](/products/virtualization-platform/reference/cr/projecttemplate.html));
1. Выполняется рендеринг [шаблона для ресурсов](/products/virtualization-platform/reference/cr/projecttemplate.html#projecttemplate-v1alpha1-spec-resourcestemplate) с помощью [Helm](https://helm.sh/docs/). Значения для рендеринга берутся из параметра [parameters](/products/virtualization-platform/reference/cr/project.html#project-v1alpha2-spec-parameters) ресурса [Project](/products/virtualization-platform/reference/cr/project.html#project);
1. Создаётся пространство имён с именем, которое совпадает c именем [Project](/products/virtualization-platform/reference/cr/project.html#project);
1. По очереди создаются все ресурсы, описанные в шаблоне.

{% alert level="warning" %}
При изменении шаблона проекта, все созданные проекты будут обновлены в соответствии с новым шаблоном.
{% endalert %}

### Изоляция проекта

В основе проекта используется механизм изоляции ресурсов в рамках пространства имён.
Пространства имён позволяют группировать поды, сервисы, секреты и другие объекты, но не обеспечивают полноценной изоляции.
Проект расширяет функциональность пространств имен, предлагая дополнительные инструменты для повышения уровня контроля и безопасности.
Для управления уровнем изоляции проекта можно использовать возможности Kubernetes, например:

* **Ресурсы контроля доступа** (AuthorizationRule / RoleBinding) — позволяют управлять взаимодействием объектов внутри пространства имён. С их помощью можно задавать правила и назначать роли, чтобы точно контролировать, кто и что может делать в проекте.
* **Ресурсы контроля использования нагрузки** (ResourceQuota) — с их помощью можно задать лимиты на использование процессорного времени (CPU), оперативной памяти (RAM), а также количества объектов внутри пространства имён. Это помогает избежать чрезмерной нагрузки и обеспечивает мониторинг за приложениями в рамках проекта.
* **Ресурсы контроля сетевой связности** (NetworkPolicy) — управляют входящим и исходящим сетевым трафиком в пространстве имён. Таким образом, можно настроить разрешенные подключения между подами, улучшить безопасность и управляемость сетевого взаимодействия в рамках проекта.

Эти инструменты можно комбинировать, чтобы настроить проект в соответствии с требованиями вашего приложения.
