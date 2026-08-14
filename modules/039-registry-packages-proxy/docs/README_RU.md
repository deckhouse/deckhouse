---
title: "Модуль registry-packages-proxy"
description: "Внутренний прокси-сервер пакетов registry."
---

Модуль `registry-packages-proxy` предоставляет сервис HTTP-прокси внутри кластера для доступа к пакетам из хранилищ образов контейнеров (container registry). Он выступает в качестве посредника между компонентами кластера и внешними или внутренними хранилищами образов контейнеров с функциями кеширования для оптимизации использования пропускной способности сети и повышения производительности при загрузке пакетов.

Этот модуль — критически важный компонент инфраструктуры, который работает на master-узлах и используется во время загрузки кластера и операций в процессе работы кластера для извлечения пакетов из хранилищ образов контейнеров.

Модуль развёртывает высокодоступный прокси-сервис, который:

- Работает как deployment на master-узлах с включенным `hostNetwork` для обеспечения доступности во время загрузки, когда CNI ещё недоступен.
- Прослушивает порт `4219` (HTTPS) на IP-адресе каждого master-узла.
- Предоставляет эндпоинт `/package` для извлечения пакетов из хранилища образов контейнеров по дайджесту.
- Реализует локальное кеширование извлечённых пакетов (до 1 ГБ) для снижения сетевого трафика и улучшения производительности.
- Следит за кастомными ресурсами [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource) для получения учётных данных хранилищ образов контейнеров.
- Использует `kube-rbac-proxy` для защиты доступа к прокси и эндпоинтам метрик.
- Предоставляет публичный HTTPS API (через Ingress) для исполняемых файлов и плагинов Deckhouse CLI.
- Предоставляет внутрикластерный HTTPS API для иконок пакетов (без публикации через Ingress).

## Архитектура

Прокси-сервис состоит из двух контейнеров:

1. **registry-packages-proxy** — основное приложение прокси, которое:
   - извлекает пакеты из удалённых хранилищ образов контейнеров с использованием дайджестов;
   - кеширует пакеты локально в эфемерном volume (максимум 1 ГБ);
   - поддерживает аутентификацию в хранилищах образов контейнеров через учётные данные из ресурсов [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource);
   - предоставляет проверки работоспособности и метрики Prometheus;
   - прослушивает `127.0.0.1:5080` (HTTP, внутренний).

1. **kube-rbac-proxy** — обеспечивает контроль доступа на основе RBAC:
   - предоставляет сервис на порту `4219` (HTTPS);
   - защищает эндпоинт `/metrics` с авторизацией Kubernetes RBAC;
   - защищает эндпоинт `/package`, требуя соответствующих разрешений;
   - защищает `/v1/images/*` (загрузка Deckhouse CLI) с авторизацией Kubernetes RBAC;
   - позволяет доступ к `/healthz` без аутентификации.

## HTTP API

После завершения развёртывания кластера (bootstrap) и настройки шаблона DNS-имен в параметре [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate), модуль создаёт Ingress для имени `registry-packages-proxy`, используя шаблон из `publicDomainTemplate` (например, `registry-packages-proxy.company.my`, для `publicDomainTemplate: "%s.company.my"`).

Эндпоинты ниже доступны двумя способами:

- **Публично** (через Ingress на публичном имени `registry-packages-proxy`): только `/v1/images/*`.
- **Только изнутри кластера** (через Service `registry-packages-proxy.d8-cloud-instance-manager.svc` на порту `443`, либо через `:4219` на каждом master-узле в фазе bootstrap): все маршруты, включая `/v1/packages/*`.

### Иконки пакетов (`/v1/packages/`) — только изнутри кластера

Иконки пакетов отдаются **без аутентификации** (`kube-rbac-proxy` исключает эти пути из RBAC), но **доступны только изнутри кластера**. Маршрут `/v1/packages/*` намеренно **не публикуется через Ingress**, поэтому публичное имя `registry-packages-proxy.<publicDomain>` иконки не отдаёт.

| Метод | Путь                                                                          | Описание |
|-------|-------------------------------------------------------------------------------|----------|
| `GET`, `HEAD` | `/v1/packages/<РЕПОЗИТОРИЙ-ПАКЕТОВ>/<ИМЯ-ПАКЕТА>/metadata/icon/`         | Иконка последнего semver-тега |
| `GET`, `HEAD` | `/v1/packages/<РЕПОЗИТОРИЙ-ПАКЕТОВ>/<ИМЯ-ПАКЕТА>/metadata/icon`          | То же, что выше |
| `GET`, `HEAD` | `/v1/packages/<РЕПОЗИТОРИЙ-ПАКЕТОВ>/<ИМЯ-ПАКЕТА>/metadata/icon/<ВЕРСИЯ>` | Иконка указанной версии (`<ВЕРСИЯ>` — semver, например `v1.0.1`) |

