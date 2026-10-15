---
title: Концепции
permalink: ru/architecture/marketplace/concepts.html
description: "Основные концепции Marketplace: типы Package, модель CRD, жизненный цикл от сканирования до развёртывания, ограничения Application и ограничения на имена."
lang: ru
search: package types, application constraints, CRD model, типы пакетов, ограничения приложения, модель ресурсов
---

В этом разделе описаны типы пакетов, модель ресурсов, ограничения Application и этапы установки приложения из пакета.

## Типы пакетов

Пакет (package) — это абстракция, объединяющая **Application** и **Module**. Различие определяется областью видимости и назначением:

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
    PR[PackageRepository] -->|Инициирует| PRO[PackageRepositoryOperation]
    PR -->|Заполняет| APV[ApplicationPackageVersion]
    APV -->|Входит в| AP[ApplicationPackage]
    APV -->|Используется в| APP[Application в неймспейсе]
</pre>

| Ресурс | Короткое имя | Область | Роль |
|---|---|---|---|
| [PackageRepository](../../reference/api/cr.html#packagerepository) | — | Кластер | Подключение к хранилищу образов и настройка расписания сканирования |
| [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Кластер | Операция сканирования, которая обнаруживает версии |
| [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) | `apv` | Кластер | Одна на каждую обнаруженную версию пакета. Содержит метаданные, OpenAPI-схемы и требования |
| [ApplicationPackage](../../reference/api/cr.html#applicationpackage) | `ap` | Кластер | Сводная информация о пакете: какие репозитории содержат пакет, на какие версии указывают каналы обновлений, сколько экземпляров его используют |
| [Application](../../reference/api/cr.html#application) | — | Неймспейс | Установленный экземпляр приложения. Управляет развёртыванием через Nelm |

### Содержимое ApplicationPackageVersion

Каждый объект [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) содержит информацию, необходимую для настройки и развёртывания приложения:

| Содержимое | Поле |
| --- | --- |
| Локализованное описание пакета (`en`/`ru`) | `status.packageMetadata.description` |
| Стадия зрелости пакета (`Preview`, `General Availability` и другие) | `status.packageMetadata.stage` |
| Ограничения на версии DP и Kubernetes, а также зависимости от модулей (`mandatory`, `conditional`, `anyOf`, `noneOf`) | `status.packageMetadata.requirements` |
| Параметры подтверждения удаления приложения. Подробнее — [в подразделе «Подтверждение удаления»](application-development.html#подтверждение-удаления) | `status.packageMetadata.disableOptions` |
| Изменения в версии пакета из `changelog.yaml` | `status.packageMetadata.changelog` |
| Схема OpenAPI v3 для проверки `Application.spec.settings` | `status.packageSchemas.settingsSchema` |
| Схема OpenAPI v3 для итоговых values, передаваемых в хуки и шаблоны | `status.packageSchemas.valuesSchema` |

## Этапы установки приложения из пакета

Установка приложения из пакета включает несколько этапов: от подключения и сканирования репозитория пакетов до создания ресурса Application, развёртывания приложения и проверки его состояния.

1. Администратор создаёт [PackageRepository](../../reference/api/cr.html#packagerepository).
2. DP автоматически создаёт [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation). При создании ресурса выполняется первое сканирование репозитория, затем сканирование повторяется с интервалом, указанным [в параметре `scanInterval`](../../reference/api/cr.html#packagerepository-v1alpha1-spec-scaninterval).
3. В ходе операции DP сканирует репозиторий и создаёт объекты [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) для каждой обнаруженной версии.
4. Пользователь создаёт [Application](../../reference/api/cr.html#application) в своём неймспейсе, указывая имя репозитория пакетов ([`packageRepositoryName`](../../reference/api/cr.html#application-v1alpha1-spec-packagerepositoryname)), имя пакета приложения ([`packageName`](../../reference/api/cr.html#application-v1alpha1-spec-packagename)) и версию ([`packageVersion`](../../reference/api/cr.html#application-v1alpha1-spec-packageversion)).
5. DP проверяет `spec.settings` по `settingsSchema` из соответствующего ApplicationPackageVersion.
6. Nelm разворачивает Helm-шаблоны из bundle пакета.
7. Ход развёртывания отражает условие (condition) `Installed` ресурса Application — до завершения установки это единственное условие ресурса. Когда оно становится `True`, появляются остальные условия (`ConfigurationApplied`, `Scaled`, `Ready` и другие).

## Ограничения Application

Все ограничения обеспечивают изоляцию неймспейса и не позволяют приложениям влиять на ресурсы уровня кластера.

### Функциональные ограничения

Применяются следующие ограничения:

1. **Запрет создания CRD** — шаблоны Application не должны содержать объекты CustomResourceDefinition.
2. **Запрет объектов уровня кластера** — все ресурсы, которые создаёт Application, должны быть ресурсами уровня неймспейса.
3. **Нет зависимостей от других приложений** — Application может объявлять зависимости только от модулей (через `requirements.modules` в `package.yaml`), но не от других приложений.
4. **Хуки ограничены неймспейсом** — хуки не должны читать или записывать ресурсы вне своего неймспейса.
5. **Только явная установка** — приложения никогда не включаются по умолчанию; установка требует явного действия пользователя.

### Ограничения на имена

Объекты Application называются по формату `d8a-<INSTANCE_NAME>-<SUFFIX>` (подробнее — [в разделе «Шаблоны»](templates.html#имена-объектов)), поэтому имя экземпляра входит в имя каждого объекта. Чтобы имена объектов укладывались в ограничения Kubernetes, применяются следующие ограничения:

- **имя экземпляра Application** (`metadata.name`) не должно быть длиннее **24 символов**. Validating-вебхук отклоняет Application с более длинным именем;
- **суффикс имени ресурса внутри Application** не должен быть длиннее **23 символов** для StatefulSet, Job и CronJob, имена которых должны укладываться в 52 символа (4 + 24 + 1 + 23 = 52), и не более **34 символов** для остальных объектов, имена которых должны укладываться в 63 символа.

Например, StatefulSet `master` экземпляра `redis-cache` (11 символов) называется `d8a-redis-cache-master` (22 символа).
