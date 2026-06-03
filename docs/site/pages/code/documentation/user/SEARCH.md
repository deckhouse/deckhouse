---
title: "Advanced search"
menuTitle: Advanced search
searchable: true
description: Guide to using advanced search in Deckhouse Code
permalink: en/code/documentation/user/search.html
lang: en
weight: 45
---

Search in Deckhouse Code helps you quickly find the information you need across projects, groups, or the entire instance.
Results are ranked by relevance and allow you to jump directly to the source object.

Advanced search powered by OpenSearch allows you to:

- find code patterns across all accessible projects;
- track usage of deprecated functions and libraries;
- search comments on issues and merge requests;
- find commits by message or SHA;
- search wiki page content.

## Use advanced search

Prerequisites:

- An administrator must [enable advanced search (OpenSearch)](../admin/configuration/advanced-search.html).

To search:

1. In the top bar, select **Search**.
1. Enter your search term.
1. Press **Enter**.

You can also use advanced search in a project or group context.

## Search scopes

Scopes describe the type of data you are searching.

### Basic search

The following scopes are available for basic search (without OpenSearch):

| Scope | Global | Group | Project |
|-------|:------:|:-----:|:-------:|
| Code | ✗ | ✗ | ✓ |
| Comments | ✗ | ✗ | ✓ |
| Commits | ✗ | ✗ | ✓ |
| Issues | ✓ | ✓ | ✓ |
| Merge requests | ✓ | ✓ | ✓ |
| Milestones | ✓ | ✓ | ✓ |
| Projects | ✓ | ✓ | ✗ |
| Users | ✓ | ✓ | ✓ |
| Wiki | ✗ | ✗ | ✓ |

### Advanced search

When OpenSearch is enabled, the following scopes are available:

| Scope | Global | Group | Project |
|-------|:------:|:-----:|:-------:|
| Code | ✓ | ✓ | ✓ |
| Comments | ✓ | ✓ | ✓ |
| Commits | ✓ | ✓ | ✓ |
| Issues | ✓ | ✓ | ✓ |
| Merge requests | ✓ | ✓ | ✓ |
| Milestones | ✓ | ✓ | ✓ |
| Projects | ✓ | ✓ | ✗ |
| Users | ✓ | ✓ | ✓ |
| Wiki | ✓ | ✓ | ✓ |

{% alert level="info" %}
The administrator can restrict access to global search or disable certain scopes to improve performance.
{% endalert %}

When OpenSearch is enabled, search for code, commits, wiki, and comments runs through OpenSearch and respects the **access matrix**.
Users see only objects they have permission to read.
Search for issues, merge requests, and other entities runs through the database.

## Using search

General procedure for searching in Deckhouse Code:

1. Click **Search** in the top navigation bar.
1. Enter a search query.
1. Press **Enter** — results appear on the search page.
1. Use filters to refine results by group, project, or object type.

![Search](/images/code/search_en.png)

### Global search

Allows searching across all projects and groups within the instance.

1. In the left menu, select **Search**.
1. Enter a query and press **Enter**.

### Project search

1. Open the target project.
1. In the left menu, select **Search**.
1. Enter a query and press **Enter**.

### Group search

1. Open the target group.
1. In the left menu, select **Search**.
1. Enter a query and press **Enter**.

### Additional features

- Search supports autocomplete for projects, groups, and users.
- When advanced search is enabled, autocomplete also works for commit messages, filenames, code, issues, and merge requests.
- When searching, you can quickly navigate to a commit by its SHA.

## Syntax

Advanced search uses [`simple_query_string`](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-simple-query-string-query.html),
which supports both exact and fuzzy queries.

| Syntax | Description | Example |
|--------|-------------|---------|
| `"` | Exact search | `"gem sidekiq"` |
| `~` | Fuzzy search | `J~ Doe` |
| `\|` | Or | `display \| banner` |
| `+` | And | `display +banner` |
| `-` | Exclude | `display -banner` |
| `*` | Partial match | `bug error 50*` |
| `\` | Escape | `\*md` |
| `#` | Issue ID (in comments) | `#23456` |
| `!` | Merge request ID (in comments) | `!23456` |

### Code search

| Syntax | Description | Example |
|--------|-------------|---------|
| `filename:` | Filename | `filename:*spec.rb` |
| `path:` | Repository location (full or partial matches) | `path:spec/workers/` |
| `extension:` | File extension without `.` | `extension:js` |
| `blob:` | Git object ID | `blob:998707*` |

The code search UI also provides a language filter.

### Examples

| Query | Description |
|-------|-------------|
| `rails -filename:gemfile.lock` | Returns `rails` in all files except `gemfile.lock`. |
| `RSpec.describe Resolvers -*builder` | Returns `RSpec.describe Resolvers` excluding matches starting with `builder`. |
| `bug \| (display +banner)` | Returns `bug` or both `display` and `banner`. |
| `helper -extension:yml -extension:js` | Returns `helper` in all files except `.yml` and `.js` files. |
| `helper path:lib/git` | Returns `helper` in files with a `lib/git*` path (for example, `spec/lib/gitlab`). |

## Known limitations

- Only files up to 1024 KB (1 MB) are indexed.
  An administrator can change this limit in the instance configuration.
- By default, only the default branch is indexed.
  An administrator can allow indexing of additional branches through a per-project regex.
- Minimum query length is 2 characters.
- Maximum query length is 64 words or 4096 characters.
- After pushing changes to a repository, search results may update with a delay while background indexing completes.
- When OpenSearch is unavailable, search for code, commits, wiki, and comments may return empty results or an error message.

## Related topics

- [Advanced search (OpenSearch) — administrator configuration](../admin/configuration/advanced-search.html)
