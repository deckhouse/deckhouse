---
title: "The operator-trivy module"
description: operator-trivy is a Deckhouse module for periodic scanning for vulnerabilities in a Kubernetes cluster.
---

The module allows you to run periodic vulnerability scans. The module uses the [Trivy](https://github.com/aquasecurity/trivy) project. 

Scanning is performed every 24 hours in namespaces with the `security-scanning.deckhouse.io/enabled=""` label.

If no namespaces with the label `security-scanning.deckhouse.io/enabled=""` are found, the `default` namespace is scanned. Once any namespace with the label `security-scanning.deckhouse.io/enabled=""` is found, scanning for the `default` namespace will be disabled and in order for it to be scanned, the label `kubectl label namespace default security-scanning.deckhouse.io/enabled=""` will also need to be set.
