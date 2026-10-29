---
title: Protected parameters
description: Cluster parameters that are critical for cluster operation and cannot be changed in a running cluster by default. podSubnetCIDR, podSubnetNodeCIDRPrefix and serviceSubnetCIDR.
lang: en
---

The protected parameters are part of the [cluster configuration](glossary.html#cluster-configuration). An attempt to change them is blocked by the admission webhook with an error message. The procedure for changing them, with the [installer](glossary.html#installer) or manually, is described in the documentation.

For details, see [Deckhouse Platform configuration](admin/configuration/).
