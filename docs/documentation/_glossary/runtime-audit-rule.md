---
title: Runtime audit rule
description: A rule of the runtime audit that describes a pattern of suspicious behaviour as a condition expression in the Falco syntax. Deckhouse Platform ships built-in rules and accepts custom ones.
lang: en
---

Built-in rules include rules for the Kubernetes audit, which reveal problems with the security of Deckhouse Platform and of the audit mechanism itself, and the `fstec` regulatory rules that meet the requirements of FSTEC of Russia order No. 118 for containerization tools. Custom rules are defined with a [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) object of the [`runtime-audit-engine`](/modules/runtime-audit-engine/) module. The [runtime audit](glossary.html#runtime-audit) matches events against all rules and produces [security events](glossary.html#security-event).

For details, see [Runtime audit architecture](architecture/security/runtime-audit.html).
