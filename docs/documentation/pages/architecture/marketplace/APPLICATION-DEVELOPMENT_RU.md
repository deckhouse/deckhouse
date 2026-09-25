---
title: Разработка приложений
permalink: ru/architecture/marketplace/application-development.html
description: "Создание пакета Application для Deckhouse Platform Marketplace: бутстрап, структура проекта, package.yaml, требования, локальный рендеринг, проверка, сборка, настройка CI/CD и организация OCI-артефактов в реестре."
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

Чтобы создать директорию `<APPLICATION_NAME>/` в текущей рабочей директории с заготовкой пакета и инициализировать Git-репозиторий с первым коммитом, выполните команду `d8 package bootstrap app <APPLICATION_NAME>` (`application` — псевдоним `app`).

Пример:

```bash
d8 package bootstrap app myapp --hooks
cd myapp
git remote add origin <GITLAB_REPO_URL>
git push --set-upstream origin main
```

Доступные параметры:

| Параметр | Описание |
|---|---|
| `--hooks` | Создать Go-хуки: пример хука и хук валидации настроек |
| `--werf` | Описать сборку образа в файле `werf.inc.yaml` вместо `Dockerfile` |
| `--extended` | Добавить файл `oss.yaml` со списком компонентов с открытым исходным кодом, которые использует приложение |
| `-o, --output <OUTPUT_PATH>` | Путь, в котором будет создан пакет (по умолчанию: `<CURRENT_WORKING_DIRECTORY>/<APPLICATION_NAME>`) |

## Структура проекта

В сгенерированном проекте манифест пакета, схемы, шаблоны, хуки, образы, документация и конфигурация непрерывной интеграции и непрерывной доставки (CI/CD) размещаются в отдельных каталогах и файлах.

```text
myapp/
├── .gitignore
├── .gitlab-ci.yml             # Пайплайн CI/CD.
├── .pkglint.yaml              # Настройки команды d8 package verify.
├── changelog.yaml             # Изменения в версии пакета.
├── docs/
│   ├── README.md              # Документация приложения.
│   ├── README_RU.md
│   ├── CONFIGURATION.md
│   ├── CONFIGURATION_RU.md
│   └── icon.svg               # Иконка приложения.
├── hooks/                     # Go-хуки (--hooks).
│   ├── hooks.yaml             # Инструкции для сборки бинарного файла хуков.
│   └── batch/
│       ├── go.mod
│       ├── go.sum
│       ├── main.go
│       ├── settings/
│       │   └── check.go       # Хук валидации настроек.
│       └── triggers/
│           └── hook.go        # Пример хука.
├── images/                    # Образы контейнеров приложения.
│   └── echo/
│       └── Dockerfile         # werf.inc.yaml при --werf.
├── openapi/
│   ├── settings.yaml          # OpenAPI-схема для Application.spec.settings.
│   ├── doc-ru-settings.yaml   # Описания настроек на русском языке.
│   └── values.yaml            # OpenAPI-схема для значений Helm.
├── oss.yaml                   # Компоненты с открытым исходным кодом (--extended).
├── package.yaml               # Манифест пакета.
└── templates/                 # Helm-шаблоны.
    ├── _helpers/              # Хелперы шаблонов.
    ├── deployment.yaml
    ├── pdb.yaml
    ├── registry-secret.yaml
    ├── service.yaml
    └── vpa.yaml
```

