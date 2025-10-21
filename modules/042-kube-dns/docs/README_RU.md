---
title: "Модуль kube-dns"
description: "Управление DNS в кластере Kubernetes с помощью CoreDNS."
---

Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.

> **Внимание!** Модуль удаляет ранее установленные kubeadm'ом Deployment, ConfigMap и RBAC для CoreDNS. При развертывании собственного CoreDNS избегайте использования имен `coredns` или `system:coredns` для любых ресурсов (Deployment, Service, ConfigMap, ServiceAccount, ClusterRole, ClusterRoleBinding). Используйте альтернативные имена, например `infra-dns`, чтобы предотвратить их автоматическое удаление Deckhouse.
