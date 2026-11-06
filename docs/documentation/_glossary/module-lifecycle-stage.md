---
title: Module lifecycle stage
description: The maturity stage of a module or a Marketplace package, Experimental, Preview, General Availability or Deprecated. It is declared in module.yaml or package.yaml and, together with the release channel, defines how stable a version is.
lang: en
---

- `Experimental`: the functionality may change significantly and compatibility with future versions is not guaranteed; such modules cannot be enabled unless the `allowExperimentalModules` parameter of the [`deckhouse`](/modules/deckhouse/) module is set to `true`.
- `Preview`: a preliminary version; the basic features are preserved, compatibility is ensured but may require migration.
- `General Availability`: ready for production environments.
- `Deprecated`: development and support are discontinued.

The stage of a [Marketplace package](glossary.html#marketplace-package) is published in the package metadata when Deckhouse Platform scans the repository.

For details, see [Versioning modules](architecture/module-development/versioning/).
