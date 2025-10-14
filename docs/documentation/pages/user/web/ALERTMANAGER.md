---
title: Alert management
permalink: en/user/web/alertmanager.html
---

The alert management web UI can be used for handling alerts in a Deckhouse cluster.
It allows you to view detailed information about alerts, including the source, severity, and trigger time,
and to temporarily silence alert notifications when needed.

## Accessing the web UI

1. To open the alert management web UI,
   click the corresponding link in the side menu on the Grafana overview page.

   ![Opening the alert management web UI](../../images/alertmanager-email/alertmanager-webinterface.png)

1. If you are accessing the web UI for the first time, enter your user credentials.
   Once the authentication is complete, you will see the main web UI page
   displaying a summary of all alerts in the cluster.

   ![Alert management web UI](../../images/alertmanager-email/alertmanager-interface.png)

1. Alerts are grouped by category.
   To expand a group, click the `+` icon to the left of the group name.

   ![Alerts grouped by category](../../images/alertmanager-email/alertmanager-alerts.png)

1. Once expanded, the group will display the full list of associated alerts.

   ![A list of alerts in a group](../../images/alertmanager-email/alertmanager-alertsgroup.png)
