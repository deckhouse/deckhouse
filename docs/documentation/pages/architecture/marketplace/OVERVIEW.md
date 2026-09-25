---
title: Marketplace
permalink: en/architecture/marketplace/
description: "Architecture of the Deckhouse Platform Marketplace system. Package abstraction, delivery unit types, resource model, and subsystem overview."
---

Marketplace is the subsystem in Deckhouse Platform (DP) that manages the lifecycle of delivery units called **Packages**. A Package can be either an **Application** (a user workload deployed into a namespace) or a **Module** (a cluster capability extension). Currently, only Applications are supported; Module support is planned for a future version.

Marketplace is available starting from DP version 1.76.

## Sections

The [Concepts](concepts.html) section describes the Package abstraction, resource model, Application constraints, and the full scan-to-deploy lifecycle.

The [Application development](application-development.html) section describes how to create an Application package from scratch: bootstrapping, project structure, `package.yaml`, local rendering and linting, build, CI/CD, and artifact layout in the registry.

The [Application settings](settings.html) section describes the OpenAPI schemas of settings and values: validation, default values, and the extensions that control the settings form in the web interface.

The [Templates](templates.html) section describes the values available to Application templates, object naming, container images, Ingress, and application endpoints.

The [Nelm annotations](nelm-annotations.html) section covers the full set of Nelm annotations used in Application templates to control deployment order, resource lifecycle, tracking, and logging.

The [Hooks](hooks.html) section describes how to write Go hooks for Applications using the `ApplicationHookInput` type from module-sdk.

The [Lifecycle and debugging](lifecycle.html) section describes how DP installs, updates, and deletes an Application, and which tools help to debug a package.
