---
title: "Module documentation"
description: "A web interface with documentation for the Deckhouse Platform."
webIfaces:
- name: documentation
---

The `documentation` module creates a web interface for access to the documentation of the Deckhouse Platform version running in the cluster and to the documentation of the available modules.

To get the web interface address, in the [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) template of the Deckhouse Platform global configuration parameter, replace the `%s` key with `documentation`. For example, if `publicDomainTemplate` is set as `%s-kube.company.my`, then the documentation web interface will be available at `documentation-kube.company.my`.

## Main features

The module provides the following features:

- The documentation of the Deckhouse Platform version running in the cluster, without access to the Internet
- The documentation of the modules of every ModuleSource of the cluster, enabled or not
- Search throughout the platform documentation and the modules
- The pages in Markdown, an `llms.txt` index and a `corpus.json` file for AI tools
- Authentication through the `user-authn` module, an external authentication service or basic authentication with a generated password: see the [usage page](usage.html#authentication)
- Keeping the prepared versions in the system S3 storage or [in an S3 storage of your own](usage.html#s3-storage-of-your-own), and a [high availability mode](usage.html#high-availability-mode) with two replicas

## How the module works

The documentation of the Deckhouse Platform version the module is part of is built into the module.

The documentation of a specific module version is prepared on demand and cached:

1. A user opens a page of a module version that is not prepared yet.
1. The module downloads the documentation from the image of the version in the container registry and builds its pages.
1. The prepared version is kept on the volume of the documentation pod. When [keeping versions in S3](configuration.html#parameters-modulestorage-s3-type) is on, the version is kept in S3 too, and after a restart of the pod the module opens it without preparing it again.
1. One version of every module is kept always. For an enabled module it is the installed version. For the other modules it is the version of the most stable channel: stable when it exists. The search index and the AI tool files (`llms.txt`, `corpus.json`) of all modules are built from these versions.

When a module has a ModuleRelease in the `Pending` phase, the module prepares the documentation of that version in advance. The module reads the list of versions from the container registry every 30 minutes. A version published later shows up in the list within 30 minutes.

## Limitations

Each documentation pod keeps the prepared versions on a 4 GiB volume, in two parts of 2 GiB:

| Part | Content | Room | Removal |
|---|---|---|---|
| The versions users open | A version takes about 1.3 MB | About 1500 versions | Above 2 GiB the version opened longest ago goes first; a version not opened for 7 days goes anyway and is prepared again when opened |
| The versions the search uses | One version of each module, with its search and AI files; about 3.5 MB each | About 600 versions | Never removed; the `D8DocumentationPinnedVersionsTooLarge` alert fires when they take more than 2 GiB |
