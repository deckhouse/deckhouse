---
title: Module settings conversion
description: A rule in the `openapi/conversions` directory of a module that converts module parameters from one version of the OpenAPI specification to the next, for example when a parameter is renamed or moved.
lang: en
---

Each conversion works between two consecutive versions and is stored in a `v<N>.yaml` file with its tests. The chain of conversions must cover all versions of the parameter specification without gaps, so that Deckhouse Platform can convert the settings of a [module](glossary.html#module) automatically when the version of its settings schema changes.

For details, see [Module structure](architecture/module-development/structure/).
