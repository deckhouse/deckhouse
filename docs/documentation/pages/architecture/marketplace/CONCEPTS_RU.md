---
title: Концепции
permalink: ru/architecture/marketplace/concepts.html
description: "Основные концепции Marketplace: типы Package, модель CRD, жизненный цикл от сканирования до развёртывания, ограничения Application и ограничения на имена."
lang: ru
search: package types, application constraints, CRD model, типы пакетов, ограничения приложения, модель ресурсов
---

## Типы Package

**Package** — это абстракция, объединяющая **Application** и **Module**. Различие определяется областью видимости и назначением:

| Характеристика | Module | Application |
|---|---|---|
| **Назначение** | Инфраструктурное расширение кластера | Пользовательская нагрузка |
| **Область видимости** | Кластер (один экземпляр на кластер) | Неймспейс (неограниченное число экземпляров) |
| **Множественные экземпляры** | Нет (1:1 с кластером) | Да (N экземпляров в разных неймспейсах) |
| **Включение по умолчанию** | Может быть включён через bundle | Только по явному действию пользователя |
| **Создание CRD** | Разрешено | Запрещено |
| **Объекты уровня кластера** | Разрешены | Запрещены |

## Модель ресурсов

Marketplace Deckhouse Platform (DP) использует пять кастомных ресурсов:

<script src="/assets/js/mermaid.min.js"></script>
<script>mermaid.initialize({ startOnLoad: true });</script>

<pre class="mermaid">
flowchart TD
    PR[PackageRepository] -->|инициирует| PRO[PackageRepositoryOperation]
    PR -->|заполняет| APV[ApplicationPackageVersion]
    APV -->|входит в| AP[ApplicationPackage]
    APV -->|используется в| APP[Application в неймспейсе]
</pre>

| Ресурс | Короткое имя | Область | Роль |
|---|---|---|---|
| [`PackageRepository`](../../reference/api/cr.html#packagerepository) | — | Cluster | Подключение к хранилищу образов и расписание сканирования |
| [`PackageRepositoryOperation`](../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | Операция сканирования, которая обнаруживает версии |
| [`ApplicationPackageVersion`](../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | Одна на каждую обнаруженную версию пакета; содержит метаданные, OpenAPI-схемы и требования |
| [`ApplicationPackage`](../../reference/api/cr.html#applicationpackage) | `ap` | Cluster | Информационный агрегат: какие репозитории содержат пакет, на какие версии указывают каналы обновлений, сколько экземпляров его используют |
| [`Application`](../../reference/api/cr.html#application) | — | Namespace | Установленный экземпляр; управляет развёртыванием через Nelm |

### Содержимое ApplicationPackageVersion

Каждый объект [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) содержит:

- `status.packageMetadata.description` — локализованное описание пакета (`en`/`ru`)
- `status.packageMetadata.stage` — стадия зрелости (`Preview`, `General Availability` и т. д.)
- `status.packageMetadata.requirements` — ограничения на версии DP и Kubernetes; зависимости от модулей (`mandatory`, `conditional`, `anyOf`, `noneOf`)
- `status.packageMetadata.disableOptions` — подтверждение перед удалением приложения (см. [«Подтверждение удаления»](application-development.html#подтверждение-удаления))
- `status.packageMetadata.changelog` — изменения в версии из `changelog.yaml`
- `status.packageSchemas.settingsSchema` — схема OpenAPI v3 для проверки `Application.spec.settings`
- `status.packageSchemas.valuesSchema` — схема OpenAPI v3 для итоговых values, которые передаются в хуки и шаблоны

## Жизненный цикл от сканирования до развёртывания

1. Администратор создаёт [PackageRepository](../../reference/api/cr.html#packagerepository).
2. DP автоматически создаёт [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation) (первое сканирование — при создании, затем — с интервалом `scanInterval`).
3. Операция сканирует хранилище образов и создаёт объекты [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) для каждой обнаруженной версии.
4. Пользователь создаёт [Application](../../reference/api/cr.html#application) в своём неймспейсе, указывая `packageRepositoryName`, `packageName` и `packageVersion`.
5. DP проверяет `spec.settings` по `settingsSchema` из соответствующего ApplicationPackageVersion.
6. Nelm разворачивает Helm-шаблоны из bundle пакета.
7. Ход развёртывания отражает условие (condition) `Installed` ресурса Application — до завершения установки это единственное условие ресурса. Когда оно становится `True`, появляются остальные условия (`ConfigurationApplied`, `Scaled`, `Ready` и другие).

## Ограничения Application

Все ограничения обеспечивают изоляцию неймспейса и не позволяют приложениям влиять на ресурсы уровня кластера.

### Функциональные ограничения

1. **Запрет создания CRD** — шаблоны Application не должны содержать объекты `CustomResourceDefinition`.
2. **Запрет объектов уровня кластера** — все ресурсы, которые создаёт Application, должны быть ресурсами уровня неймспейса.
3. **Нет зависимостей от других приложений** — Application может объявлять зависимости только от модулей (через `requirements.modules` в `package.yaml`), но не от других приложений.
4. **Хуки ограничены неймспейсом** — хуки не должны читать или записывать ресурсы вне своего неймспейса.
5. **Только явная установка** — приложения никогда не включаются по умолчанию; установка требует явного действия пользователя.

### Ограничения на имена

Объекты Application называются `d8a-<INSTANCE_NAME>-<SUFFIX>` (см. [«Шаблоны»](templates.html#имена-объектов)), поэтому имя экземпляра входит в имя каждого объекта. Чтобы имена объектов укладывались в ограничения Kubernetes:

- **Имя экземпляра Application** (`metadata.name`): не более **24 символов**. Validating-вебхук отклоняет Application с более длинным именем.
- **Суффикс имени ресурса внутри Application**: не более **23 символов** для StatefulSet, Job и CronJob, имена которых должны укладываться в 52 символа (4 + 24 + 1 + 23 = 52), и не более **34 символов** для остальных объектов, имена которых должны укладываться в 63 символа.

Пример: StatefulSet `master` экземпляра `redis-cache` (11 символов) называется `d8a-redis-cache-master` (22 символа).
