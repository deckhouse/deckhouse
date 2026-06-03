---
title: "Расширенный поиск (OpenSearch)"
menuTitle: Расширенный поиск
searchable: true
description: Настройка и эксплуатация расширенного поиска на базе OpenSearch в Deckhouse Code
permalink: ru/code/documentation/admin/configuration/advanced-search.html
lang: ru
weight: 55
---

Документация для администратора по расширенному поиску в Deckhouse Code: эксплуатация индексации после подключения OpenSearch.
Подключение и настройка — в [документации модуля code](/modules/code/stable/advanced-search.html).
Инструкции для пользователей — в [руководстве пользователя](../../user/search.html).

## Эксплуатация

Управление индексацией, мониторинг и устранение неполадок.

### Управление на уровне инстанса

Перейдите в «Admin» → «Настройки» → «Поиск».

Раздел доступен после [подключения OpenSearch](/modules/code/stable/advanced-search.html).

#### Приостановка индексации

Установите флаг **Приостановить индексирование OpenSearch**, чтобы приостановить фоновые задачи индексации и переиндексации.
После снятия паузы Sidekiq pause control автоматически возобновит работу в течение нескольких минут.

#### Режим индексации веток

| Режим | Описание |
|-------|----------|
| **Только ветка по умолчанию** | Индексируется только ветка по умолчанию для всех проектов |
| **Разрешить регулярное выражение для веток на уровне проекта** | Проекты могут задать regex для индексации дополнительных веток |

{% alert level="warning" %}
После смены режима индексации веток выполните ручную переиндексацию кода.
Результаты поиска могут быть неполными до завершения индексации.
{% endalert %}

#### Статус индексов OpenSearch

На странице отображается таблица с четырьмя индексами:

| Индекс | Область поиска |
|--------|----------------|
| Code | Код (`blobs`) |
| Commit | Коммиты |
| Wiki | Wiki-страницы |
| Note | Комментарии |

Для каждого индекса показаны: имя в OpenSearch, наличие, количество документов, состояние индекса.

- **Переиндексировать** — переиндексация одного индекса (удаляет существующие документы и ставит фоновые задачи).
- **Переиндексировать все индексы** — переиндексация всех индексов.

{% alert level="warning" %}
Операция «Переиндексировать» удаляет существующие документы.
Результаты поиска могут быть неполными до завершения фоновой индексации.
{% endalert %}

При операции **Переиндексировать все индексы** фоновая переиндексация автоматически ставится для индексов Code, Wiki и Note.

### Admin API

#### Переиндексация индексов

```shell
curl --request POST \
  --header "PRIVATE-TOKEN: <admin_token>" \
  --data "schema_class=recreate_all" \
  "https://code.example.com/api/v4/admin/opensearch/recreate_indices"
```

Параметр `schema_class`:

| Значение | Описание |
|----------|----------|
| `Search::Opensearch::IndicesSchema::Code` | Индекс кода |
| `Search::Opensearch::IndicesSchema::Wiki` | Индекс wiki |
| `Search::Opensearch::IndicesSchema::Note` | Индекс комментариев |
| `recreate_all` | Все индексы |

#### Статистика очереди индексации

```shell
curl --header "PRIVATE-TOKEN: <admin_token>" \
  "https://code.example.com/api/v4/admin/opensearch/indexing_queue_stats"
```

Ответ содержит количество оставшихся задач и время последнего обновления.

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

Метрики `*_failed_total` увеличиваются при ошибках подключения к opensearch или ошибках авторизации.
Рост `*_failed_total` указывает на недоступность OpenSearch или неверные credentials.
Рост `*_duration_seconds` при стабильном `*_total` — на медленные ответы OpenSearch.

Для мониторинга индексации репозиториев ориентируйтесь на `search_repository_indexer_*`, для обращений к OpenSearch из Sidekiq — на `sidekiq_elasticsearch_*`.
Для мониторинга пользовательского поиска — на `http_elasticsearch_*`.

На странице «Admin» → «Настройки» → «Поиск» виджет прогресса индексации показывает число оставшихся задач переиндексации.
Те же данные доступны через [Admin API](#статистика-очереди-индексации).

#### Очередь Sidekiq

Задачи индексации OpenSearch выполняются в отдельной очереди `global-search-indexing`, а не в общей очереди `default`.
Маршрутизация настраивается правилом Sidekiq: все воркеры с категорией `fe_global_search` попадают в эту очередь.
Отдельная очередь изолирует нагрузку индексации от остальных фоновых задач Deckhouse Code.

#### Cron-задачи

| Расписание | Назначение |
|------------|------------|
| Каждую минуту | Индексация комментариев — обрабатывает накопившуюся очередь изменений notes |
| Ежедневно в 03:00 | Ставит индексацию проектов |

Cron-задачи не выполняют индексацию напрямую: они запускают или возобновляют соответствующие воркеры в очереди `global-search-indexing`.

#### Логи

Задачи индексации OpenSearch пишутся в логи Sidekiq. Для фильтрации используйте имя очереди `global-search-indexing`.

### Устранение неполадок

#### OpenSearch недоступен

- Проверьте настройки подключения — см. [документацию модуля code](/modules/code/stable/advanced-search.html).
- На странице «Admin» → «Настройки» → «Поиск» появится сообщение о невозможности подключения.
- Поиск вернёт  ошибку.

#### Неполные результаты поиска

- Дождитесь завершения фоновой индексации (виджет прогресса на странице «Admin» → «Настройки» → «Поиск»).
- Запустите переиндексацию на уровне проекта или **Переиндексировать** для нужного индекса в «Admin» → «Настройки» → «Поиск».

#### Задачи индексации не появляются

Если новые задачи не ставятся в очередь `global-search-indexing`:

1. Проверьте, не включена ли пауза **Приостановить индексирование OpenSearch** в «Admin» → «Настройки» → «Поиск». Снимите флаг и дождитесь возобновления (cron-задача выполняется каждые 5 минут).
1. Если пауза снята, а задачи по-прежнему не появляются, выполните очистку Redis — возможны зависшие lease или duplicate-ключи Sidekiq:

   ```shell
   bundle exec rails runner fe/scripts/clear_search_opensearch_worker_redis.rb
   ```

   Скрипт снимает exclusive lease для `Search::RepositoryIndexerWorker`, concurrency limit и dedup-ключи очереди `global-search-indexing`.

## Связанные темы

- [Расширенный поиск — руководство пользователя](../../user/search.html)
- [Расширенный поиск — документация модуля code](/modules/code/stable/advanced-search.html)
