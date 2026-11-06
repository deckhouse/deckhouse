---
title: Deckhouse CLI (d8)
description: The command-line interface for working with the products of the Deckhouse ecosystem. It combines commands for Kubernetes, Deckhouse Platform administration, application delivery, image copying, backups, diagnostics, virtualization and other tasks.
lang: en
---

Starting from release 1.59, `d8` is installed on all Deckhouse Platform cluster nodes. It can also be installed on an administrator machine with [trdl](https://trdl.dev/) (an open-source tool for securely delivering and updating binary files) or from the executable file. Commands are grouped by purpose: `d8 k` is equivalent to `kubectl`, `d8 system` works with platform settings, modules, queues and packages, `d8 mirror` copies platform images, `d8 v` manages virtual machines, `d8 backup` creates backups.

For details, see [Deckhouse CLI overview and installation](cli/d8/).
