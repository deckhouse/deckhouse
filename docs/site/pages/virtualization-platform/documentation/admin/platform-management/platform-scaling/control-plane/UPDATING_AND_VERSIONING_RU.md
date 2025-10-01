---
title: "Обновление Kubernetes и управление версиями"
permalink: ru/virtualization-platform/documentation/admin/platform-management/platform-scaling/control-plane/updating-and-versioning.html
lang: ru
---

## Обновление и управление версиями

Процесс обновления control plane в DVP полностью автоматизирован.

- В DVP поддерживаются последние пять версий Kubernetes.
- Control plane можно откатывать на одну минорную версию назад и обновлять на несколько версий вперёд — шаг за шагом, по одной версии за раз.
- Patch-версии (например, `1.27.3` → `1.27.5`) обновляются автоматически вместе с версией Deckhouse, и управлять этим процессом нельзя.
- Minor-версии задаются вручную в параметре `kubernetesVersion` в ресурсе ClusterConfiguration.

### Изменение версии Kubernetes

1. Откройте редактирование [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration):

   ```shell
   d8 platform edit cluster-configuration
   ```

1. Установите желаемую версию Kubernetes (`kubernetesVersion`):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterConfiguration
   cloud:
     prefix: demo-stand
     provider: Yandex
   clusterDomain: cloud.education
   clusterType: Cloud
   defaultCRI: Containerd
   kubernetesVersion: "1.30"
   podSubnetCIDR: 10.111.0.0/16
   podSubnetNodeCIDRPrefix: "24"
   serviceSubnetCIDR: 10.222.0.0/16
   ```

1. Сохраните изменения.
