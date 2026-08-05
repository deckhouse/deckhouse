---
title: "Расширенный поиск (администрирование)"
menuTitle: Расширенный поиск (администрирование)
searchable: true
description: Настройка и эксплуатация расширенного поиска на базе OpenSearch в Deckhouse Code
permalink: ru/code/documentation/admin/configuration/advanced-search.html
lang: ru
weight: 55
relatedLinks:
  - title: "Расширенный поиск — руководство пользователя"
    url: ../../user/search.html
# После публикации страницы advanced-search в документации модуля code
# добавить сюда пункт «Расширенный поиск — документация модуля code».
---

Документация для администратора по расширенному поиску в Deckhouse Code: эксплуатация индексации после подключения OpenSearch.
<!-- После публикации страницы advanced-search в документации модуля code вернуть сюда предложение
со ссылкой на неё: «Подключение и настройка — в документации модуля code».
Ссылку не оформлять markdown-синтаксисом внутри комментария: сборщик связанных ссылок
не вырезает HTML-комментарии и утащит её в блок «Дополнительные ресурсы». -->
Инструкции для пользователей — в [руководстве пользователя](../../user/search.html).

## Эксплуатация

Управление индексацией, мониторинг и устранение неполадок.

### Управление на уровне инстанса

Чтобы управлять индексацией на уровне инстанса, перейдите в «Admin» → «Настройки» → «Поиск».

Раздел доступен после подключения OpenSearch.

#### Приостановка индексации

Установите флаг «Приостановить индексирование OpenSearch», чтобы приостановить фоновые задачи индексации и переиндексации.
Чтобы снова включить индексацию, отключите флаг «Приостановить индексирование OpenSearch». После этого Sidekiq pause control автоматически возобновит работу в течение нескольких минут.

#### Режим индексации веток

В проектах могут использоваться следующие режимы индексации веток:

| Режим | Описание |
|-------|----------|
| **Только ветка по умолчанию** | Индексируется только ветка по умолчанию для всех проектов |
| **Разрешить регулярное выражение для веток на уровне проекта** | Для проектов можно задать regex для индексации дополнительных веток |

{% alert level="warning" %}
После смены режима индексации веток выполните ручную переиндексацию кода.
Результаты поиска могут быть неполными до завершения индексации.
{% endalert %}

#### Статус индексов OpenSearch

На странице отображается таблица индексов:

| Суффикс индекса | Область поиска |
|-----------------|----------------|
| `code` | Код (`blobs`) |
| `commits` | Коммиты |
| `wiki` | Wiki-страницы |
| `notes` | Комментарии |
| `milestones` | Этапы (milestones) |
| `merge-requests` | Запросы на слияние |
| `work-items` | Задачи (work items) |

Имя индекса в OpenSearch состоит из общего префикса инстанса и суффикса из таблицы, например `deckhouse-development-code`.

Для каждого индекса показаны: имя в OpenSearch, наличие, количество документов, состояние индекса.

#### Операции над индексами

На той же странице доступны следующие операции:

- **Переиндексировать** — переиндексация одного индекса (удаляет существующие документы и ставит фоновые задачи).
- **Переиндексировать все индексы** — переиндексация всех индексов.

{% alert level="warning" %}
Операция «Переиндексировать» удаляет существующие документы.
Результаты поиска могут быть неполными до завершения фоновой индексации.
{% endalert %}

Индекс `commits` переиндексируется вместе с индексом `code`: отдельной операции для коммитов нет.
По этой же причине у коммитов нет отдельного значения `schema_class`.

#### Admin API

Те же операции доступны через административный REST API:

- `POST /api/v4/admin/opensearch/recreate_indices` — пересоздание индексов;
- `GET /api/v4/admin/opensearch/indexing_queue_stats` — статистика очереди индексации.

