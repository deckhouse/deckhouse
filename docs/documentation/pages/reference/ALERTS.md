---
title: "The list of alerts"
permalink: en/reference/alerts.html
---

The page displays a list of all alerts of monitoring in the Deckhouse Kubernetes Platform.

{% alert %}
The list does not contain alerts of modules from a source.
{% endalert %}

Alerts are grouped by modules. To the right of the alert name, there are icons indicating the minimum DKP edition in which the alert is available, and the alert severity.

For each alert, a summary is provided, and if available, the detailed alert description can be viewed by expanding it.

## Alert severity

Alert descriptions contain the Severity (S) parameter, which indicates the level of criticality. Its value varies from `S1` to `S9` and can be interpreted as follows:

* `S1` — maximum level, critical failure/crash (immediate action required);
* `S2` — high level, close to maximum, possible accident (rapid response required);
* `S3` — medium level, potentially serious problem (verification required);
* `S4`-`S9` — low level. There is a problem, but overall performance is not impaired.

{% include deckhouse-alerts.liquid %}
