---
title: "Cloud provider — VMware Cloud Director: configuration"
force_searchable: true
---

The module is automatically enabled for all cloud clusters deployed in VMware Cloud Director.

## List of required VMware Cloud Director services

The following VMware Cloud Director service must be available for Deckhouse Kubernetes Platform to operate:

| Service                   | API version      |
| :------------------------ | :---------:      |
| VMware Cloud Director API | `37.2` and later |

{% alert level="info" %}
For VMware Cloud Director API versions earlier than `37.2`, compatibility mode for legacy API versions is used.
{% endalert %}

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

{% include module-settings.liquid %}
