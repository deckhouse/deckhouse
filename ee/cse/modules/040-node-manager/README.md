# CSE edit

## templates/nvidia-gpu
## templates/early-oom
## templates/machine-controller-manager
## monitoring/prometheus-rules/early-oom.tpl

1. mapped to an empty folder to remove the functionality

## openapi/config-values.yaml
## openapi/doc-ru-config-values.yaml
 
1. remove .earlyOomEnabled

## crds/mcm.yaml

1. Have the CRD cloud providers been removed? It's unclear why not all of them.

## hooks/gpu_enabled.go
## hooks/mig_custom_config_name.go

1. Disable gpu
