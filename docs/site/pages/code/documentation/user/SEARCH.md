---
title: "Search"
menuTitle: Search
searchable: true
description: Guide to using search in Deckhouse Code
permalink: en/code/documentation/user/search.html
lang: en
weight: 45
---

## Overview

Search in Deckhouse Code helps you quickly find the information you need across projects, groups, or the entire instance.  
Search is available for various entities. The results are ranked by relevance and allow you to jump directly to the source object.

---

## Search Scopes

|                      | Global | Group | Project |
|----------------------|---------|--------|----------|
| Code                 | If full-text search is enabled | If full-text search is enabled | ✓ |
| Comments             | If full-text search is enabled | If full-text search is enabled | ✓ |
| Commits              | If full-text search is enabled | If full-text search is enabled | ✓ |
| Issues               | ✓ | ✓ | ✓ |
| Merge Requests       | ✓ | ✓ | ✓ |
| Milestones           | ✓ | ✓ | ✓ |
| Projects             | ✓ | ✓ | ✗ |
| Users                | ✓ | ✓ | ✓ |
| Wiki                 | ✗ | ✗ | ✓ |

> The administrator can restrict access to global search or disable certain areas to improve performance.

---

## Full-Text Search

Full-text search is implemented through the database and the Git server.  
To activate it, the administrator must enable the feature flag in the Rails console [Toolbox](/modules/code/stable/maintenance.html#toolbox).  

Open console and run follow comands:

```shell
gitlab-rails console -e production
```

```ruby
::Feature.enable(:fe_full_text_search)
```

The system supports **full-text search across repositories** while respecting **the access matrix**.  
This means that users will only see results for objects they have permission to read.  
Access to file contents, issues, comments, and other entities is determined by the current access settings at the project, group, and instance levels.

> Thus, search in Deckhouse Code fully complies with security and access control requirements.

---

## Using Search

1. Click **Search** in the top navigation bar.  
2. Enter a search query (minimum of two characters).  
3. Press **Enter** — results will appear on the search page.  
4. Use filters to refine results by group, project, or object type.

---

## Project Search

1. Open the desired project.  
2. In the left menu, select **Search**.  
3. Enter a query and press **Enter**.  

---

## Global Search

Allows searching across all projects and groups within the instance.

1. In the left menu, select **Search**.  
2. Enter a query and press **Enter**.  

---

## Group Search

1. Open the desired group.  
2. In the left menu, select **Search**.  
3. Enter a query and press **Enter**.  

---

## Limitations

- Minimum query length — 2 characters.  
- Maximum — 64 words or 4096 characters.  

---

## Additional Features

- Autocomplete for projects, groups, and users.  
- If full-text search is enabled, autocomplete also works for commit messages, filenames, code, issues, and merge requests.  
- Quick navigation to a commit by SHA.  

---
