---
title: Application instance
description: An application installed from a Marketplace package as an [Application](reference/api/cr.html#application) object in a namespace. The same package can be installed several times with separate names and settings, and the instance name is a part of the names of all application objects.
lang: en
---

Each instance is fully independent: two Redis instances, one for caching and one for sessions, can live in the same [namespace](glossary.html#namespace). Deckhouse Platform processes every instance in a separate queue. The objects of an instance are named `d8a-<INSTANCE_NAME>-<SUFFIX>`, so the instance name is limited to 24 characters, and every rendered object carries the `packages.deckhouse.io/instance` label.

For details, see [Installing and managing applications](user/marketplace/applications.html).
