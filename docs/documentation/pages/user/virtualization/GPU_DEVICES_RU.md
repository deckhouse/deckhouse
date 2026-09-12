---
title: "GPU-устройства в виртуальной машине"
permalink: ru/user/virtualization/gpu-devices.html
description: "Подключение предоставленного администратором GPU-устройства к виртуальной машине проекта."
search: GPU в ВМ, проброс GPU, GPUClass, видеоадаптер
lang: ru
---

{% alert level="warning" %}
Проброс GPU-устройств — экспериментальная возможность, доступная в коммерческих редакциях DP.
{% endalert %}

DP подключает физические GPU-устройства к виртуальным машинам с помощью DRA (Dynamic Resource Allocation). Устройство запрашивается по ссылке на `GPUClass` в блоке [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).

Ресурсы `GPUClass` готовит администратор, поэтому узнайте у него, какие классы доступны в кластере.

Чтобы запросить GPU-устройство, добавьте блок [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) в спецификацию машины:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  # ... другие настройки ВМ ...
  gpus:
    - gpuClassName: nvidia-h100
```

В параметре `gpuClassName` укажите имя существующего ресурса `GPUClass`. Чтобы подключить несколько устройств, добавьте в список ещё элементы, порядок в нём не важен. К одной машине подключается не более 16 устройств.

Изменение блока [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) применяется только после перезапуска виртуальной машины.
