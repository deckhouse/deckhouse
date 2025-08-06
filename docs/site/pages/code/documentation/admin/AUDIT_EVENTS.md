---
title: "Audit events"
menuTitle: Audit events
force_searchable: true
description: Audit events
permalink: en/code/documentation/admin/audit-events.html
lang: en
weight: 50
---

Security audit is a detailed analysis of your infrastructure aimed at identifying potential vulnerabilities and unsafe practices. Code helps simplify the audit process with audit events that allow you to track a wide range of actions happening in the system.

Examples of how audit events can be used:

- Tracking who changed a user's access level in a Code project and when it happened.
- Monitoring user account creation and deletion.

Audit events help you:

- Assess risks and strengthen security measures.
- Respond to incidents.

Common use cases:

- Detecting suspicious activity — for example, mass changes to employee email addresses or repository deletions.
- Tracking changes to environment variables in CI pipelines.
- Monitoring updates to project and group visibility levels.

All audit events are stored indefinitely, giving you constant access to the full event history.

To view audit events, switch to administrator mode and select "Audit events" in the side menu.

## Accessing audit events via API

Code also provides an API method to retrieve a list of audit events with filtering and sorting capabilities.

**Method:**  
`POST /api/v4/admin/audit_events/search`

**Description:**  
Returns a list of audit events. You can filter by date range, full-text search, and entity types.

{% alert level="warning" %}
The date range must be within a single calendar month. If `created_after` and `created_before` refer to different months, `created_before` will be automatically adjusted to the last day of the month specified by `created_after`.
{% endalert %}

### Request parameters

| Parameter         | Type    | Required | Description                                                                                       |
|-------------------|---------|----------|---------------------------------------------------------------------------------------------------|
| `created_after`   | string  | No       | Start date (inclusive) in ISO8601 format. Defaults to the beginning of the current month.          |
| `created_before`  | string  | No       | End date (inclusive) in ISO8601 format. Defaults to the end of the current month.                  |
| `q`               | string  | No       | Free-text search within the event message.                                                         |
| `sort`            | string  | No       | Sort by creation date. Allowed values: `created_asc`, `created_desc` (default).                   |
| `entity_types`    | array   | No       | List of entity types to filter: `User`, `Project`, `Group`, `Gitlab::Audit::InstanceScope`.       |

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
