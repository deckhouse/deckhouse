---
title: Security software settings for working with Deckhouse
permalink: en/security_software_setup.html
---

If security scanners (antivirus tools) scan nodes of the Kubernetes cluster, then it may be necessary to configure them to exclude false positives.

Deckhouse uses the following directories when working ([download in csv...](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}
