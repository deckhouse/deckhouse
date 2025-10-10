---
title: Integrity control
permalink: en/architecture/security/integrity-control.html
---

Integrity control is a set of mechanisms for verifying containers to ensure their security
and compliance with the specified configuration.

The Deckhouse Kubernetes Platform (DKP) implements integrity control for system containers and user application containers. It works as follows:

- When containers are started.
- While containers are running.

## System container integrity control

System container integrity control includes:

- Signature verification when loading images and starting containers.
- Ensuring container immutability at startup and during operation.

![System container integrity control in DKP](../../images/architecture/security/integrity-control-system-applications-en.png)

### Signature verification when loading an image and starting a system container

Signature verification is performed for system containers running in the `d8-*` and `kube-system` namespaces.

Signing DKP system container images uses the principle of an attached signature. The signature is added to the image manifest in the `io.deckhouse.delivery-kit.signature` annotation.

When loading the image and starting the container, the signature is verified using a set of public certificates built into containerd. If there is no signature on the locally downloaded image, the image is considered corrupted and must be re-downloaded from the registry.

### Ensuring the immutability of system containers

The immutability of system containers at startup and during operation is achieved through the following measures:

- Using the EROFS-snapshotter instead of OverlayFS in the containerd v2 container runtime (CRI). EROFS-snapshotter converts each layer of the OCI image to EROFS format: layers become files containing content rather than directories. This makes each layer immutable: it is no longer possible to replace anything in an existing container.
- Moving away from using an `upperdir` layer with read-and-write (RW) permissions. When building an image, a standard layer is created where commonly used mount points (`/tmp`, `/etc/resolv.conf`, etc.) are collected. This layer is read-only (RO). It is automatically connected to all containers created from the image.
- Use of the DM-Verity control mechanism. DM-Verity is a Linux kernel component that allows on-the-fly checks to ensure that the data on the disk has not been modified outside of the controlled process. During the build phase, Deckhouse Delivery Kit calculates the DM-Verity checksum for each image layer and adds it as a layer annotation in the OCI manifest.
When deploying an image, containerd enables DM-Verity verification and compares the received checksum with the checksum from the manifest. When the container is running, the presence of the DM-Verity tag on EROFS layers is checked, and the hash for DM-verity is compared with the one specified in the signed image manifest.

## Integrity control of user application containers

DKP implements integrity control of user application containers at startup and during operation.

![User application container integrity control in DKP](../../images/architecture/security/integrity-control-user-applications-en.png)

### Integrity control when starting user application containers

DKP provides application container integrity control at the CRI level.

After downloading an application image, DKP verifies its integrity by checking the SHA-256 hash.  
A container can only be started if the checksum verification succeeds.

The sequence of integrity checks at startup:

1. The image is loaded into the node's local storage.
1. Image metadata is extracted, including the SHA-256 hash.
1. SHA-256 hash is verified by comparing it with the reference value.
1. If the hashes match, the check passes. If they don't match, the image is not started.

To enhance security, you can also configure image pull policies
using [security policies](../../admin/configuration/security/policies.html) to ensure
that only up-to-date image versions are used for container startup.

## Integrity control of running user application containers

Security event auditing in DKP includes analyzing Linux kernel events and Kubernetes API audit events.
This makes it possible to track whether applications in pods are running unchanged, conform to their expected state,
and have not been modified.

Auditing uses:

- Built-in rules
- Custom rules that can be added using the [Falco condition syntax](https://falco.org/docs/concepts/rules/conditions/)

Integrity control of running containers can detect threats such as launching command-line shells inside containers or pods,
discovering containers running in privileged mode, mounting insecure paths into containers, or attempts to read sensitive data.

For more details on configuring security audits, refer to [Security event audit](../../admin/configuration/security/events/runtime-audit.html).
