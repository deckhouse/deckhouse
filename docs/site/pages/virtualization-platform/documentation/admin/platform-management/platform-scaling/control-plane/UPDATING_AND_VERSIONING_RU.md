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
- Minor-версии задаются [параметром `kubernetesVersion`](/modules/control-plane-manager/configuration.html#parameters-kubernetesversion) ModuleConfig модуля [`control-plane-manager`](/modules/control-plane-manager/).

### Изменение версии Kubernetes

1. Отредактируйте ModuleConfig модуля `control-plane-manager`:

   ```shell
   d8 k edit mc control-plane-manager
   ```

1. Установите желаемую версию Kubernetes в `spec.settings.kubernetesVersion`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 3
     enabled: true
     settings:
       kubernetesVersion: "1.30"
   ```

   Укажите `kubernetesVersion: "Automatic"`, чтобы отслеживать версию Kubernetes, которая считается стабильной для текущего релиза Deckhouse. Если параметр не задан, Deckhouse использует устаревшее поле `ClusterConfiguration.kubernetesVersion` (если оно есть), иначе — версию по умолчанию текущего релиза.

1. Сохраните изменения.

{% alert level="warning" %}
Не задавайте `kubernetesVersion` в [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) — поле устарело. Явный пин в ClusterConfiguration без переноса в ModuleConfig `control-plane-manager` приводит к алерту [D8ObsoleteKubernetesVersionInClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/alerts.html#control-plane-manager-d8obsoletekubernetesversioninclusterconfiguration).
{% endalert %}
