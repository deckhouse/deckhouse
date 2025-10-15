---
title: "Search"
menuTitle: Search
searchable: true
description: Guide to using search in Deckhouse Code
permalink: en/code/documentation/user/search.html
lang: en
weight: 45
---

Search in Deckhouse Code helps you quickly find the information you need across projects, groups, or the entire instance.
Search is available for various entities. The results are ranked by relevance and allow you to jump directly to the source object.

## Search scopes

|                      | Global | Group | Project |
|----------------------|---------|--------|----------|
| Code                 | If full-text search is enabled | If full-text search is enabled | ✓ |
| Comments             | If full-text search is enabled | If full-text search is enabled | ✓ |
| Commits              | If full-text search is enabled | If full-text search is enabled | ✓ |
| Issues               | ✓ | ✓ | ✓ |
| Merge requests       | ✓ | ✓ | ✓ |
| Milestones           | ✓ | ✓ | ✓ |
| Projects             | ✓ | ✓ | ✗ |
| Users                | ✓ | ✓ | ✓ |
| Wiki                 | ✗ | ✗ | ✓ |

{% alert level="info" %}
The administrator can restrict access to global search or disable certain scopes to improve performance.
{% endalert %}

## Full-text search

Full-text search is implemented through the database and the Git server.
To activate full-text search, the administrator must enable the corresponding feature flag:

1. Open the Rails console provided in the [Toolbox](/modules/code/stable/maintenance.html#toolbox) utility set by running the following command:

   ```shell
   gitlab-rails console -e production
   ```

1. Enable the full-text search flag by running the following command in the Rails console:

   ```ruby
   ::Feature.enable(:fe_full_text_search)
   ```

The platform supports **full-text search across repositories** while respecting **the access matrix**.  
This means that users will only see results for objects they have permission to read.
Access to file contents, issues, comments, and other entities is determined by the current access settings at the project, group, and instance levels.

{% alert level="info" %}
Thus, search in Deckhouse Code fully complies with security and access control requirements.
{% endalert %}

## Using search

General procedure for searching in Deckhouse Code:

1. Click "Search" in the top navigation bar.
2. Enter a search query.
3. Press **Enter** — results will appear on the search page.
4. Use filters to refine results by group, project, or object type.

![Search](/images/code/search_en.png)

### Limitations

The following query length limitations apply to search in Deckhouse Code:

- Minimum query length: 2 characters.
- Maximum query length: 64 words or 4096 characters.

### Global search

Allows searching across all projects and groups within the instance.

1. In the left menu, select "Search".
2. Enter a query and press **Enter**.

### Project search

1. Open the target project.
2. In the left menu, select "Search".
3. Enter a query and press **Enter**.

### Group search

1. Open the target group.
2. In the left menu, select "Search".
3. Enter a query and press **Enter**.

### Additional features

- Search in Deckhouse Code supports autocomplete for projects, groups, and users.
- If full-text search is enabled, autocomplete also works for commit messages, filenames, code, issues, and merge requests.
- When searching, you can quickly navigate to the required commit via its SHA.
