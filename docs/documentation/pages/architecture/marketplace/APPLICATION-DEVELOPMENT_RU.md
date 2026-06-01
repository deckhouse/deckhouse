---
title: Разработка приложений
permalink: ru/architecture/marketplace/application-development.html
description: "Создание пакета Application для Deckhouse Kubernetes Platform Marketplace: бутстрап, структура проекта, package.yaml, настройка CI/CD, локальная сборка и организация OCI-артефактов в реестре."
lang: ru
search: application development, package.yaml, d8 package, разработка приложения, структура пакета, CI/CD пакета
---

## Предварительные условия

Установите `deckhouse-cli` (`d8`):

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/deckhouse/deckhouse-cli/main/tools/install.sh)"
```

Войдите в реестр пакетов с помощью [лицензионного токена](https://license.deckhouse.io/):

```bash
d8 dk cr login -u license-token dev-registry.deckhouse.io --password <ВАШ_ТОКЕН>
```

## Бутстрап нового Application

`d8 package bootstrap application <name>` создаёт директорию `<name>/` в текущей рабочей директории со скелетом пакета и инициализирует git-репозиторий с первым коммитом.

```bash
d8 package bootstrap application myapp --hooks
cd myapp
git remote add origin <gitlab-repo.git>
git push --set-upstream origin main
```

**Флаги:**

| Флаг | Описание |
|---|---|
| `--hooks` | Сгенерировать скелет Go-хуков |
| `--werf` | Использовать werf для сборки образов |
| `--extended` | Добавить расширенный набор файлов |
| `-o, --output <path>` | Кастомный путь вывода (по умолчанию: `<cwd>/<name>`) |

## Структура проекта

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
│   └── values.yaml         # OpenAPI-схема для Helm values
├── oss.yaml
├── package.yaml            # Манифест пакета
└── templates/              # Helm-шаблоны
    ├── deployment.yaml
    ├── registry-secret.yaml
    └── service.yaml
```

## package.yaml

Центральный манифест пакета Application. Определяет метаданные, тип, требования и совместимость.

```yaml
apiVersion: v1
type: "Application"
name: redis
descriptions:
  ru: "Redis — in-memory база данных"
  en: "Redis — in-memory database"
version: "v1.0.1"      # Инжектируется автоматически при сборке.
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
| `descriptions` | Да | Локализованное описание для каталога и UI (`ru`, `en`) |
| `version` | Да | Semver-версия; инжектируется при сборке |
| `type` | Да | `Application` или `Module` |
| `stage` | Да | Стадия зрелости (`Preview`, `General Availability` и т. д.) |
| `category` | Да | Категория для классификации в каталоге |
| `requirements.deckhouse` | Нет | Ограничение на минимальную версию DKP |
| `requirements.kubernetes` | Нет | Ограничение на минимальную версию Kubernetes |
| `requirements.modules` | Нет | Зависимости от модулей (semver-ограничения) |

## Локальная сборка

Сборка и публикация пакета в реестр:

```bash
d8 package build -v v0.0.1 -r dev-registry.deckhouse.io/deckhouse/packages
```

Для локальной разработки используйте модуль [payload-registry](https://deckhouse.ru/modules/payload-registry/) в качестве личного реестра.

## Линтинг

Проверка структуры и конфигурации пакета:

```bash
d8 package verify
```

Выводит ошибки и предупреждения на основе `.pkglint.yaml` и встроенных правил.

## Настройка CI/CD

### Переменные окружения

| Переменная | Описание |
|---|---|
| `PACKAGES_REGISTRY_LOGIN` | Логин для публикации в реестр |
| `PACKAGES_REGISTRY_PASSWORD` | Пароль или токен реестра |

### Выпуск релиза

Пайплайн запускается по semver git-тегу:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Пайплайн собирает пакет и публикует его в реестр. После завершения пайплайна версия пакета становится доступной для сканирования через PackageRepository.

## Организация OCI-артефактов в реестре

```text
registry.deckhouse.io/deckhouse/<edition>/packages:<name>
    Тег с именем пакета — для поддержки листинга

registry.deckhouse.io/deckhouse/<edition>/packages/<name>:<version>
    Bundle — содержит шаблоны, openapi/, hooks/

registry.deckhouse.io/deckhouse/<edition>/packages/<name>/extra/<image>:<version>
    Дополнительные образы (контейнеры приложения)

registry.deckhouse.io/deckhouse/<edition>/packages/<name>/version:<version>
    Метаданные версии — содержат package.yaml, version.json, changelog.yaml

registry.deckhouse.io/deckhouse/<edition>/packages/<name>/version:<release-channel>
    Рекомендуемая версия для release channel
```

### Содержимое bundle

Основной образ bundle (`<name>:<version>`) содержит:

```text
├── package.yaml       # Манифест пакета
├── openapi/           # Схемы settings и values
├── templates/         # Helm-шаблоны
└── hooks/             # Хуки жизненного цикла
```

### Содержимое образа метаданных

Образ метаданных версии (`<name>/version:<version>`) содержит:

```text
├── package.yaml       # Манифест пакета
├── version.json       # Semver-версия
└── changelog.yaml     # Release notes
```
