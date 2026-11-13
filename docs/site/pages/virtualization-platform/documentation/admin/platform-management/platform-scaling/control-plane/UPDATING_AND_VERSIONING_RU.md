---
title: "Обновление Kubernetes и управление версиями"
permalink: ru/virtualization-platform/documentation/admin/platform-management/platform-scaling/control-plane/updating-and-versioning.html
lang: ru
---

## Обновление и управление версиями

Процесс обновления control plane в DVP полностью автоматизирован.

- В DVP поддерживаются последние пять версий Kubernetes.
- Control plane можно откатывать на одну минорную версию назад и обновлять на несколько версий вперёд — шаг за шагом, по одной версии за раз.
- Patch-версии (например, `1.35.3` → `1.35.5`) обновляются автоматически вместе с версией Deckhouse, и управлять этим процессом нельзя.
- Minor-версии задаются вручную в [параметре `kubernetesVersion`](/modules/control-plane-manager/configuration.html#parameters-kubernetesversion) ModuleConfig `control-plane-manager`.

### Изменение версии Kubernetes

1. Откройте редактирование ModuleConfig `control-plane-manager`:

   ```shell
   d8 k edit mc control-plane-manager
   ```

1. Установите желаемую версию Kubernetes в параметре `settings.kubernetesVersion`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 3
     enabled: true
     settings:
       kubernetesVersion: "1.35"
   ```

   {% alert level="info" %}
   Поле `kubernetesVersion` [ресурса ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration), которое использовалось ранее для выбора версии Kubernetes, устарело. Пока параметр `settings.kubernetesVersion` модуля `control-plane-manager` не задан, версию Kubernetes определяет это поле. Если параметр задан, используется его значение.
   {% endalert %}

1. Сохраните изменения.
