---
title: Package bundle
description: The main image of a package version in the container registry. It contains the package manifest, image digests, OpenAPI schemas, Helm templates, the hooks binary, documentation and the changelog.
lang: en
---

The bundle is published as `<REPOSITORY>/<PACKAGE_NAME>:<PACKAGE_VERSION>`, next to the container images of the application (referenced by digest), the version metadata image and the [release channel](glossary.html#release-channel) images. Deckhouse Platform downloads the bundle of the requested version as the first step of installing an [application instance](glossary.html#application-instance), then reads `package.yaml`, the schemas and `images_digests.json` and discovers the hooks.

For details, see [Application development](architecture/marketplace/application-development.html).
