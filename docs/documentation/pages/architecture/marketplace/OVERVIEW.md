---
title: Marketplace
permalink: en/architecture/marketplace/
description: "Architecture of the Deckhouse Platform Marketplace system. Package abstraction, delivery unit types, resource model, and subsystem overview."
---

[Marketplace](../../glossary.html#marketplace) is a [package](../../glossary.html#marketplace-package) management system in Deckhouse Platform (DP). There are two types of packages:

- **Application**: A user workload deployed into a namespace.
- **Module**: A cluster capability extension.

Currently, only Application packages are supported. Module support is planned for a future DP version.

{% alert level="info" %}
Marketplace is available in DP starting from version 1.76.
{% endalert %}

To learn about connecting package repositories and managing repository scanning, refer to ["Administration"](../../admin/configuration/marketplace/).

To learn about installing and removing applications from Marketplace, as well as troubleshooting, refer to ["Usage"](../../user/marketplace/).

## Sections

The Marketplace documentation is divided into the following sections:

- The ["Concepts"](concepts.html) section describes package types, resource model, Application constraints, and application installation steps.

- The ["Application development"](application-development.html) section describes how to create an Application package from scratch: bootstrapping, project structure, `package.yaml`, local rendering and linting, build, CI/CD, and package artifacts in the container registry.

- The ["Application settings"](settings.html) section describes the OpenAPI schemas of settings and values: validation, default values, and the extensions that control the settings form in the web interface.

- The ["Templates"](templates.html) section describes the values available to Application templates, object naming, container images, Ingress, and [application endpoints](../../glossary.html#application-endpoints).

- The ["Nelm annotations"](nelm-annotations.html) section covers the full set of Nelm annotations used in Application templates to control deployment order, resource lifecycle, tracking, and logging.

- The ["Hooks"](hooks.html) section describes how to write Go [hooks](../../glossary.html#module-hook) for Applications using the `ApplicationHookInput` type from module-sdk.

- The ["Lifecycle and debugging"](lifecycle.html) section describes how DP installs, updates, and deletes an Application, and which tools help to debug a package.
