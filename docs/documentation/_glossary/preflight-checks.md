---
title: Preflight checks
description: Checks that the installer runs before installing Deckhouse Platform.
lang: en
---

The checks are grouped into general checks, checks for static clusters and checks for cloud clusters. An individual check is skipped with the `--preflight-skip-check` flag and its name, `--preflight-skip-all-checks` skips all of them, and `dhctl preflight list` prints the full list. Cluster subnets and the DNS name template are validated when the [installation configuration file](glossary.html#installation-config) is loaded and cannot be skipped.

For details, see [Installation](installing/).
