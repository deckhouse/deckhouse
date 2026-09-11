---
title: "GPU-устройства в виртуальных машинах"
permalink: ru/admin/configuration/virtualization/gpu-devices.html
description: "Проброс GPU-устройств в виртуальные машины: требования к кластеру, модуль gpu и ресурс GPUClass."
search: GPU-устройства, проброс GPU, GPUClass, видеоадаптер
lang: ru
---

{% alert level="warning" %}
Проброс GPU-устройств — экспериментальная возможность, доступная в коммерческих редакциях DP.
{% endalert %}

Модуль подключает физические GPU-устройства к виртуальным машинам через DRA (Dynamic Resource Allocation). Владелец проекта запрашивает устройство по ссылке на `GPUClass` в блоке [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) своей машины, а кластер к этому готовите вы.

Чтобы проброс заработал, обеспечьте следующее:

- [Kubernetes](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) версии не ниже 1.34 с feature gates DRA, которые нужны конфигурации вашего кластера.
- Feature gate `GPU` в настройках модуля `virtualization`.
- Установленный в кластере DRA-провайдер GPU, который публикует устройства с атрибутами `gpu.deckhouse.io`.
- Ресурс `GPUClass`, отбирающий устройства нужной модели. Модуль GPU создаёт по нему ресурс DeviceClass с таким же именем, через который устройство и выделяется машине.

Чтобы включить feature gate, добавьте его в настройки модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  settings:
    featureGates:
      - GPU
```

После этого сообщите владельцам проектов имена доступных ресурсов `GPUClass`. К одной машине подключается не более 16 устройств, а изменение блока [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) применяется только после её перезапуска.
