---
title: bashible
description: The service that runs on every cluster node and executes the bash scripts, also called bashible, that configure the node according to its node group.
lang: en
---

The scripts are Go templates grouped into bundles. An individual script is called a step. They are written so that they can be re-run safely. The service re-runs the steps when the configuration checksum of the [node group](glossary.html#node-group) changes. The steps are stored on the node in `/var/lib/bashible/bundle_steps/`. [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) adds custom steps to the bashible configuration of a group.

bashible is part of the Cluster & Infrastructure [subsystem](glossary.html#subsystem). It runs at the operating system level as a system service and is not part of the [`node-manager`](/modules/node-manager/) module.

For details, see [Bashible](architecture/cluster-and-infrastructure/node-management/bashible.html).
