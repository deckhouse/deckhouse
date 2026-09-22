---
title: Разработка приложений
permalink: ru/architecture/marketplace/application-development.html
description: "Создание пакета Application для Deckhouse Platform Marketplace: бутстрап, структура проекта, package.yaml, настройка CI/CD, локальная сборка и организация OCI-артефактов в реестре."
lang: ru
search: application development, package.yaml, d8 package, разработка приложения, структура пакета, CI/CD пакета
---

## Предварительные условия

Установите `deckhouse-cli` (`d8`):

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/deckhouse/deckhouse-cli/main/tools/install.sh)"
```

Войдите в реестр OCI-артефактов с помощью [лицензионного токена](https://license.deckhouse.io/):

```bash
d8 dk cr login -u license-token dev-registry.deckhouse.io --password <LICENSE_TOKEN>
```

## Бутстрап пакета Application

Чтобы создать директорию `<APPLICATION_NAME>/` в текущей рабочей директории с заготовкой пакета и инициализировать Git-репозиторий с первым коммитом, выполните команду `d8 package bootstrap application <APPLICATION_NAME>`.

Пример:

```bash
d8 package bootstrap application myapp --hooks
cd myapp
git remote add origin <GITLAB_REPO_URL>
git push --set-upstream origin main
```

Доступные параметры:

| Параметр | Описание |
|---|---|
| `--hooks` | Создать заготовки Go-хуков |
| `--werf` | Использовать werf для сборки образов |
| `--extended` | Добавить расширенный набор файлов |
| `-o, --output <OUTPUT_PATH>` | Путь, в котором будет создан пакет (по умолчанию: `<CURRENT_WORKING_DIRECTORY>/<APPLICATION_NAME>`) |

## Структура проекта

В сгенерированном проекте манифест пакета, схемы, шаблоны, хуки, образы, документация и конфигурация непрерывной интеграции и непрерывной доставки (CI/CD) размещаются в отдельных каталогах и файлах.

```text
myapp/
├── .gitignore
├── .gitlab-ci.yml          # Пайплайн CI/CD
├── changelog.yaml
├── docs/
│   └── README.md           # Документация приложения
├── hooks/                  # Go-хуки
│   ├── hooks.yaml
│   └── batch/
│       ├── go.mod
│       ├── go.sum
│       ├── main.go
│       └── triggers/
│           └── hook.go
├── images/                 # Исходный код образов или инструкции для их загрузки
│   └── myapp/
│       └── werf.inc.yaml
├── openapi/
│   ├── config-values.yaml  # OpenAPI-схема для Application.spec.settings
│   └── values.yaml         # OpenAPI-схема для значений Helm
├── oss.yaml
├── package.yaml            # Манифест пакета
└── templates/              # Helm-шаблоны
    ├── deployment.yaml
    ├── registry-secret.yaml
    └── service.yaml
```

## package.yaml

Файл `package.yaml` — основной манифест пакета Application. Он определяет метаданные, тип, требования и совместимость.

Пример файла `package.yaml`:

```yaml
apiVersion: v1
type: "Application"
name: redis
descriptions:
  ru: "Redis — in-memory база данных"
  en: "Redis — in-memory database"
# Добавляется автоматически при сборке.
version: "v1.0.1"
stage: "Preview"
category: "Databases"
# Требования к окружению.
requirements:
  deckhouse:
    constraint: ">= 1.70"
  kubernetes:
    constraint: ">= 1.31"
  modules:
    mandatory:
      - name: cert-manager
        constraint: ">= 1.0.0"