Права доступа, допустимые значения `schema_class`, коды ответов и примеры запросов описаны в разделе [«API OpenSearch»](#api-opensearch).

### Мониторинг

#### Метрики

Обращения к OpenSearch учитываются в Prometheus отдельно для HTTP-запросов (поиск в UI и API) и для фоновых задач Sidekiq (индексация).
Имена метрик содержат `elasticsearch` — это историческое название в GitLab, метрики относятся к OpenSearch.

**HTTP-запросы** (поиск пользователей):

| Метрика | Описание |
|---------|----------|
| `http_elasticsearch_requests_total` | Число обращений к OpenSearch за один HTTP-запрос |
| `http_elasticsearch_requests_duration_seconds` | Суммарное время обращений к OpenSearch за один HTTP-запрос |
| `http_elasticsearch_requests_failed_total` | Число неудачных обращений за один HTTP-запрос (ошибки подключения или авторизации) — **добавлено в Deckhouse Code** |

**Sidekiq** (фоновая индексация):

| Метрика | Описание |
|---------|----------|
| `sidekiq_elasticsearch_requests_total` | Число обращений к OpenSearch за выполнение одного Sidekiq-задания |
| `sidekiq_elasticsearch_requests_duration_seconds` | Суммарное время обращений к OpenSearch за выполнение одного Sidekiq-задания |
| `sidekiq_elasticsearch_requests_failed_total` | Число неудачных обращений за выполнение одного Sidekiq-задания (ошибки подключения или авторизации) — **добавлено в Deckhouse Code** |

**Индексатор репозиториев** (`Search::RepositoryIndexerWorker` — код, коммиты, wiki):

| Метрика | Лейблы | Описание |
|---------|--------|----------|
| `search_repository_indexer_starts_total` | `indexer_class` | Число запусков индексации после прохождения проверки `advanced_search_enabled` |
| `search_repository_indexer_runs_total` | `outcome`, `indexer_class` | Число завершённых запусков после получения exclusive lock (`outcome`: `success` или `error`) |
| `search_repository_indexer_duration_seconds` | `outcome`, `indexer_class` | Длительность фазы индексации под exclusive lock |
| `search_repository_indexer_lock_contention_total` | — | Число случаев, когда lock не получен и задание перенесено |

Значение `indexer_class` — тип выполняемой индексации:

| `indexer_class` | Когда используется |
|-----------------|-------------------|
| `Search::RepositoryIndexer::IncrementalIndexService` | Инкрементальная индексация после изменений в репозитории |
| `Search::RepositoryIndexer::FullIndexService` | Полная переиндексация (force) |
| `Search::RepositoryIndexer::MaintainsService` | Обновление индекса по событию |
| `Search::RepositoryIndexer::DeleteService` | Удаление документов из индекса (пустой или удалённый репозиторий/wiki) |

Рост `search_repository_indexer_lock_contention_total` — признак конкуренции за lock между заданиями одного проекта.
Рост `search_repository_indexer_runs_total{outcome="error"}` — ошибки go-indexer или сервисов индексации; детали в логах Sidekiq.

Метрики `*_failed_total` увеличиваются при ошибках подключения к OpenSearch или ошибках авторизации.
Рост `*_failed_total` указывает на недоступность OpenSearch или на использование неверных данных для доступа.
Рост `*_duration_seconds` при стабильном `*_total` — на медленные ответы OpenSearch.

Для мониторинга индексации репозиториев ориентируйтесь на `search_repository_indexer_*`, для обращений к OpenSearch из Sidekiq — на `sidekiq_elasticsearch_*`.
Для мониторинга пользовательского поиска — на `http_elasticsearch_*`.

На странице «Admin» → «Настройки» → «Поиск» виджет прогресса индексации показывает число оставшихся задач переиндексации.
Те же данные доступны через эндпоинт [`indexing_queue_stats`](#get-apiv4adminopensearchindexing_queue_stats).

#### Очередь Sidekiq

Задачи индексации OpenSearch выполняются в отдельной очереди `global-search-indexing`, а не в общей очереди `default`.
Маршрутизация настраивается правилом Sidekiq: все воркеры с категорией `fe_global_search` попадают в эту очередь.
Отдельная очередь изолирует нагрузку индексации от остальных фоновых задач Deckhouse Code.

#### Cron-задачи

Для автоматизации индексации регулярно выполняются следующие cron-задачи:

| Расписание | Назначение |
|------------|------------|
| Каждую минуту | Индексация комментариев — обрабатывает накопившуюся очередь изменений notes |
| Ежедневно в 03:00 | Запускает индексацию проектов |

Cron-задачи не выполняют индексацию напрямую: они запускают или возобновляют соответствующие воркеры в очереди `global-search-indexing`.

#### Логи

Задачи индексации OpenSearch пишутся в логи Sidekiq. Для фильтрации используйте имя очереди `global-search-indexing`.

### Устранение неполадок

#### OpenSearch недоступен

- Проверьте настройки подключения. Подробнее — в документации модуля `code`.
- На странице «Admin» → «Настройки» → «Поиск» появится сообщение о невозможности подключения.
- Поиск вернёт ошибку.

#### Неполные результаты поиска

- Дождитесь завершения фоновой индексации (виджет прогресса на странице «Admin» → «Настройки» → «Поиск»).
- Запустите переиндексацию на уровне проекта или операцию «Переиндексировать» для нужного индекса в «Admin» → «Настройки» → «Поиск».

#### Задачи индексации не появляются

Если новые задачи не ставятся в очередь `global-search-indexing`:

1. Проверьте, не включена ли пауза (включён флаг «Приостановить индексирование OpenSearch») в «Admin» → «Настройки» → «Поиск». Снимите флаг и дождитесь возобновления (cron-задача выполняется каждые 5 минут).
1. Если пауза снята, а задачи по-прежнему не появляются, выполните очистку Redis — возможны зависшие lease или duplicate-ключи Sidekiq:

   ```shell
   bundle exec rails runner fe/scripts/clear_search_opensearch_worker_redis.rb
   ```

   Скрипт снимает exclusive lease для `Search::RepositoryIndexerWorker`, concurrency limit и dedup-ключи очереди `global-search-indexing`.

## API OpenSearch

В этом разделе описаны административные OpenSearch-эндпоинты Deckhouse Code.
Параметры пользовательского поиска — [в разделе «API поиска»](../../user/search.html#api-поиска).

### Права доступа

- `POST /api/v4/admin/opensearch/recreate_indices` — только администратор (`authenticated_as_admin!`);
- `GET /api/v4/admin/opensearch/indexing_queue_stats` — пользователь с правом `read_admin_search_indexing_queue_stats` на `:global`.

### POST /api/v4/admin/opensearch/recreate_indices

Синхронно пересоздаёт индекс(ы) OpenSearch и ставит фоновые задачи повторной индексации.

#### Тело запроса

| Поле | Тип | Обязательное | Допустимые значения |
|---|---|---|---|
| `schema_class` | string | Да | `recreate_all`, `Search::Opensearch::IndicesSchema::Code`, `Search::Opensearch::IndicesSchema::Wiki`, `Search::Opensearch::IndicesSchema::Note`, `Search::Opensearch::IndicesSchema::Milestone`, `Search::Opensearch::IndicesSchema::WorkItem`, `Search::Opensearch::IndicesSchema::MergeRequest` |

#### Ответы

Тексты полей `message` в примерах ниже возвращаются API на английском языке.

- `202 Accepted`

```json
{
  "message": "OpenSearch indices were reset; reindex jobs were enqueued."
}
```

- `400 Bad Request` (например, OpenSearch выключен или сервис вернул ошибку)

```json
{
  "message": "OpenSearch is disabled"
}
```

#### Пример запроса

```bash
curl --request POST \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --header "Content-Type: application/json" \
  --data '{"schema_class":"recreate_all"}' \
  --url "https://gitlab.example.com/api/v4/admin/opensearch/recreate_indices"
```

### GET /api/v4/admin/opensearch/indexing_queue_stats

Возвращает статистику Sidekiq-очереди индексации OpenSearch.

#### Ответ (200 OK)

```json
{
  "total": 42,
  "updated_at": "2026-07-01T12:34:56.789Z"
}
```

Поля:

- `total` — общее количество задач индексации в очереди;
- `updated_at` — timestamp ISO8601 с миллисекундами (или `null`).

#### Пример запроса

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/admin/opensearch/indexing_queue_stats"
```
