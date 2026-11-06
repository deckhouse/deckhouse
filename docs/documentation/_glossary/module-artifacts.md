---
title: Module artifacts
description: The three kinds of artifacts a module build pushes to the container registry, images of application containers, the module image and the release. Deckhouse Platform pulls and updates a module from them.
lang: en
---

Images of application containers are tagged with content-based tags and referenced from the templates. The module image is built by `werf.yaml` and tagged with the semantic version. The release is the artifact of a module version: it carries a semantic version tag and a tag of the [release channel](glossary.html#release-channel), and Deckhouse Platform decides whether to update the module in the cluster by its data. The registry with the artifacts serves as a [module source](glossary.html#module-source).

For details, see [Build and publish](architecture/module-development/build/).
