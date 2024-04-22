######################################
#          Config files info         #
######################################
{{- define "config-files-info"  }}
files:
 - templateName: registry-manager-config.yaml
   filePath: /config/config.yaml

secrets:
 - templateName: system-registry-manager-configs
   annotationNameForCheckSum: manager-configs/CheckSum
{{- end }}

######################################
#             Config.yaml            #
######################################
{{- define "registry-manager-config.yaml"  }}
---
manager:
  namespace: d8-system
  serviceName: system-registry-manager
  daemonsetName: system-registry-manager
  workerPort: 8097
  leaderElection: {}

registry:
  mode: {{ $.Values.systemRegistry.mode }}
  {{- if eq $.Values.systemRegistry.mode "Proxy"  }}
  proxy:
    host: {{ $.Values.systemRegistry.proxy.host }}
    scheme: {{ $.Values.systemRegistry.proxy.scheme }}
    ca: {{ $.Values.systemRegistry.proxy.ca }}
    path: {{ $.Values.systemRegistry.proxy.path }}
    user: {{ $.Values.systemRegistry.proxy.user }}
    password: {{ $.Values.systemRegistry.proxy.password }}
    storageMode: {{ $.Values.systemRegistry.proxy.storageMode }}
  {{- end }}
  {{- if eq $.Values.systemRegistry.mode "Detached" }}
  detached:
    storageMode: {{ $.Values.systemRegistry.detached.storageMode }}
  {{- end }}
images:
  dockerDistribution: {{ $.Values.global.modulesImages.registry.base }}@{{ $.Values.global.modulesImages.digests.systemRegistry.dockerDistribution }}
  dockerAuth: {{ $.Values.global.modulesImages.registry.base }}@{{ $.Values.global.modulesImages.digests.systemRegistry.dockerAuth }}
  seaweedfs: {{ $.Values.global.modulesImages.registry.base }}@{{ $.Values.global.modulesImages.digests.systemRegistry.seaweedfs }}
{{- end }}


##########################################
# secret system-registry-manager-configs #
##########################################
{{- define "system-registry-manager-configs"  }}
---
apiVersion: v1
kind: Secret
metadata:
  name: system-registry-manager-configs
  namespace: d8-system
  {{- include "helm_lib_module_labels" (list $ (dict "app" "d8-system-registry")) | nindent 2 }}
type: Opaque
data:
  {{- $configFilesInfo := (include "config-files-info" $ ) | fromYaml }}
  {{- range $configFilesInfo.files }}
  "{{ .templateName }}": {{ include .templateName $  | b64enc }}
  {{- end }}
{{- end }}
