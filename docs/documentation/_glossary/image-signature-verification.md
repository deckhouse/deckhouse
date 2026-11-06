---
title: Image signature verification
description: Verification of the integrity and authenticity of user application images during Kubernetes API request validation, before a pod is created, against Cosign signatures and the public keys listed in a [SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy).
lang: en
---

At the build stage the image is signed with a private key using Cosign. The signature is published in the registry as a separate tag and does not change the image. In the cluster a SecurityPolicy with the `verifyImageSignatures` parameter lists the public keys, and the [`admission-policy-engine`](/modules/admission-policy-engine/) module with Ratify verifies the signature of every image when the pod is created. This is the part of [integrity control](glossary.html#integrity-control) available in all [editions](glossary.html#edition). Verification at the container runtime level exists only in Certified Core and Certified Pro.

For details, see [Integrity control](architecture/security/integrity-control.html).
