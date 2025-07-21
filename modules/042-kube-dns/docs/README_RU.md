---
title: "Модуль kube-dns"
description: "Управление DNS в кластере Kubernetes с помощью CoreDNS."
---

Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.

> **Внимание!** Модуль удаляет ранее установленные kubeadm'ом Deployment, ConfigMap и RBAC для CoreDNS.
