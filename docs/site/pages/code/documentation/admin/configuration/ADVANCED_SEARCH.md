---
title: "Advanced search"
menuTitle: Advanced search
searchable: true
description: Configure and operate advanced search powered by OpenSearch in Deckhouse Code
permalink: en/code/documentation/admin/configuration/advanced-search.html
lang: en
weight: 55
relatedLinks:
  - title: "Advanced search — user guide"
    url: ../../user/search.html

# Once the advanced-search page is published in the code module documentation,

# add an "Advanced search — code module documentation" item here.
---

Administrator documentation for advanced search in Deckhouse Code: operating indexing after OpenSearch is connected.
<!-- Once the advanced-search page is published in the code module documentation, restore the
sentence linking to it: "For connection and setup, see the code module documentation".
Do not use markdown link syntax inside this comment: the related links collector does not
strip HTML comments and would pull the link into the "Additional resources" block. -->
For user instructions, see the [user guide](../../user/search.html).

## Operations

Manage indexing, monitoring, and troubleshooting.

### Instance-level management

Go to "Admin" → "Settings" → "Search".

This section is available after OpenSearch is connected.

#### Pause indexing

Enable "Pause OpenSearch indexing" to pause background indexing and reindexing jobs.
To enable indexing again, disable the "Pause OpenSearch indexing" flag.
After removing the pause, Sidekiq pause control resumes jobs automatically within a few minutes.

#### Branch indexing mode

The following branch indexing modes can be used in projects:

| Mode | Description |
|------|-------------|
| **Default branch only** | Only the default branch is indexed for all projects |
| **Allow per-project branch regex** | For projects, you can configure a regex to index additional branches |

{% alert level="warning" %}
After changing the branch indexing mode, manually enqueue a code reindex.
Search results may be incomplete until indexing completes.
{% endalert %}

#### OpenSearch index status

The page displays a table of indices:

| Index suffix | Search scope |
|--------------|--------------|
| `code` | Code (`blobs`) |
| `commits` | Commits |
| `wiki` | Wiki pages |
| `notes` | Comments |
| `milestones` | Milestones |
| `merge-requests` | Merge requests |
| `work-items` | Work items |

An OpenSearch index name consists of the shared instance prefix and the suffix from the table, for example `deckhouse-development-code`.

For each index, the table shows the OpenSearch index name, presence, document count, and index health.

#### Index operations

The following operations are available on the same page:

- **Reindex** — reindex a single index (removes existing documents and enqueues background jobs).
- **Reindex all indices** — reindex all indices.

{% alert level="warning" %}
The **Reindex** operation removes existing documents.
Search results may be incomplete until background indexing catches up.
{% endalert %}

The `commits` index is reindexed together with the `code` index: there is no separate operation for commits.
For the same reason, commits have no dedicated `schema_class` value.

You can also perform operations on indices using queries to the [OpenSearch API](#opensearch-api).

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

The indexing progress widget on "Admin" → "Settings" → "Search" shows the number of remaining full reindex jobs.
The same data is available through the [`indexing_queue_stats`](#get-apiv4adminopensearchindexing_queue_stats) endpoint.

#### Sidekiq queue

OpenSearch indexing jobs run in the dedicated `global-search-indexing` queue, not in the shared `default` queue.
Routing is configured with a Sidekiq rule: all workers with the `fe_global_search` category go to this queue.
A separate queue isolates indexing load from other Deckhouse Code background jobs.

#### Cron jobs

The following cron jobs are regularly performed to automate indexing:

| Schedule | Purpose |
|----------|---------|
| Every minute | Comment indexing — processes the accumulated notes change queue |
| Daily at 03:00 | Enqueues project indexing |

Cron jobs do not index directly: they start or resume the corresponding workers in the `global-search-indexing` queue.

#### Logs

OpenSearch indexing jobs are written to Sidekiq logs. Filter by queue name `global-search-indexing`.

### Troubleshooting

#### OpenSearch is unavailable

- Check the connection settings. For more information see the `code` module documentation.
- The "Admin" → "Settings" → "Search" page displays a connection failure message.
- Search returns an error.

#### Incomplete search results

- Wait for background indexing to complete (progress widget on "Admin" → "Settings" → "Search").
- Run reindexing at the project level or "Reindex" for the required index in "Admin" → "Settings" → "Search".

#### Indexing jobs are not appearing

If new jobs are not enqueued to the `global-search-indexing` queue:

1. Check whether "Pause OpenSearch indexing" is enabled in "Admin" → "Settings" → "Search". Clear the flag and wait for jobs to resume (the cron job runs every 5 minutes).
1. If the pause is cleared but jobs still do not appear, run Redis cleanup — stuck leases or Sidekiq duplicate keys are possible:

   ```shell
   bundle exec rails runner fe/scripts/clear_search_opensearch_worker_redis.rb
   ```

   The script clears the exclusive lease for `Search::RepositoryIndexerWorker`, concurrency limits, and dedup keys for the `global-search-indexing` queue.

## OpenSearch API

This section documents Deckhouse Code admin OpenSearch endpoints.
For user-facing search parameters, see ["Search API"](../../user/search.html#search-api).

### POST /api/v4/admin/opensearch/recreate_indices

Synchronously recreates OpenSearch index(es) and enqueues background reindex jobs. To rebuild (reindex) all indexes, run the query without a body. To rebuild a specific index, specify it in the query body.

Access rights: admin only (`authenticated_as_admin!`).

#### Request body

| Field | Type | Required | Allowed values |
|---|---|---|---|
| `schema_class` | string | Yes | `recreate_all`, `Search::Opensearch::IndicesSchema::Code`, `Search::Opensearch::IndicesSchema::Wiki`, `Search::Opensearch::IndicesSchema::Note`, `Search::Opensearch::IndicesSchema::Milestone`, `Search::Opensearch::IndicesSchema::WorkItem`, `Search::Opensearch::IndicesSchema::MergeRequest` |

#### Responses

- `202 Accepted`

```json
{
  "message": "OpenSearch indices were reset; reindex jobs were enqueued."
}
```

- `400 Bad Request` (for example, if OpenSearch is disabled or there is a service error)

```json
{
  "message": "OpenSearch is disabled"
}
```

#### Request example

This query rebuilds all indexes:

```bash
curl --request POST \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --header "Content-Type: application/json" \
  --data '{"schema_class":"recreate_all"}' \
  --url "https://gitlab.example.com/api/v4/admin/opensearch/recreate_indices"
```

### GET /api/v4/admin/opensearch/indexing_queue_stats

Returns Sidekiq queue stats for OpenSearch indexing.

Access rights: authenticated user with permission `read_admin_search_indexing_queue_stats` on `:global`.

#### Response (200 OK)

```json
{
  "total": 42,
  "updated_at": "2026-07-01T12:34:56.789Z"
}
```

Response fields:

- `total`: Total number of indexing jobs in the queue.
- `updated_at`: ISO8601 timestamp with milliseconds (or `null`).

#### Request example

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/admin/opensearch/indexing_queue_stats"
```
