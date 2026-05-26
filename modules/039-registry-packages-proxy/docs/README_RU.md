---
title: "Модуль registry-packages-proxy"
description: "Внутренний прокси-сервер пакетов registry."
---

Модуль `registry-packages-proxy` предоставляет сервис HTTP-прокси внутри кластера для доступа к пакетам из container registries. Он выступает в качестве посредника между компонентами кластера и внешними или внутренними container registry, предлагая возможности кеширования для оптимизации использования полосы пропускания и улучшения производительности извлечения пакетов.

Этот модуль — критически важный компонент инфраструктуры, который работает на master-узлах и используется во время загрузки кластера и операций во время выполнения для извлечения пакетов из container registries.

Модуль развертывает высокодоступный прокси-сервис, который:

- Работает как deployment на master-узлах с включенным `hostNetwork` для обеспечения доступности во время загрузки, когда CNI еще недоступен.
- Прослушивает порт `4219` (HTTPS) на IP-адресе каждого master-узла.
- Предоставляет эндпоинт `/package` для извлечения пакетов container registry по дайджесту.
- Реализует локальное кеширование извлеченных пакетов (до 1 ГБ) для снижения сетевого трафика и улучшения производительности.
- Следит за кастомными ресурсами [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource) для получения учетных данных container registry для разных репозиториев.
- Использует `kube-rbac-proxy` для защиты доступа к прокси и эндпоинтам метрик.
- Предоставляет публичный HTTPS API (через Ingress) для бинарников Deckhouse CLI и иконок пакетов.

## Архитектура

Прокси-сервис состоит из двух контейнеров:

1. **registry-packages-proxy** — основное приложение прокси, которое:
   - извлекает пакеты из удаленных container registries с использованием дайджестов;
   - кеширует пакеты локально в эфемерном volume (максимум 1 ГБ);
   - поддерживает аутентификацию в container registry через учетные данные из ресурсов [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource);
   - предоставляет проверки работоспособности и метрики Prometheus;
   - прослушивает `127.0.0.1:5080` (HTTP, внутренний).

1. **kube-rbac-proxy** — обеспечивает контроль доступа на основе RBAC:
   - предоставляет сервис на порту `4219` (HTTPS);
   - защищает эндпоинт `/metrics` с авторизацией Kubernetes RBAC;
   - защищает эндпоинт `/package`, требуя соответствующих разрешений;
   - защищает `/v1/images/*` (загрузка Deckhouse CLI) с авторизацией Kubernetes RBAC;
   - позволяет доступ к `/healthz` без аутентификации;

## Публичный HTTP API (Ingress)

После завершения bootstrap кластера и настройки `publicDomainTemplate` модуль создаёт Ingress с хостом:

`registry-packages-proxy.<publicDomainTemplate>`

Все пути ниже доступны по HTTPS через этот хост (а также на порту `4219` каждого master-узла для доступа изнутри кластера).

### Иконки пакетов (`/v1/packages/`)

Иконки пакетов **публичные**: заголовок `Authorization: Bearer` и RBAC Kubernetes не требуются.

| Метод | Путь | Описание |
|-------|------|----------|
| `GET`, `HEAD` | `/v1/packages/<имя-пакета>/metadata/icon/` | Иконка последнего semver-тега |
| `GET`, `HEAD` | `/v1/packages/<имя-пакета>/metadata/icon` | То же, что выше |
| `GET`, `HEAD` | `/v1/packages/<имя-пакета>/metadata/icon/<версия>` | Иконка указанной версии (`<версия>` — semver, например `v1.0.1`) |

Прокси читает файл `docs/icon.svg` из OCI-образа `packages/<имя-пакета>:<тег>` в registry кластера (учётные данные задаются через [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource)).

Успешный ответ: `Content-Type: image/svg+xml` и `Content-Disposition: attachment; filename="<имя-пакета>.svg"`.

Пример:

```shell
curl -fsS "https://registry-packages-proxy.example.com/v1/packages/my-module/metadata/icon/"
```

### Загрузка Deckhouse CLI (`/v1/images/`)

Для этих эндпоинтов нужен действительный токен Kubernetes (или клиентский сертификат, принимаемый `kube-rbac-proxy`) и право RBAC `get` на subresource `deployments/cli-binary` с именем `registry-packages-proxy` в namespace `d8-cloud-instance-manager`.

Выдайте доступ через ClusterRole `d8:registry-packages-proxy:cli-download` (привяжите к пользователям или ServiceAccount через `ClusterRoleBinding` или `RoleBinding`).

| Метод | Путь | Описание |
|-------|------|----------|
| `GET` | `/v1/images/<образ>/tags` | JSON со списком тегов |
| `GET`, `HEAD` | `/v1/images/<образ>/tags/<тег>` | OCI-образ в формате `application/x-gzip` (слои сведены) |

Допустимые значения `<образ>`:

- `deckhouse-cli`
- `deckhouse-cli/plugins/<плагин>` (один сегмент пути для `<плагин>`)

Пример:

```shell
curl -fsS -H "Authorization: Bearer ${TOKEN}" \
  "https://registry-packages-proxy.example.com/v1/images/deckhouse-cli/tags"
```

### Внутренний эндпоинт `/package`

Устаревший эндпоинт `/package?digest=...` (bootstrap и внутренние компоненты) по-прежнему защищён RBAC (subresource `deployments/http`). Через публичный Ingress не публикуется.

## Поток извлечения пакетов

Когда компонент запрашивает пакет:

1. Запрос включает параметр `digest` (обязательный) и опциональные параметры `repository` и `path`.
1. Прокси проверяет локальный кеш на наличие запрошенного дайджеста.
1. Если запрашиваемый пакет есть в кеше, он извлекается из кеша.
1. Если запрашиваемый пакет отсутствует в кеше:
   - Прокси извлекает учетные данные для указанного репозитория из отслеживаемых ресурсов [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource).
   - Пакет извлекается из удаленного container registry.
   - Пакет передается клиенту с одновременным кешированием для будущих запросов.
1. Ответы включают соответствующие HTTP-заголовки для кеширования (`Cache-Control`, `ETag`, `Content-Length`).

## Высокая доступность

Модуль обеспечивает высокую доступность через:

- Запуск нескольких реплик на master-узлах (в HA-конфигурациях).
- Правила anti-affinity для подов для распределения подов по разным master-узлам.
- PodDisruptionBudget для предотвращения одновременного нарушения работы всех реплик.
- Поддержку Vertical Pod Autoscaler для автоматической настройки ресурсов.

## Роли RBAC, создаваемые модулем

| ClusterRole | Назначение |
|-------------|------------|
| `d8:registry-packages-proxy:cli-download` | Доступ к `/v1/images/*` |
| `d8:registry-packages-proxy:packages-download` | Зарезервирована для будущих защищённых маршрутов `/v1/packages/*` (иконки публичные) |

## Ограничения

- Модуль работает исключительно на master-узлах.
- Требует `hostNetwork: true` для работы во время фазы загрузки.
- Размер кеша ограничен 1 ГБ на под.
- Большинство HTTP-эндпоинтов требуют RBAC Kubernetes; без аутентификации доступны только health check и иконки пакетов.
- Иконки отдаются только как SVG из фиксированного пути `docs/icon.svg` внутри образа пакета.
