---
title: "Audit events"
menuTitle: Audit events
force_searchable: true
description: Audit events
permalink: en/code/documentation/admin/audit-events.html
lang: en
weight: 50
---

A Audit events is a detailed analysis of your infrastructure aimed at identifying potential vulnerabilities and unsafe practices. Code helps simplify the audit process with audit events that allow you to track a wide range of actions happening in the system.

Examples of how audit events can be used:

- Tracking who changed a user’s access level in a Code project and groups.
- Creating and deleting users.

Audit events help:

- Assess risks and strengthen security measures.
- Respond to incidents.

For example:

- Detect suspicious activity, mass changes to employee emails, or repository deletions.
- Track changes to CI environment variables.
- Monitor visibility level changes for projects and groups.

All audit events are stored indefinitely. With no retention limit, you always have access to the full event history.

To access audit events, switch to Admin Mode and select **Audit Events** from the left sidebar.

## Accessing Audit Events via API

Code provides an API method to retrieve a list of audit events with filtering and sorting capabilities.

**Method:**  
`POST /api/v4/admin/audit_events/search`

**Description:**  
Returns a list of audit events. You can filter by date range, full-text search, and entity types.  
> ⚠️ The date range must be within a single calendar month. If `created_after` and `created_before` span different months, `created_before` will be automatically adjusted to the last day of the `created_after` month.

### Request parameters

| Parameter         | Type    | Required | Description                                                                                       |
|-------------------|---------|----------|---------------------------------------------------------------------------------------------------|
| `created_after`   | string  | no       | Start date (inclusive) in ISO8601 format. Defaults to the beginning of the current month          |
| `created_before`  | string  | no       | End date (inclusive) in ISO8601 format. Defaults to the end of the current month                  |
| `q`               | string  | no       | Free-text search within the event message                                                         |
| `sort`            | string  | no       | Sort by creation date. Allowed values: `created_asc`, `created_desc` (default)                   |
| `entity_types`    | array   | no       | List of entity types to filter: `User`, `Project`, `Group`, `Gitlab::Audit::InstanceScope`       |

**Example request:**

```bash
curl --request POST "https://example.com/api/v4/admin/audit_events/search" \
     --header "PRIVATE-TOKEN: <your_access_token>" \
     --header "Content-Type: application/json" \
     --data '{
       "created_after": "2025-08-01",
       "created_before": "2025-08-31",
       "q": "repository",
       "sort": "created_desc",
       "entity_types": ["Project"]
     }'
```