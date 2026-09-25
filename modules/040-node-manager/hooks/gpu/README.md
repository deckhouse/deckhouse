# gpu

Hooks of the NVIDIA GPU support, owned by the GPU team. They exist while the built-in GPU support lives in node-manager. The external `gpu` module replaces it, and these hooks step aside when that module is enabled.

- `gpu_enabled` labels the nodes of GPU NodeGroups with the GPU mode, the device plugin configuration and the MIG configuration, so the NVIDIA components pick the right configuration per node.
- `mig_custom_config_name` derives a stable name for every custom MIG configuration of a NodeGroup from its content. The MIG configuration templates refer to that name.
- `metrics_gpu_in_core_deprecated` exports a metric for every NodeGroup that still uses the built-in GPU support while the `gpu` module is disabled. The deprecation alert is built on it.

In CSE the first two hooks are replaced by empty files at build time, see `ee/cse/modules/040-node-manager/hooks/gpu`.
