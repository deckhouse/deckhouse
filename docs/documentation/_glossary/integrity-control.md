---
title: Integrity control
description: A set of mechanisms that verify containers to ensure their security and compliance with the specified configuration, including image signature verification at startup, runtime audit of running containers and immutable images of platform modules.
lang: en
---

Deckhouse Platform provides three mechanisms: [image signature verification](glossary.html#image-signature-verification) of user workloads during Kubernetes API request validation, [runtime audit](glossary.html#runtime-audit) of running containers, and integrity protection of Deckhouse Platform modules. Cryptographic verification of image signatures at the container runtime (CRI) level with DM-Verity protection of unpacked layers is available only in the certified [editions](glossary.html#edition).

Starting with version 1.74, modules are installed as read-only EROFS images mounted through DM-Verity, which protects an installed module from being replaced or modified on the node. This mechanism is enabled automatically when the node OS supports the `erofs` kernel module. Without it Deckhouse Platform keeps working and raises an alert.

For details, see [Integrity control](architecture/security/integrity-control.html).
