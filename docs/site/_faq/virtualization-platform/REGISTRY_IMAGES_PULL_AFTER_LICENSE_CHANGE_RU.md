---
title: Как восстановить кластер, если после смены лицензии образы из registry.deckhouse.io не загружаются?
section: platform_management
lang: ru
---

После смены лицензии на кластере с `containerd v1` и удаления устаревшей лицензии образы из `registry.deckhouse.io` могут перестать загружаться. При этом на узлах остаётся устаревший файл конфигурации `/etc/containerd/conf.d/dvcr.toml`, который не удаляется автоматически. Из-за него не запускается модуль `registry`, без которого не работает DVCR.

Манифест NodeGroupConfiguration (NGC) после применения удалит файл на узлах. После запуска модуля `registry` манифест нужно удалить, так как это разовое исправление.

1. Сохраните манифест в файл (например, `containerd-dvcr-remove-old-config.yaml`):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-dvcr-remove-old-config.sh
   spec:
     weight: 32 # Должен быть в диапазоне 32–90
     nodeGroups: ["*"]
     bundles: ["*"]
     content: |
       # Copyright 2023 Flant JSC
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #      http://www.apache.org/licenses/LICENSE-2.0
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.

       rm -f /etc/containerd/conf.d/dvcr.toml
   ```

1. Примените сохранённый манифест:

   ```bash
   d8 k apply -f containerd-dvcr-remove-old-config.yaml
   ```

1. Проверьте, что модуль `registry` запущен:

   ```bash
   d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
   ```

   Пример вывода при успешном запуске:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Удалите разовый манифест NodeGroupConfiguration:

   ```bash
   d8 k delete -f containerd-dvcr-remove-old-config.yaml
   ```

Подробнее о миграции см. в статье [Миграция container runtime на containerd v2](/products/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/migrating.html).
