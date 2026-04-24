---
title: "FAQ: Platform management"
permalink: en/virtualization-platform/documentation/faq/platform-management.html
---

## How to increase the DVCR size?

The DVCR volume size is set in the `virtualization` module ModuleConfig (`spec.settings.dvcr.storage.persistentVolumeClaim.size`). The new value must be greater than the current one.

1. Check the current DVCR size:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Example output:

   ```console
    {"size":"58G","storageClass":"linstor-thick-data-r1"}
   ```

1. Increase `size` using `patch` (set the value you need):

   ```shell
   d8 k patch mc virtualization \
     --type merge -p '{"spec": {"settings": {"dvcr": {"storage": {"persistentVolumeClaim": {"size":"59G"}}}}}}'
   ```

   Example output:

   ```console
   moduleconfig.deckhouse.io/virtualization patched
   ```

1. Verify that ModuleConfig shows the new size:

   ```shell
   d8 k get mc virtualization -o jsonpath='{.spec.settings.dvcr.storage.persistentVolumeClaim}'
   ```

   Example output:

   ```console
   {"size":"59G","storageClass":"linstor-thick-data-r1"}
   ```

1. Check the current DVCR status:

   ```shell
   d8 k get pvc dvcr -n d8-virtualization
   ```

   Example output:

   ```console
   NAME STATUS VOLUME                                    CAPACITY    ACCESS MODES   STORAGECLASS           AGE
   dvcr Bound  pvc-6a6cedb8-1292-4440-b789-5cc9d15bbc6b  57617188Ki  RWO            linstor-thick-data-r1  7d
   ```

## How to restore the cluster if images from registry.deckhouse.io cannot be pulled after a license change?

After a license change on a cluster with `containerd v1` and removal of the outdated license, images from `registry.deckhouse.io` may stop being pulled. Nodes then retain the outdated configuration file `/etc/containerd/conf.d/dvcr.toml`, which is not removed automatically. Because of it, the `registry` module does not start, and without it DVCR does not work.

Applying a NodeGroupConfiguration (NGC) manifest removes the file on the nodes. After the `registry` module starts, delete the manifest, since this is a one-time fix.

1. Save the manifest to a file (for example, `containerd-dvcr-remove-old-config.yaml`):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-dvcr-remove-old-config.sh
   spec:
     weight: 32 # Must be in range 32–90
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

1. Apply the saved manifest:

   ```bash
   d8 k apply -f containerd-dvcr-remove-old-config.yaml
   ```

1. Verify that the `registry` module is running:

   ```bash
   d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
   ```

   Example output when the `registry` module has started successfully:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Delete the one-time NodeGroupConfiguration manifest:

   ```bash
   d8 k delete -f containerd-dvcr-remove-old-config.yaml
   ```

For more information on migration, see [Migrating container runtime to containerd v2](/products/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/migrating.html).
