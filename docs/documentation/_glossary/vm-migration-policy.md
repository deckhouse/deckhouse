---
title: VM migration policy
description: The policy that determines when a live migration of a virtual machine uses the AutoConverge mechanism, CPU throttling that guarantees the migration completes. Set by the liveMigrationPolicy parameter of the machine.
lang: en
---

AutoConverge gradually lowers the CPU frequency of the source machine so that fewer dirty memory pages appear, and completes the [live migration](glossary.html#live-migration) as soon as the transfer rate exceeds the memory change rate. It helps on a slow network at the cost of application performance in the machine.

The allowed values: `AlwaysSafe` (never throttle), `PreferSafe` (the default, no throttling, but a migration with `force=true` may throttle), `AlwaysForced` (always use AutoConverge) and `PreferForced` (use AutoConverge, but a migration with `force=false` runs without it).

For details, see [Live migration of virtual machines](user/virtualization/vm-migration.html).