```

**Справочник полей:**

| Поле | Обязательное | Описание |
|---|---|---|
| `name` | Да | Уникальное имя пакета |
| `descriptions` | Да | Локализованное описание для каталога и пользовательского интерфейса (UI) (`ru`, `en`) |
| `version` | Да | Версия в формате семантического версионирования (SemVer); добавляется автоматически при сборке |
| `type` | Да | `Application` или `Module` |
| `stage` | Да | Стадия зрелости (`Preview`, `General Availability` и т. д.) |
| `category` | Да | Категория для классификации в каталоге |
| `requirements.deckhouse` | Нет | Ограничение на минимальную версию Deckhouse Platform (DP) |
| `requirements.kubernetes` | Нет | Ограничение на минимальную версию Kubernetes |
| `requirements.modules` | Нет | Зависимости от модулей (ограничения версий в формате SemVer) |

## OpenAPI-схемы

Каталог `openapi/` содержит две схемы:

- `config-values.yaml` (или `settings.yaml`) — схема для `Application.spec.settings` (пользовательская конфигурация);
- `values.yaml` — схема для полного набора значений Helm.

### Подстановка значения ресурса кластера, доступного по гранту (x-deckhouse-grantable-resource)

Поле `settings` типа `string` можно связать с cluster-wide-ресурсом, которым управляет
[`multitenancy-manager`](/modules/multitenancy-manager/) и который доступен проекту по гранту (например, StorageClass).

Когда поле связано и пользователь оставляет его пустым, в `values` подставляется имя ресурса, заданное для проекта как значение по умолчанию. При указании собственного значения оно проверяется по списку ресурсов, доступных проекту. Значение, которого в этом списке нет, отклоняется.

Чтобы связать поле с cluster-wide-ресурсом, добавьте к полю расширение `x-deckhouse-grantable-resource` и укажите имя ресурса, доступного проекту по гранту
(AvailableClusterResource или GrantableClusterResourceDefinition), например `storageclasses`.

{% alert level="info" %}
Группа, версия и тип ресурса (GVK) задаются в гранте. Указывать их в `openapi/settings.yaml` не нужно.
{% endalert %}

Пример `openapi/settings.yaml` с `x-deckhouse-grantable-resource`:

```yaml
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-grantable-resource: storageclasses
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-grantable-resource: postgresclasses
```

Поведение:

- Значение по умолчанию определяется для каждого проекта из AvailableClusterResource в неймспейсе приложения, поэтому разные проекты могут получать разные значения по умолчанию.
- Явно заданное пользователем значение имеет более высокий приоритет, чем значение по умолчанию для проекта.
- Если отсутствует определение кастомного ресурса (CRD), для проекта не создан каталог или в каталоге не задано значение по умолчанию, поле остаётся без изменений. Значение не подставляется и не проверяется.

### Неизменяемые поля (x-deckhouse-immutable)

Некоторые настройки не следует менять после применения конфигурации приложения: например, смена `storageClass` после создания томов либо ни на что не влияет, либо может привести к ошибкам в работе приложения. Чтобы запретить изменение значения такого поля после успешного применения конфигурации приложения, добавьте к нему расширение `x-deckhouse-immutable: true`.

Пример `openapi/settings.yaml` с `x-deckhouse-immutable`:

```yaml
type: object
properties:
  storageClass:
    type: string
    default: default
    x-deckhouse-immutable: true
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-immutable: true
      volumeSize:
        type: string
