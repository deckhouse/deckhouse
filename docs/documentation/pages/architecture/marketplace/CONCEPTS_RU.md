---
title: Концепции
permalink: ru/architecture/marketplace/concepts.html
description: "Основные концепции Marketplace: типы Package, модель CRD, жизненный цикл от сканирования до деплоя, ограничения Application и ограничения на имена."
lang: ru
search: package types, application constraints, CRD model, типы пакетов, ограничения приложения, модель ресурсов
---

## Типы Package

**Package** — это абстракция, объединяющая **Application** и **Module**. Различие определяется областью видимости и назначением:

| Характеристика | Module | Application |
|---|---|---|
| **Назначение** | Инфраструктурное расширение кластера | Пользовательская нагрузка |
| **Область видимости** | Cluster-wide (один на кластер) | Namespaced (неограниченное число экземпляров) |
| **Множественные экземпляры** | Нет (1:1 с кластером) | Да (N экземпляров в разных неймспейсах) |
| **Включение по умолчанию** | Может быть включён через bundle | Только по явному действию пользователя |
| **Создание CRD** | Разрешено | Запрещено |
| **Cluster-wide объекты** | Разрешены | Запрещены |

## Модель ресурсов

Marketplace Deckhouse Kubernetes Platform (DKP) использует пять custom resources:

<script src="/assets/js/mermaid.min.js"></script>
<script>mermaid.initialize({ startOnLoad: true });</script>

<pre class="mermaid">
flowchart TD
    PR[PackageRepository] -->|инициирует| PRO[PackageRepositoryOperation]
    PR -->|заполняет| APV[ApplicationPackageVersion]
    APV -->|агрегирует| AP[ApplicationPackage]
    APV -->|используется| APP[Application\nnamespace-scoped]
</pre>

| Ресурс | Короткое имя | Область | Роль |
|---|---|---|---|
| [`PackageRepository`](../../reference/api/cr.html#packagerepository) | — | Cluster | Подключение реестра и расписание сканирования |
| [`PackageRepositoryOperation`](../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | Задача сканирования, обнаруживающая версии |
| [`ApplicationPackageVersion`](../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | Одна на каждую обнаруженную версию пакета; содержит метаданные, OpenAPI-схемы и требования |
| [`ApplicationPackage`](../../reference/api/cr.html#applicationpackage) | — | Cluster | Информационный агрегат: какие репозитории содержат пакет, сколько экземпляров используют его |
| [`Application`](../../reference/api/cr.html#application) | `app` | Namespace | Установленный экземпляр; управляет деплоем через Nelm |

### Содержимое ApplicationPackageVersion

Каждый объект [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) содержит:

- `status.packageMetadata.description` — локализованное описание пакета (`en`/`ru`)
- `status.packageMetadata.category` — категория в каталоге
- `status.packageMetadata.stage` — стадия зрелости (`Preview`, `General Availability` и т. д.)
- `status.packageMetadata.requirements` — ограничения на версии DKP и Kubernetes; зависимости от модулей (`mandatory`, `conditional`, `anyOf`, `noneOf`)
- `status.packageMetadata.versionCompatibilityRules` — правила совместимости для обновлений и даунгрейдов
- `status.packageSchemas.settingsSchema` — OpenAPI v3 схема для валидации `Application.spec.settings`
- `status.packageSchemas.valuesSchema` — OpenAPI v3 схема для effective values, передаваемых в хуки и шаблоны

## Жизненный цикл от сканирования до деплоя

1. Администратор создаёт [PackageRepository](../../reference/api/cr.html#packagerepository).
2. DKP автоматически создаёт [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation) (первое сканирование при создании, затем каждые `scanInterval`).
3. Операция сканирует реестр и создаёт объекты [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) для каждой обнаруженной версии.
4. Пользователь создаёт [Application](../../reference/api/cr.html#application) в своём неймспейсе, указывая `packageName`, `packageVersion` и опционально `packageRepositoryName`.
5. DKP проверяет `spec.settings` по `settingsSchema` из соответствующего ApplicationPackageVersion.
6. Nelm разворачивает Helm-шаблоны из bundle пакета.
7. Условия (conditions) ресурса Application отражают прогресс деплоя: `Installed` → `ConfigurationApplied` → `Scaled` → `Ready`.

## Ограничения Application

Все ограничения обеспечивают изоляцию неймспейса и предотвращают влияние Applications на ресурсы уровня кластера.

### Функциональные ограничения

1. **Запрет создания CRD** — шаблоны Application не должны содержать объекты `CustomResourceDefinition`.
2. **Запрет cluster-wide объектов** — все ресурсы, создаваемые Application, должны быть namespaced.
3. **Нет зависимостей от других Applications** — Application может объявлять зависимости только от модулей (через `requirements.modules` в `package.yaml`), но не от других Applications.
4. **Хуки ограничены неймспейсом** — хуки не должны читать или записывать ресурсы вне своего неймспейса.
5. **Только явная установка** — Applications никогда не активируются по умолчанию; установка требует явного действия пользователя.

### Ограничения на имена

Kubernetes ограничивает длину имён Pod до 63 символов. Имя пода Application состоит из:

- имени экземпляра — ≤24 символа
- имени ресурса — ≤24 символа
- суффикса Deployment — 15 символов

Поэтому:

- **Имя экземпляра Application** (`metadata.name`): не более **24 символов**
- **Имя ресурса внутри Application** (например, суффикс имени Deployment): не более **24 символов**

Пример: экземпляр `redis-cache` (11 символов) + ресурс `master-deployment` (17 символов) + суффикс (15 символов) = 43 символа — укладывается в ограничение 63 символа.
