---
title: "The operator-trivy module"
---

The module allows you to run periodic vulnerability scans. The module uses the [Trivy](https://github.com/aquasecurity/trivy) project. 

Scanning is performed every 24 hours in namespaces with the `security-scanning.deckhouse.io/enabled ` label.
