---
title: Integrity control
permalink: en/architecture/security/integrity-control.html
search: integrity control, container integrity, container validation, integrity checking, cosign, image signature verification
description: Architecture of the integrity control mechanisms in Deckhouse Kubernetes Platform.
---

Integrity control is a set of mechanisms for verifying containers to ensure their security and compliance with the specified configuration.

Deckhouse Kubernetes Platform (DKP) provides the following mechanisms:

- Integrity control of user workloads at startup: image signature verification during Kubernetes API request validation.
- Integrity control of running containers: runtime audit.
- Integrity protection of DKP modules: modules are installed as immutable images.

{% alert level="warning" %}
Integrity control of images at the container runtime (CRI) level — cryptographic image signature verification and protection of unpacked layers from modification — is implemented only in the DKP CSE Lite and CSE Pro editions. In other editions, this mechanism is not available; refer to ["Integrity control of images at the CRI level"](#integrity-control-of-images-at-the-cri-level).
{% endalert %}

## Integrity control of images at the CRI level

Cryptographic image signature verification when an image is pulled and a container is started, as well as immutability control of the unpacked image layers using DM-Verity, are implemented at the containerd level and are available **only in the DKP CSE Lite and CSE Pro editions**. Platform image signatures are verified using the set of public certificates built into containerd; the private signing key belongs to the platform vendor, so a custom key cannot be added for verification. For details on this mechanism, refer to the DKP CSE documentation.

In other DKP editions (CE, BE, SE, SE+, EE), there is no image signature verification at the CRI level:

- When containerd v2 is used, the EROFS snapshotter is applied: each OCI image layer is converted into a separate file in the EROFS format and mounted read-only. This reduces the risk of tampering with images already unpacked on a node, but does not eliminate it completely and does not replace image authenticity verification. For details on switching to containerd v2, refer to ["Migrating container runtime to containerd v2"](../../admin/configuration/platform-scaling/node/migrating.html).
- The SHA-256 hash verification performed by containerd when pulling an image protects against data corruption in transit, but does not confirm the image authenticity: it does not make it possible to determine who built the image.

To control the integrity and authenticity of user application images in these editions, use signature verification during Kubernetes API request validation (refer to the section below).

## Integrity control of user workloads at startup

{% alert level="warning" %}
This feature is available in the following DKP editions: SE+, EE, CSE Lite, CSE Pro.
{% endalert %}

The integrity and authenticity of user application images are verified during Kubernetes API request validation, that is, before a Pod is created, rather than at the CRI level. The mechanism is implemented by the [`admission-policy-engine`](admission-policy-engine.html) module and relies on signatures created with [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/).

The sequence of integrity checks at startup:

1. At the build stage, the image is signed with a private key using Cosign. The signature is published in the container image registry as a separate tag and does not modify the image itself.
1. A SecurityPolicy resource with the [`policies.verifyImageSignatures`](/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) parameter is created in the cluster, listing the public keys and the image address patterns.
1. When a Pod is created or updated, the validating webhook passes the request to Gatekeeper, which in turn queries the Ratify component.
1. Ratify verifies the image signature against the specified public keys.
1. If the signature is missing or does not match the specified keys, Pod creation is denied.

Additionally, you can restrict the list of container image registries that Pods are allowed to be started from using the `policies.allowedRepos` parameter of the [OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy) resource.

For more details on the configuration, refer to ["Image signature verification"](../../admin/configuration/security/policies.html#image-signature-verification).

## Integrity control of running containers

Runtime auditing in DKP includes analyzing Linux kernel events and Kubernetes API audit events.
This makes it possible to track whether applications in pods are running unchanged, conform to their expected state,
and have not been modified.

Auditing uses:

- Built-in rules
- Custom rules that can be added using the [Falco condition syntax](https://falco.org/docs/concepts/rules/conditions/)

Integrity control of running containers can detect threats such as launching command-line shells inside containers or pods,
discovering containers running in privileged mode, mounting insecure paths into containers, or attempts to read sensitive data.

For more details on configuring runtime audit, refer to ["Runtime audit"](../../admin/configuration/security/events/runtime-audit.html).

## Integrity protection of DKP modules

Starting with version 1.74, DKP modules are installed not as a directory with files, but as an image in the EROFS format that is mounted read-only through the DM-Verity kernel mechanism. This protects the contents of an already installed module from being modified on the node: the module files cannot be overwritten in the mounted image, and DM-Verity verifies the image blocks when they are accessed.

The mechanism is enabled automatically if the `erofs` filesystem is registered in the kernel on the node running the DKP controller (a master node by default), that is, the corresponding kernel module is either loaded or built into the kernel. To check it, run the following command:

```shell
grep -w erofs /proc/filesystems
```

If the `erofs` filesystem is not available, DKP will continue to operate and will install modules the regular way, without protecting their integrity. The switch happens without a dedicated alert, so make sure to check `erofs` availability in advance.

{% alert level="warning" %}
DKP loads the `erofs` kernel module and configures it to load automatically only on nodes that use containerd v2 as the container runtime. If master nodes use containerd v1, the `erofs` module may end up not loaded, and DKP modules will be installed without integrity protection, even if the kernel supports `erofs`.

To make sure the mechanism is engaged, either switch master nodes to [containerd v2](../../admin/configuration/platform-scaling/node/migrating.html), or have the operating system load the `erofs` module.
{% endalert %}

Note the following when using this mechanism:

- The mechanism protects modules from being modified after installation, but does not confirm their authenticity: there is no cryptographic signature verification for modules. The container image registry the module is pulled from remains the source of trust.
- The mechanism applies only to DKP modules and does not cover user application images.
- The mechanism is available in all DKP editions and does not require to be enabled separately.
