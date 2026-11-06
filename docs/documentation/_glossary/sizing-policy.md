---
title: Sizing policy
description: The block of a virtual machine class that defines which combinations of cores, core fraction and memory are allowed for virtual machines of this class.
lang: en
---

A policy consists of a list of rules, each applying to its own range of cores. The ranges must not overlap. Changes to the policy affect existing virtual machines: a machine that no longer meets the requirements gets the `SizingPolicyMatched` condition with the `False` status. The allowed values of the core fraction set in the policy define the degree of [CPU oversubscription](glossary.html#cpu-oversubscription).

For details, see [Virtual machine sizing policy](admin/configuration/virtualization/vm-classes-sizing.html).
