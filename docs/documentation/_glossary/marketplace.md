---
title: Marketplace
description: The package management system of Deckhouse Platform. Administrators connect package repositories, and users install applications from the packages in their namespaces.
lang: en
---

A cluster administrator connects a package repository with [PackageRepository](reference/api/cr.html#packagerepository), monitors the scanning operations in which Marketplace discovers the available packages, and grants users the permissions to view package versions and to create Application objects. An installed application is an [Application](reference/api/cr.html#application) object in the [namespace](glossary.html#namespace) of the user.

Marketplace is available in Deckhouse Platform starting from version 1.76.

For details, see [Marketplace](admin/configuration/marketplace/).
