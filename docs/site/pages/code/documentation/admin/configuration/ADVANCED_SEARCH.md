---
title: "Advanced search (OpenSearch)"
menuTitle: Advanced search
searchable: true
description: Configure and operate advanced search powered by OpenSearch in Deckhouse Code
permalink: en/code/documentation/admin/configuration/advanced-search.html
lang: en
weight: 55
---

Administrator documentation for advanced search in Deckhouse Code: operating indexing after OpenSearch is connected.
For connection and setup, see the [code module documentation](/modules/code/stable/advanced-search.html).
For user instructions, see the [user guide](../../user/search.html).

## Operations

Manage indexing, monitoring, and troubleshooting.

### Instance-level management

Go to **Admin** → **Settings** → **Search**.

This section is available after [OpenSearch is connected](/modules/code/stable/advanced-search.html).

#### Pause indexing

Enable **Pause OpenSearch indexing** to pause background indexing and reindexing jobs.
After removing the pause, Sidekiq pause control resumes jobs automatically within a few minutes.

#### Branch indexing mode

| Mode | Description |
|------|-------------|
| **Default branch only** | Only the default branch is indexed for all projects |
| **Allow per-project branch regex** | Projects can configure a regex to index additional branches |

{% alert level="warning" %}
After changing the branch indexing mode, manually enqueue a code reindex.
Search results may be incomplete until indexing completes.
{% endalert %}

#### OpenSearch index status

The page displays a table with four indices:

| Index | Search scope |
|-------|--------------|
| Code | Code (`blobs`) |
| Commit | Commits |
| Wiki | Wiki pages |
| Note | Comments |

For each index, the table shows the OpenSearch index name, presence, document count, and index health.

- **Reindex** — reindex a single index (removes existing documents and enqueues background jobs).
- **Reindex all indices** — reindex all indices.

{% alert level="warning" %}
The **Reindex** operation removes existing documents.
Search results may be incomplete until background indexing catches up.
{% endalert %}

The **Reindex all indices** operation automatically enqueues background reindexing for Code, Wiki, and Note indices.

### Admin API

#### Recreate indices

```shell
curl --request POST \
  --header "PRIVATE-TOKEN: <admin_token>" \
  --data "schema_class=recreate_all" \
  "https://code.example.com/api/v4/admin/opensearch/recreate_indices"
```

The `schema_class` parameter:

| Value | Description |
|-------|-------------|
| `Search::Opensearch::IndicesSchema::Code` | Code index |
| `Search::Opensearch::IndicesSchema::Wiki` | Wiki index |
| `Search::Opensearch::IndicesSchema::Note` | Comments index |
| `recreate_all` | All indices |

#### Indexing queue stats

```shell
curl --header "PRIVATE-TOKEN: <admin_token>" \
  "https://code.example.com/api/v4/admin/opensearch/indexing_queue_stats"
```

The response contains the number of remaining jobs and the last update time.

### Monitoring

#### Metrics

OpenSearch requests are tracked in Prometheus separately for HTTP requests (search in the UI and API) and for Sidekiq background jobs (indexing).
Metric names contain `elasticsearch` — a historical GitLab name; the metrics refer to OpenSearch.

**HTTP requests** (user search):

| Metric | Description |
|--------|-------------|
| `http_elasticsearch_requests_total` | Number of OpenSearch requests per HTTP request |
| `http_elasticsearch_requests_duration_seconds` | Total OpenSearch request time per HTTP request |
| `http_elasticsearch_requests_failed_total` | Failed OpenSearch requests per HTTP request (connection or authorization errors) — **added in Deckhouse Code** |

**Sidekiq** (background indexing):

| Metric | Description |
|--------|-------------|
| `sidekiq_elasticsearch_requests_total` | Number of OpenSearch requests per Sidekiq job |
| `sidekiq_elasticsearch_requests_duration_seconds` | Total OpenSearch request time per Sidekiq job |
| `sidekiq_elasticsearch_requests_failed_total` | Failed OpenSearch requests per Sidekiq job (connection or authorization errors) — **added in Deckhouse Code** |