`<РЕПОЗИТОРИЙ-ПАКЕТОВ>` — это `metadata.name` кастомного ресурса [PackageRepository](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#packagerepository); поле `spec.registry.repo` этого ресурса определяет, по какому пути в хранилище образов прокси читает иконку. Прокси читает иконку из OCI-образа `<spec.registry.repo>/<ИМЯ-ПАКЕТА>:<ТЕГ>`.

Прокси ищет внутри образа пакета следующие файлы в указанном порядке приоритета и возвращает первый найденный:

| Путь                | `Content-Type`    |
|---------------------|-------------------|
| `docs/icon.svg`     | `image/svg+xml`   |
| `docs/icon.png`     | `image/png`       |
| `docs/icon.jpg`     | `image/jpeg`      |
| `docs/icon.jpeg`    | `image/jpeg`      |

Если ни одного из этих файлов нет (или файл больше 4 МиБ), прокси возвращает `404 Not Found`; вызывающий код должен использовать иконку по умолчанию. SVG предпочтительнее, так как не зависит от разрешения.

Пример запроса из пода внутри кластера:

```shell
curl -fsSk "https://registry-packages-proxy.d8-cloud-instance-manager.svc/v1/packages/my-repo/my-module/metadata/icon/"
```

Пример успешного ответа (когда в образе есть `docs/icon.svg`):

```console
Content-Type: image/svg+xml
Content-Disposition: attachment; filename="<ИМЯ-ПАКЕТА>.svg"
```

### Загрузка Deckhouse CLI (`/v1/images/`) — публичный

Эти эндпоинты доступны через публичный Ingress (`registry-packages-proxy.<PUBLIC_DOMAIN>`) и требуют действительный токен Kubernetes или клиентский сертификат, принимаемый `kube-rbac-proxy`. У учётной записи должно быть разрешение RBAC `get` для субресурса `deployments/cli-binary` с именем `registry-packages-proxy` в неймспейсе `d8-cloud-instance-manager`.

Чтобы предоставить необходимое разрешение, используйте ClusterRole `d8:registry-packages-proxy:cli-download`. Подробнее о настройке доступа — в подразделе [«Выдача доступа к загрузке Deckhouse CLI»](#выдача-доступа-к-загрузке-deckhouse-cli).

Это же разрешение используется для загрузки плагинов Deckhouse CLI. Плагины доступны по пути `deckhouse-cli/plugins/<PLUGIN>` через те же эндпоинты `/v1/images/`.

| Метод | Путь                            | Описание |
|-------|---------------------------------|----------|
| `GET` | `/v1/images/<IMAGE>/tags`       | Возвращает JSON со списком тегов |
| `GET`, `HEAD` | `/v1/images/<IMAGE>/images/<VERSION>` | Возвращает OCI-образ в формате `application/x-gzip` (со сведёнными слоями) |
| `GET` | `/v1/images/<IMAGE>/manifests/<REFERENCE>` | Возвращает манифест образа |

Допустимые значения `<IMAGE>`:

- `deckhouse-cli`;
- `deckhouse-cli/plugins/<PLUGIN>`, где `<PLUGIN>` — один сегмент пути.

Эндпоинт `/v1/images/<IMAGE>/images/<VERSION>` принимает необязательный параметр запроса `platform=<OS>-<ARCH>` и выбирает соответствующий дочерний манифест из мультиплатформенного индекса. Если параметр не указан, используется платформа `linux/amd64`.

Пример запроса списка тегов Deckhouse CLI:

```shell
curl -fsS \
  -H "Authorization: Bearer ${TOKEN}" \
  "https://registry-packages-proxy.example.com/v1/images/deckhouse-cli/tags"
```

### Внутренний эндпоинт `/package`

Устаревший эндпоинт `/package?digest=...` (bootstrap и внутренние компоненты) по-прежнему защищён RBAC (subresource `deployments/http`). Через публичный Ingress не публикуется.

## Поток извлечения пакетов

Когда компонент запрашивает пакет:

1. Запрос включает параметр `digest` (обязательный) и необязательные параметры `repository` и `path`.
1. Прокси проверяет локальный кеш на наличие запрошенного дайджеста.
1. Если запрашиваемый пакет есть в кеше, он извлекается из кеша.
1. Если запрашиваемый пакет отсутствует в кеше:
   - Прокси извлекает учётные данные для указанного хранилища образов из отслеживаемых ресурсов [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource).
   - Пакет извлекается из удалённого хранилища образов.
   - Пакет передаётся клиенту с одновременным кешированием для будущих запросов.
1. Ответы включают соответствующие HTTP-заголовки для кеширования (`Cache-Control`, `ETag`, `Content-Length`).

## Высокая доступность

Модуль обеспечивает высокую доступность через:

- Запуск нескольких реплик на master-узлах (в HA-конфигурациях).
- Правила anti-affinity для распределения подов по разным master-узлам.
- PodDisruptionBudget для предотвращения одновременного нарушения работы всех реплик.
- Поддержку Vertical Pod Autoscaler для автоматической настройки ресурсов.

## Роли RBAC, создаваемые модулем

| ClusterRole | Назначение |
|-------------|------------|
| `d8:registry-packages-proxy:cli-download` | Доступ к `/v1/images/*`: исполняемый файл `deckhouse-cli` и его плагины |
| `d8:registry-packages-proxy:packages-download` | Зарезервирована для будущих защищённых маршрутов `/v1/packages/*` (иконки отдаются анонимно, доступ только изнутри кластера) |

Ни одна из ролей не привязана никому по умолчанию.

## Выдача доступа к загрузке Deckhouse CLI

Для работы [Deckhouse CLI](/products/kubernetes-platform/documentation/v1/cli/d8/) необходимо предоставить пользователю права на загрузку исполняемых файлов и получение публичного адреса прокси.

{% alert level="warning" %}
Для создания ClusterRoleBinding или RoleBinding учётная запись должна иметь право назначать соответствующую роль. При необходимости используйте учётную запись с соответствующими правами, например, `/etc/kubernetes/super-admin.conf`.
{% endalert %}

1. Предоставьте доступ к загрузке исполняемых файлов.
   Для этого используйте ClusterRole `d8:registry-packages-proxy:cli-download`.
   Эта роль предоставляет права, необходимые для самообновления Deckhouse CLI (`d8 cli`) и загрузки плагинов (`d8 plugins`).

   Создайте ClusterRoleBinding:

   ```shell
   d8 k create clusterrolebinding d8-cli-download \
     --clusterrole=d8:registry-packages-proxy:cli-download \
     --group=<GROUP>
   ```

   Вместо `--group=<GROUP>` можно предоставить права конкретному пользователю (`--user=<EMAIL>`) или сервисному аккаунту (`--serviceaccount=<NAMESPACE>:<NAME>`).

1. Предоставьте доступ к получению публичного адреса прокси.
   Deckhouse CLI получает публичный адрес прокси из Ingress `registry-packages-proxy`.

   - Создайте Role `d8-cli-ingress`, которая предоставляет разрешение `get` для этого Ingress:

     ```shell
     d8 k -n d8-cloud-instance-manager create role d8-cli-ingress \
       --verb=get \
       --resource=ingresses \
       --resource-name=registry-packages-proxy
     ```

   - Создайте RoleBinding для назначения созданной роли нужной группе:

     ```shell
     d8 k -n d8-cloud-instance-manager create rolebinding d8-cli-ingress \
       --role=d8-cli-ingress \
       --group=<GROUP>
     ```

   Без предоставленного доступа адрес прокси необходимо указывать вручную:

   ```shell
   d8 cli check --rpp-endpoint https://registry-packages-proxy.<PUBLIC_DOMAIN>
   ```

1. Проверьте предоставленные права без входа под соответствующим пользователем.
   Права ограничены через `resourceNames`, поэтому указывайте имя соответствующего ресурса при проверке.

   - Проверьте наличие прав на загрузку исполняемых файлов:

     ```shell
     d8 k auth can-i get deployments/registry-packages-proxy \
       -n d8-cloud-instance-manager \
       --subresource=cli-binary \
       --as=<EMAIL_OR_SERVICE_ACCOUNT>
     ```

   - Проверьте наличие прав на получение Ingress `registry-packages-proxy`:

     ```shell
     d8 k auth can-i get ingresses/registry-packages-proxy \
       -n d8-cloud-instance-manager \
       --as=<EMAIL_OR_SERVICE_ACCOUNT>
     ```

   `Kube-rbac-proxy` кеширует результаты авторизации: отказ — на 30 секунд, разрешение — на 5 минут. Учитывайте это при проверке: после предоставления права ответ `403` может возвращаться ещё до 30 секунд, а после отзыва права доступ может сохраняться до 5 минут.

## Ограничения

- Модуль работает исключительно на master-узлах.
- Требует `hostNetwork: true` для работы во время фазы загрузки.
- Размер кеша ограничен 1 ГБ на под.
- Большинство HTTP-эндпоинтов требуют RBAC Kubernetes; без аутентификации доступны только проверки работоспособности (health check) и иконки пакетов. Иконки пакетов дополнительно ограничены доступом только изнутри кластера (без маршрута через публичный Ingress).
- Иконки читаются из фиксированных путей внутри образа пакета (`docs/icon.svg`, `docs/icon.png`, `docs/icon.jpg`, `docs/icon.jpeg`); SVG предпочтительнее. Максимальный размер иконки — 4 МиБ.
