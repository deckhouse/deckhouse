---
title: Grantable cluster-wide resource
description: A cluster-wide resource, for example a StorageClass, that the [`multitenancy-manager`](/modules/multitenancy-manager/) module makes available to a project through a grant. Application settings bound to such a resource get the project default and are validated against the resources available to the project.
lang: en
---

A `settings` field of type `string` is bound to a grantable resource with the `x-deckhouse-grantable-resource` extension, which names the resource available to the [project](glossary.html#project), for example `storageclasses`. When the user leaves the field empty, the resource name configured as the project default is injected into the values. A value that is not in the list of resources available to the project is rejected. The group, version and kind of the resource are defined by the grant.

For details, see [Application settings](architecture/marketplace/settings.html).
