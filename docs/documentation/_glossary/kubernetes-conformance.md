---
title: Kubernetes conformance
description: Testing of Deckhouse Platform against the conformance test suite of the CNCF Certified Kubernetes Conformance Program. The tests are run for every supported Kubernetes minor version.
lang: en
---

The tests are run with Sonobuoy in the `certified-conformance` mode, and the JUnit reports are published in the reference. The results are valid only when the [`multitenancy-manager`](/modules/multitenancy-manager/) module is disabled.

For details, see [Kubernetes conformance test results](reference/kubernetes-conformance.html).
