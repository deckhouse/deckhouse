---
title: Deckhouse subsystem
permalink: en/architecture/deckhouse/
search: Deckhouse subsystem, Deckhouse controller
description: Architecture of the Deckhouse subsystem in Deckhouse Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

This subsection describes the architecture of the modules that are part of the Deckhouse subsystem of Deckhouse Platform (DP).

The Deckhouse subsystem includes the following modules:

* [`deckhouse`](/modules/deckhouse/): Deckhouse controller.
* [`console`](/modules/console/stable/): Deckhouse web UI.
* [`deckhouse-tools`](/modules/deckhouse-tools/): Provides a web interface for downloading the [`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/) CLI utility.
* [`documentation`](/modules/documentation/): Provides a web interface with documentation corresponding to the running DP version.
* [`registry`](/modules/registry/): Manages the configuration of DP components
  responsible for working with the container registry and provides an internal image storage.
