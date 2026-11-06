---
title: Admission policy
description: A rule that is applied to objects such as pods and Services when they are created or modified in the cluster, based on their manifests, and not while they run.
lang: en
---

Admission policies formalize the parameters that are allowed or prohibited in object manifests. Deckhouse Platform implements them with the [`admission-policy-engine`](/modules/admission-policy-engine/) module and divides them into three categories: [Pod Security Standards](glossary.html#pod-security-standards), operational policies ([OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy)) that validate parameters not related to security, such as allowed image registries, and security policies ([SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy)) that validate security-related parameters, such as privileged mode or host namespaces.

The policies complement each other: if several apply to a [namespace](glossary.html#namespace), an object must satisfy all of them. When a policy changes, existing pods are validated on the next restart, and Deckhouse Platform raises an [alert](glossary.html#alert) for running objects that violate a policy. [SecurityPolicyException](/modules/admission-policy-engine/cr.html#securitypolicyexception) allows fine-grained exceptions for individual pods or containers.

For details, see [Security policies](admin/configuration/security/policies.html).
