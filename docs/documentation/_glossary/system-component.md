---
title: System component
description: A process or a group of processes that run in a container or as a systemd service on a node and keep the platform and its modules running.
lang: en
---

Examples of system components are kubelet and containerd.

Some system components run in [user namespaces](glossary.html#user-namespace). For example, when a user creates a virtual machine or a managed service, the pods that support this object start in the user namespace.