```

Поведение:

- Расширение действует только при значении `true`. Любое другое значение расширения игнорируется.
- Если `x-deckhouse-immutable` добавлено к объекту, неизменяемым становится весь объект. После первого успешного применения конфигурации приложения изменение любого вложенного поля будет отклонено, даже если для него отдельно не указано `x-deckhouse-immutable`. Указывайте расширение на уровне объекта, только если необходимо запретить изменение всего объекта. Чтобы запретить изменение только одного поля, добавьте `x-deckhouse-immutable` непосредственно к нему, как к `postgres.storageClass` в примере выше. В этом случае ограничение не распространяется на `postgres.volumeSize`, и его значение можно изменять.
- Обновление, меняющее зафиксированное значение, отклоняется validating-вебхуком с указанием имени поля.
- В веб-интерфейсе значение поля с расширением можно задать при установке приложения. В форме редактирования установленного приложения поле доступно только для чтения.
- Сравнение выполняется с фактически применённой конфигурацией после подстановки значений по умолчанию из схемы. Поэтому поле с расширением можно удалить из манифеста только в том случае, если его `default` совпадает с уже применённым значением. Если значения по умолчанию нет или оно отличается, изменение отклоняется.

{% alert level="info" %}
Если удалить из манифеста весь объект, значения по умолчанию для его вложенных полей не подставляются. Поэтому удаление объекта с полями, использующими `x-deckhouse-immutable`, может привести к потере зафиксированных значений и будет отклонено.
{% endalert %}

- Метка не распространяется на элементы массивов и записи карт, которые обновление добавляет: у нового элемента нет предыдущего значения, относительно которого его можно фиксировать.

## Локальная сборка

Чтобы собрать пакет и опубликовать его в реестре OCI-артефактов, выполните команду:

```bash
d8 package build -v v0.0.1 -r dev-registry.deckhouse.io/deckhouse/packages
```

Для локальной разработки используйте модуль [`payload-registry`](/modules/payload-registry/) в качестве собственного хранилища образов контейнеров.

## Проверка пакета

Чтобы проверить структуру и конфигурацию пакета, выполните команду:

```bash
d8 package verify
```

Команда выводит ошибки и предупреждения на основе `.pkglint.yaml` и встроенных правил.

## Настройка CI/CD

Пайплайн CI/CD публикует релизы пакета в реестре OCI-артефактов. Для публикации релиза необходимо настроить учётные данные реестра OCI-артефактов, а затем создать Git-тег в формате SemVer и отправить его в репозиторий.

### Переменные окружения

Для аутентификации в реестре OCI-артефактов пайплайн использует следующие переменные:

| Переменная | Описание |
|---|---|
| `PACKAGES_REGISTRY_LOGIN` | Имя пользователя для публикации в реестр OCI-артефактов |
| `PACKAGES_REGISTRY_PASSWORD` | Пароль или токен реестра OCI-артефактов |

### Выпуск релиза

Пайплайн запускается по Git-тегу в формате SemVer:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Пайплайн собирает пакет и публикует его в реестр OCI-артефактов. После завершения пайплайна версия пакета становится доступной для сканирования через PackageRepository.

## Организация OCI-артефактов в реестре

Пакет и связанные с ним данные публикуются в OCI-совместимом реестре. Bundle пакета, дополнительные образы и метаданные версий хранятся по отдельным путям.

| Путь | Описание |
|---|---|
| `registry.deckhouse.io/deckhouse/<EDITION>/packages:<PACKAGE_NAME>` | Тег с именем пакета — используется для получения списка пакетов |
| `registry.deckhouse.io/deckhouse/<EDITION>/packages/<PACKAGE_NAME>:<PACKAGE_VERSION>` | Bundle — содержит шаблоны, `openapi/`, `hooks/` |
| `registry.deckhouse.io/deckhouse/<EDITION>/packages/<PACKAGE_NAME>/extra/<IMAGE_NAME>:<PACKAGE_VERSION>` | Дополнительные образы (контейнеры приложения) |
| `registry.deckhouse.io/deckhouse/<EDITION>/packages/<PACKAGE_NAME>/version:<PACKAGE_VERSION>` | Метаданные версии — содержат `package.yaml`, `version.json`, `changelog.yaml` |
| `registry.deckhouse.io/deckhouse/<EDITION>/packages/<PACKAGE_NAME>/version:<RELEASE_CHANNEL>` | Рекомендуемая версия для канала обновлений |

### Содержимое bundle

Основной образ bundle (`<PACKAGE_NAME>:<PACKAGE_VERSION>`) содержит:

```text
├── package.yaml       # Манифест пакета
├── openapi/           # Схемы settings и values
├── templates/         # Helm-шаблоны
└── hooks/             # Хуки жизненного цикла
```

### Содержимое образа метаданных

Образ метаданных версии (`<PACKAGE_NAME>/version:<PACKAGE_VERSION>`) содержит:

```text
├── package.yaml       # Манифест пакета
├── version.json       # Версия в формате SemVer
└── changelog.yaml     # История изменений
```
