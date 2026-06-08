---
title: IAM subsystem
permalink: en/architecture/iam/
search: iam, identity and access management
description: Architecture of the Identity and Access Management subsystem in Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

This subsection describes the architecture of the IAM subsystem (Identity and Access Management) of Deckhouse Kubernetes Platform (DKP).

The IAM subsystem provides the following features in DKP:

* [User authentication](authentication.html)
* Role-based access control (RBAC)
* [Multitenancy](multitenancy.html)
* Automatic assignment of annotations and labels to namespaces

The IAM subsystem includes the following modules that implement the features described above:

* [`user-authn`](/modules/user-authn/): User authentication.
* [`user-authz`](/modules/user-authz/): Role-based access control (RBAC).
* [`multitenancy-manager`](/modules/multitenancy-manager/): Multitenancy.
* [`namespace-configurator`](/modules/namespace-configurator/): Automatic assignment of annotations and labels to namespaces.
