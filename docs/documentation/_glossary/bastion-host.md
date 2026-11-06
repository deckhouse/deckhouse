---
title: Bastion host
description: A host through which the master node is reached over SSH when the cluster nodes have no public IP addresses.
lang: en
---

A bastion host is needed in [layouts](glossary.html#layout) where nodes are created without public IP addresses. The [installer](glossary.html#installer) connects through it when run with the `--ssh-bastion-host`, `--ssh-bastion-user` and, if needed, `--ssh-bastion-port` parameters.
