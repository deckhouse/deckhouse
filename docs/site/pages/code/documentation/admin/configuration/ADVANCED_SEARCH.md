---
title: "Advanced search (OpenSearch)"
menuTitle: Advanced search
searchable: true
description: Configure and operate advanced search powered by OpenSearch in Deckhouse Code
permalink: en/code/documentation/admin/configuration/advanced-search.html
lang: en
weight: 55
---

Advanced search in Deckhouse Code uses OpenSearch for full-text search.
After enabling it, users get faster and more precise search with query syntax support.

To enable advanced search, you must:

1. [Deploy OpenSearch](#deploy-opensearch).
1. [Configure Deckhouse Code](#configure-gitlabyml).
1. [Enable search for users](#enable-search-for-users).

{% alert level="info" %}
Advanced search stores data from all projects in shared OpenSearch indices.
Users see only objects they have permission to access in search results.
{% endalert %}

## Enablement architecture

Advanced search is controlled at two levels:

| Level | Configuration | Default | Effect |
|-------|---------------|---------|--------|
| Instance config | `gitlab.yml` → `fe.search.advanced_search_enabled` | `false` | Background indexing, admin UI, reindex API |
| Runtime toggle | `fe_application_settings.use_advanced_search` | `false` | Search in UI and API |

When `advanced_search_enabled` is on and `use_advanced_search` is off, indexing continues, but the UI and API do not use OpenSearch for search.

## Deploy OpenSearch

OpenSearch is not included with Deckhouse Code and must be deployed separately.
Run OpenSearch on a dedicated server or in a separate namespace to avoid competing for resources with the application.

Resource requirements depend on the volume of indexed data: number of projects, repository size, and comment count.
Plan storage and memory capacity for production environments in advance.

You also need the `gitlab-elasticsearch-indexer` binary — an external repository indexer.
Set its path in `gitlab.yml` (`indexer_path`).

## Configure gitlab.yml

Add the `fe.search` section to the instance configuration:

```yaml
fe:
  search:
    advanced_search_enabled: true
    repository_indexer_max_concurrency: 30
    elasticsearch_indexed_file_size_limit_kb: 1024
    indexer_path: /path/to/gitlab-elasticsearch-indexer
    opensearch:
      url: http://opensearch.example.com:9200
      max_bulk_size_bytes: 10485760
      max_bulk_concurrency: 10
      username: admin
      password: secret
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `advanced_search_enabled` | `false` | Master switch for advanced search |
| `repository_indexer_max_concurrency` | `30` | Maximum concurrency of the external indexer |
| `elasticsearch_indexed_file_size_limit_kb` | `1024` | Maximum indexed file size (KB) |
| `indexer_path` | — | Path to the `gitlab-elasticsearch-indexer` binary (required when search is enabled) |
| `opensearch.url` | `http://localhost:9200/` | OpenSearch URL |
| `opensearch.max_bulk_size_bytes` | `10485760` (10 MB) | Maximum bulk request size |
| `opensearch.max_bulk_concurrency` | `10` | Bulk indexing concurrency |
| `opensearch.username` / `password` | — | Basic auth (both must be set together) |

Restart the application after changing the configuration.

## Enable search for users

1. Set `advanced_search_enabled: true` in `gitlab.yml` and restart the instance.
1. Enable `use_advanced_search` through the API (this setting is read-only in the admin UI):

   ```shell
   curl --request PUT \
     --header "PRIVATE-TOKEN: <admin_token>" \
     --data "fe_application_settings_attributes[use_advanced_search]=true" \
     "https://code.example.com/api/v4/application/settings"
   ```

Background indexing starts after step 1.
Search in the UI becomes available after step 2.

## Admin Area management

Go to **Admin Area → Settings → Search** (`/admin/application_settings/search`).

This section is available only when `advanced_search_enabled: true`.

### Pause indexing

Enable **Pause OpenSearch indexing** to pause background indexing and reindexing jobs.
After removing the pause, Sidekiq pause control resumes jobs automatically within a few minutes.

### Branch indexing mode

| Mode | Description |
|------|-------------|
| **Default branch only** | Indexes code from the default branch only for all projects |
| **Allow per-project branch regex** | Projects can configure a regex to index additional branches |

{% alert level="warning" %}
After changing the branch indexing mode, manually enqueue a code reindex.
Search results may be incomplete until indexing completes.
{% endalert %}

### OpenSearch index status

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
Reindex operations remove existing documents.
Search results may be incomplete until background indexing catches up.
{% endalert %}

The **Reindex all indices** operation automatically enqueues background reindexing for Code, Wiki, and Note indices.
Reindex the Commit index separately using the **Reindex** button in the table.

### Indexing progress

The **Indexing progress** widget shows the number of remaining forced reindex jobs in the Sidekiq queue.

## Project settings

A project maintainer can go to **Settings → Search** (`/-/namespace/project/-/settings/search`).

### Branch regex

When **Allow per-project branch regex** is enabled at the instance level, the maintainer can specify a regex for additional branches.
The default branch is always indexed.

Example regex: `(feature|hotfix)/.*`

{% alert level="warning" %}
Changing the regex triggers a full project reindex.
{% endalert %}

### Reindex code and wiki

- **Reindex code** — full reindex of the repository code.
- **Reindex wiki** — full reindex of the wiki (if a wiki repository exists).

The **Index up to date** badge shows whether indexing is complete for the current repository state.

## Group settings

A group owner can go to **Settings → Search** (`/groups/:id/-/settings/search`).

Group wiki reindexing is available: index status and the **Reindex wiki** button.

## Admin API

### Recreate indices

```shell
curl --request POST \
  --header "PRIVATE-TOKEN: <admin_token>" \
  --data "schema_class=recreate_all" \
  "https://code.example.com/api/v4/admin/opensearch/recreate_indices"
```

The `schema_class` parameter is the fully qualified index schema class name or `recreate_all` for all indices.

### Indexing queue stats

```shell
curl --header "PRIVATE-TOKEN: <admin_token>" \
  "https://code.example.com/api/v4/admin/opensearch/indexing_queue_stats"
```

The response contains the number of remaining jobs and the last update time.

## Monitoring

| Mechanism | Description |
|-----------|-------------|
| `elasticsearch_failed_request_count` metric | Counter for failed OpenSearch requests |
| GraphQL `searchIndexingQueueStats` | Reindex job queue (used by the admin page widget) |
| Sidekiq queue `global-search-indexing` | Background indexing job queue |

Background jobs:

| Job | Schedule | Purpose |
|-----|----------|---------|
| `Search::NotesIndexerWorker` | Every minute | Comment indexing |
| `Search::RepositoryIndexConsistencyCronWorker` | Daily at 03:00 | Repository index consistency check |
| `PauseControl::ResumeWorker` | Every 5 minutes | Resume paused indexing |

## Troubleshooting

### OpenSearch is unavailable

- Check `opensearch.url` and credentials in `gitlab.yml`.
- The **Admin → Settings → Search** page displays a connection failure message.
- Search for code, commits, wiki, and comments returns empty results or an error.

### Incomplete search results

- Wait for background indexing to complete (check **Indexing progress**).
- Run **Reindex code** at the project level or **Reindex** for the required index in Admin Area.
- Verify that `use_advanced_search` is enabled through the API.

### Stuck indexing jobs

When indexer leases or Sidekiq duplicate keys for the `global-search-indexing` queue are stuck, run the Redis cleanup script:

```shell
bundle exec rails runner fe/scripts/clear_search_opensearch_worker_redis.rb
```

The script clears the exclusive lease for `Search::RepositoryIndexerWorker`, concurrency limits, and dedup keys for the `global-search-indexing` queue.

## Related topics

- [Advanced search — user guide](../../user/search.html)