Не добавляйте `Chart.yaml` и `values.yaml` в корень пакета: `d8 package build` не включает эти файлы в bundle пакета. DP рендерит шаблоны как чарт без метаданных, а значения по умолчанию описываются в [OpenAPI-схемах](settings.html).

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
| `requirements.modules` | Нет | Зависимости от модулей (ограничения версий в формате SemVer). См. [«Требования»](#требования) |
| `disable` | Нет | Подтверждение перед удалением приложения. См. [«Подтверждение удаления»](#подтверждение-удаления) |

При сканировании репозитория DP публикует описание, стадию, требования, подтверждение удаления и содержимое `changelog.yaml` в поле `status.packageMetadata` ресурса [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion).

### Требования

Раздел `requirements` определяет условия, при которых приложение можно установить и запустить:

| Поле | Описание |
|---|---|
| `requirements.deckhouse.constraint` | Ограничение на версию DP |
| `requirements.kubernetes.constraint` | Ограничение на версию Kubernetes |
| `requirements.modules.mandatory` | Модули, которые должны быть включены. `constraint` необязателен |
| `requirements.modules.conditional` | Модули, которые не обязательны, но если включены, должны соответствовать `constraint`. `constraint` обязателен |
| `requirements.modules.anyOf` | Группы альтернативных модулей: из каждой группы должен быть включён хотя бы один модуль, соответствующий своему `constraint`, если он задан |
| `requirements.modules.noneOf` | Группы несовместимых модулей: ни один модуль группы не может быть включён. `constraint` сужает диапазон несовместимых версий; без него несовместимы все версии модуля |

У каждой группы `anyOf` и `noneOf` есть уникальное имя `name`, которое используется в сообщениях об ошибках, необязательное описание `description` и непустой список `modules`. Модуль можно указать только в одном из разделов `mandatory`, `conditional`, `anyOf` и `noneOf`. Внутри `anyOf` или `noneOf` один и тот же модуль может входить в несколько групп.

Пример:

```yaml
requirements:
  deckhouse:
    constraint: ">= 1.76"
  kubernetes:
    constraint: ">= 1.31"
  modules:
    mandatory:
      - name: cert-manager
    conditional:
      - name: prometheus
        constraint: ">= 1.60"
    anyOf:
      - name: storage
        description: "A storage for application data"
        modules:
          - name: sds-local-volume
          - name: csi-ceph
    noneOf:
      - name: local-path-storage
        description: "Local path volumes are not supported"
        modules:
          - name: local-path-provisioner
```

DP проверяет требования:

- При создании или изменении Application. Если требования не выполнены, запрос отклоняется, а в сообщении указывается невыполненное требование.
- Перед установкой. DP устанавливает приложение только после того, как включены модули из `requirements.modules.mandatory`.
- Пока приложение установлено. Если требования перестают выполняться, например, отключается обязательный модуль, DP удаляет приложение (см. [«Жизненный цикл и отладка»](lifecycle.html#приостановка)).

Версии модулей сравниваются с ограничениями без частей pre-release и build metadata.

### Подтверждение удаления

Раздел `disable` определяет подтверждение, которое пользователь должен дать перед удалением приложения в веб-интерфейсе:

```yaml
disable:
  confirmation: true
  messages:
    en: "Deleting the application deletes all the data stored in its volumes."
    ru: "Удаление приложения удалит все данные, хранящиеся в его томах."
```

DP публикует раздел в поле `status.packageMetadata.disableOptions` ресурса ApplicationPackageVersion, а веб-интерфейс показывает сообщение на языке пользователя. При удалении Application через API Kubernetes DP подтверждение не проверяет.

### changelog.yaml

Файл `changelog.yaml` описывает изменения в версии пакета:

```yaml
features:
  - "Added support for Redis 7.4."
fixes:
  - "Fixed the readiness probe of the replica."
```

DP публикует содержимое файла в поле `status.packageMetadata.changelog` ресурса ApplicationPackageVersion.

## OpenAPI-схемы

Каталог `openapi/` содержит две схемы:

- `settings.yaml` — схема для `Application.spec.settings` (пользовательская конфигурация). Также поддерживается прежнее имя `config-values.yaml`.
- `values.yaml` — схема для полного набора значений Helm.

Схемы, правила валидации и расширения, управляющие подстановкой значений, неизменяемостью полей и формой настроек в веб-интерфейсе, описаны в разделе [«Настройки приложения»](settings.html).

## Шаблоны и хуки

- Значения, доступные шаблонам, и правила для шаблонов описаны в разделе [«Шаблоны»](templates.html).
- Go-хуки и хук валидации настроек описаны в разделе [«Хуки»](hooks.html).
- Как DP устанавливает, обновляет и удаляет приложение и как его отлаживать, описано в разделе [«Жизненный цикл и отладка»](lifecycle.html).

## Локальный рендеринг

Чтобы отрендерить шаблоны пакета и вывести получившиеся манифесты, выполните в директории пакета команду:

```bash
d8 package render
```

Перед каждым объектом в выводе указывается комментарий с именем файла его шаблона.

Доступные параметры:

| Параметр | Описание |
|---|---|
| `--file <FILE_NAME>` | Вывести только объекты, отрендеренные из шаблона с указанным именем файла |
| `--render-file <PATH>` | Записать манифесты в файл без комментариев с именами файлов шаблонов |
| `-r, --remote <REPOSITORY>/<PACKAGE_NAME>:<PACKAGE_VERSION>` | Отрендерить опубликованный bundle пакета вместо локальной директории. Обязательно указание тега или дайджеста |
| `--remote-user`, `--remote-password` | Учётные данные реестра. Их также можно задать в переменных окружения `PACKAGE_REMOTE_USER` и `PACKAGE_REMOTE_PASSWORD` |

Команда использует значения-заглушки вместо значений из кластера: экземпляр `test` в неймспейсе `default`, версию пакета `dev` и настройки, сгенерированные по `openapi/settings.yaml` (из `x-example`, `x-examples`, `enum` или `default`). Ссылки на образы — заглушки с ключами по именам каталогов образов. Чтобы отрендерить шаблоны установленного приложения с его фактическими значениями, используйте `deckhouse-controller packages render` (см. [«Жизненный цикл и отладка»](lifecycle.html#состояние-в-dp)).

## Проверка пакета

Чтобы проверить структуру, манифесты и схемы пакета, выполните в директории пакета команду:

```bash
d8 package verify
```

Команда выводит ошибки и предупреждения встроенных правил и завершается с ошибкой, если найдена хотя бы одна ошибка. Чтобы получить полный список правил с описаниями, выполните `d8 package doc`.

Правила сгруппированы по линтерам:

| Линтер | Что проверяет |
|---|---|
| `package` | Наличие обязательных файлов (`changelog.yaml`, `docs/`), отсутствие артефактов сборки (`werf.yaml`, `.werf/`, `.helmignore`) и корректность `requirements` в `package.yaml` |
| `openapi` | Типы значений расширений `x-deckhouse-*`, `x-deckhouse-ui-advanced` только у настроек верхнего уровня, значения `enum` в CamelCase и наличие файла `doc-ru-*` для каждой схемы, кроме `values.yaml` |
| `templates` | Имена объектов (префикс `d8a-<INSTANCE_NAME>-`, длина суффикса имён Job и CronJob), отсутствие `metadata.namespace`, объекты PodDisruptionBudget и VerticalPodAutoscaler для рабочих нагрузок и именованный `targetPort` в Service. Шаблоны рендерятся с именем экземпляра `test` в неймспейсе `default` |
| `docs` | Непустой `docs/README.md`, русская версия каждого документа и отсутствие кириллицы в документах на английском языке |
| `images` | Имена каталогов образов без `_` и формат файлов патчей |
| `icon` | Иконка приложения `docs/icon.{png,webp,jpg,jpeg,svg}`: формат, размер до 150 КБ и размеры до 300×300 пикселей |
| `oss` | Формат `oss.yaml`, если файл существует |

Доступные параметры: `--hide-warnings` (не показывать предупреждения), `--show-ignored` (показывать результаты игнорируемых правил), `--lint-config <PATH>` (путь к файлу настроек).

Чтобы проверить опубликованный пакет, выполните `d8 package verify remote <REPOSITORY> <PACKAGE_NAME>`. По умолчанию команда проверяет bundle последней версии. Чтобы выбрать версию, используйте `--version <PACKAGE_VERSION>`, а чтобы проверить также образ метаданных версии — `--release`. Команда использует учётные данные реестра, сохранённые с помощью `d8 dk cr login`.

### .pkglint.yaml

Файл `.pkglint.yaml` понижает строгость правил. Команда ищет его в директории пакета и в родительских директориях (для `verify remote` — в текущей директории).

Пример:

```yaml
version: "1"
static:            # d8 package verify.
  linters:
    templates:
      rules:
        vpa:
          impact: ignored
        service-port:
          impact: warn
remote:            # d8 package verify remote.
  bundle:
    linters:
      docs:
        impact: warn
```

Поле `impact` принимает значения `error`, `warn` и `ignored`. Строгость правила не может быть выше строгости его линтера. Правила линтеров `package` и `openapi` не настраиваются, а правила `instance-prefix`, `instance-namespace` и `job-name` следуют строгости линтера `templates`.

## Локальная сборка

Чтобы собрать пакет и опубликовать его в реестре OCI-артефактов, выполните команду:

```bash
d8 package build -v v0.0.1 -r dev-registry.deckhouse.io/deckhouse/packages
```

Для локальной разработки используйте модуль [`payload-registry`](/modules/payload-registry/) в качестве собственного хранилища образов контейнеров.

Особенности команды:

- В `-r` указывайте корневой путь пакетов — тот же, что в поле `spec.registry.repo` ресурса PackageRepository. Имя пакета команда добавляет к пути сама.
- Указывайте версию в формате `vMAJOR.MINOR.PATCH`. При сканировании репозитория DP находит только версии в этом формате.
- Образы собираются с помощью werf (`d8 delivery-kit`) для платформы `linux/amd64`, поэтому директория пакета должна быть Git-репозиторием. Незакоммиченные изменения тоже попадают в сборку.
- Каталог `images/` должен содержать хотя бы один образ.
- Если версия уже есть в реестре, команда завершается без сборки. Чтобы пересобрать версию, используйте `-f`.

Доступные параметры:

| Параметр | Переменная окружения | Описание |
|---|---|---|
| `-v, --version` | — | Версия пакета (обязательный параметр) |
| `-r, --repo` | `PACKAGE_BUILD_REPOSITORY` | Корневой путь пакетов в реестре. Без него пакет только собирается локально |
| `-u, --user`, `-t, --token` | `PACKAGE_BUILD_REPOSITORY_USER`, `PACKAGE_BUILD_REPOSITORY_TOKEN` | Учётные данные реестра |
| `--final-repo`, `--final-user`, `--final-token` | `PACKAGE_BUILD_FINAL_REPOSITORY`, `PACKAGE_BUILD_FINAL_REPOSITORY_USER`, `PACKAGE_BUILD_FINAL_REPOSITORY_TOKEN` | Путь и учётные данные реестра, в который публикуется пакет, если он отличается от реестра сборки |
| `-f, --force` | — | Пересобрать и опубликовать версию, которая уже есть в реестре |
| `--insecure` | `PACKAGE_BUILD_INSECURE` | Разрешить HTTP и не проверять TLS-сертификаты реестров |
| `--sign`, `--sign-cert`, `--sign-key` | `PACKAGE_BUILD_SIGN_CERT`, `PACKAGE_BUILD_SIGN_KEY` | Подписать образы указанными сертификатом и ключом |

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

Пакет и связанные с ним данные публикуются в OCI-совместимом реестре. Bundle пакета, образы контейнеров и метаданные версий хранятся по отдельным путям.

| Путь | Описание |
|---|---|
| `<REPOSITORY>:<PACKAGE_NAME>` | Тег с именем пакета — используется для получения списка пакетов |
| `<REPOSITORY>/<PACKAGE_NAME>:<PACKAGE_VERSION>` | Bundle — содержит шаблоны, `openapi/`, `hooks/` |
| `<REPOSITORY>/<PACKAGE_NAME>@<DIGEST>` | Образы контейнеров приложения. Шаблоны ссылаются на них по дайджесту (см. [«Шаблоны»](templates.html#образы-контейнеров)) |
| `<REPOSITORY>/<PACKAGE_NAME>/version:<PACKAGE_VERSION>` | Метаданные версии |
| `<REPOSITORY>/<PACKAGE_NAME>/release-channel:<RELEASE_CHANNEL>` | Рекомендуемая версия для канала обновлений |

Здесь `<REPOSITORY>` — корневой путь пакетов, например, `registry.deckhouse.io/deckhouse/<EDITION>/packages`.

### Содержимое bundle

Основной образ bundle (`<PACKAGE_NAME>:<PACKAGE_VERSION>`) содержит:

```text
├── package.yaml         # Манифест пакета с версией.
├── images_digests.json  # Дайджесты образов контейнеров.
├── openapi/             # Схемы settings и values.
├── templates/           # Helm-шаблоны.
├── charts/              # Helm-сабчарты, если есть.
├── hooks/               # Бинарный файл хуков.
├── docs/                # Документация и иконка.
├── changelog.yaml       # История изменений.
└── oss.yaml             # Компоненты с открытым исходным кодом, если есть.
```

### Содержимое образа метаданных

Образ метаданных версии (`<PACKAGE_NAME>/version:<PACKAGE_VERSION>`) содержит:

```text
├── package.yaml       # Манифест пакета.
├── version.json       # Версия в формате SemVer.
├── changelog.yaml     # История изменений.
├── openapi/           # Схемы settings и values.
└── docs/              # Документация и иконка.
```

DP читает метаданные версии при сканировании репозитория и создаёт по ним объекты ApplicationPackageVersion.

### Каналы обновлений

Канал обновлений — это тег пути `<PACKAGE_NAME>/release-channel`, указывающий на копию образа метаданных рекомендуемой версии. DP распознаёт каналы `alpha`, `beta`, `early-access`, `stable`, `rock-solid` и `lts`. `d8 package build` не создаёт теги каналов обновлений — их публикует пайплайн CI/CD.

При сканировании репозитория DP записывает версии, на которые указывают каналы, в поле `status.releaseChannels` ресурса [ApplicationPackage](../../reference/api/cr.html#applicationpackage):

```yaml
status:
  releaseChannels:
    my-registry:        # Имя PackageRepository.
      alpha: v0.2.0
      stable: v0.1.21
```

Каналы носят информационный характер: версия приложения задаётся в `spec.packageVersion` и меняется, только когда её меняет пользователь.
