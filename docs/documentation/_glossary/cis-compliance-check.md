---
title: CIS compliance check
description: A check that Deckhouse Platform runs automatically in every cluster against the CIS Kubernetes Benchmark recommendations. The results are available as reports and on the Security / CIS Kubernetes Benchmark dashboard.
lang: en
---

Deckhouse Platform follows the CIS Kubernetes Benchmark both at the level of individual components and for the platform as a whole. Access to the reports, like access to [vulnerability scanning](glossary.html#vulnerability-scanning) results, is granted through the [role model](glossary.html#rbac) of the [`operator-trivy`](/modules/operator-trivy/) module.

For details, see [Configuration of container image vulnerability scanning](user/security/scanning.html).
