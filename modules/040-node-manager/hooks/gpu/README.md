# gpu

GPU-related hooks, owned by the GPU team. GPU nodes are served by the external `gpu` module; node-manager only keeps the NodeGroup `spec.gpu` API that the module reads.

- `metrics_gpu_module_disabled` exports a metric for every NodeGroup that has `spec.gpu` while the `gpu` module is disabled. The `NodeGroupGPUModuleDisabled` alert is built on it.
