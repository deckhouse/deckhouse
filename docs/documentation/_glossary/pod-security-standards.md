---
title: Pod Security Standards
description: Three levels of pod security policies, privileged, baseline and restricted, that Deckhouse Platform applies to namespaces with the [`admission-policy-engine`](/modules/admission-policy-engine/) module.
lang: en
---

Pod Security Standards are one of the categories of [admission policies](glossary.html#admission-policy). The level `privileged` imposes no restrictions, `baseline` prevents the most common privilege escalation techniques, `restricted` sets the strictest requirements for pods. Deckhouse Platform takes only the policy descriptions from Kubernetes and enforces them with Gatekeeper rather than with the Kubernetes Pod Security Admission controller.

The default level is `baseline`. It applies to [namespaces](glossary.html#namespace) without the `security.deckhouse.io/pod-policy` label, except the system ones. The label assigns a level to one namespace, the [`admission-policy-engine`](/modules/admission-policy-engine/) module setting changes the default.

For details, see [Security policies](admin/configuration/security/policies.html).