**Repository indexer** (`Search::RepositoryIndexerWorker` — code, commits, wiki):

| Metric | Labels | Description |
|--------|--------|-------------|
| `search_repository_indexer_starts_total` | `indexer_class` | Indexing runs that started after the `advanced_search_enabled` check |
| `search_repository_indexer_runs_total` | `outcome`, `indexer_class` | Completed runs after obtaining an exclusive lock (`outcome`: `success` or `error`) |
| `search_repository_indexer_duration_seconds` | `outcome`, `indexer_class` | Duration of the indexing phase under exclusive lock |
| `search_repository_indexer_lock_contention_total` | — | Times the lock was not obtained and the job was rescheduled |

The `indexer_class` label indicates the indexing type:

| `indexer_class` | When used |
|-----------------|-----------|
| `Search::RepositoryIndexer::IncrementalIndexService` | Incremental indexing after repository changes |
| `Search::RepositoryIndexer::FullIndexService` | Full reindex (`force: true`) |
| `Search::RepositoryIndexer::MaintainsService` | Index update triggered by an event |
| `Search::RepositoryIndexer::DeleteService` | Remove documents from the index (empty or deleted repository/wiki) |

An increase in `search_repository_indexer_lock_contention_total` indicates lock contention between jobs for the same project.
An increase in `search_repository_indexer_runs_total{outcome="error"}` indicates go-indexer or indexing service errors; see Sidekiq logs for details.

The `*_failed_total` metrics increase on OpenSearch connection or authorization errors.
An increase in `*_failed_total` indicates OpenSearch is unavailable or credentials are invalid.
An increase in `*_duration_seconds` with a stable `*_total` indicates slow OpenSearch responses.

For repository indexing, use `search_repository_indexer_*`; for OpenSearch requests from Sidekiq, use `sidekiq_elasticsearch_*`.
For user search, use `http_elasticsearch_*`.

The indexing progress widget on **Admin** → **Settings** → **Search** shows the number of remaining full reindex jobs.
The same data is available through the [Admin API](#indexing-queue-stats).

#### Sidekiq queue

OpenSearch indexing jobs run in the dedicated `global-search-indexing` queue, not in the shared `default` queue.
Routing is configured with a Sidekiq rule: all workers with the `fe_global_search` category go to this queue.
A separate queue isolates indexing load from other Deckhouse Code background jobs.

#### Cron jobs

| Schedule | Purpose |
|----------|---------|
| Every minute | Comment indexing — processes the accumulated notes change queue |
| Daily at 03:00 | Enqueues project indexing |

Cron jobs do not index directly: they start or resume the corresponding workers in the `global-search-indexing` queue.

#### Logs

OpenSearch indexing jobs are written to Sidekiq logs. Filter by queue name `global-search-indexing`.

### Troubleshooting

#### OpenSearch is unavailable

- Check the connection settings — see the [code module documentation](/modules/code/stable/advanced-search.html).
- The **Admin** → **Settings** → **Search** page displays a connection failure message.
- Search returns an error.

#### Incomplete search results

- Wait for background indexing to complete (progress widget on **Admin** → **Settings** → **Search**).
- Run reindexing at the project level or **Reindex** for the required index in **Admin** → **Settings** → **Search**.

#### Indexing jobs are not appearing

If new jobs are not enqueued to the `global-search-indexing` queue:

1. Check whether **Pause OpenSearch indexing** is enabled in **Admin** → **Settings** → **Search**. Clear the flag and wait for jobs to resume (the cron job runs every 5 minutes).
1. If the pause is cleared but jobs still do not appear, run Redis cleanup — stuck leases or Sidekiq duplicate keys are possible:

   ```shell
   bundle exec rails runner fe/scripts/clear_search_opensearch_worker_redis.rb
   ```

   The script clears the exclusive lease for `Search::RepositoryIndexerWorker`, concurrency limits, and dedup keys for the `global-search-indexing` queue.

## Related topics

- [Advanced search — user guide](../../user/search.html)
- [Advanced search — code module documentation](/modules/code/stable/advanced-search.html)
