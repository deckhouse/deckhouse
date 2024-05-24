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
 - templateName: system-registry-manager-pki
   annotationNameForCheckSum: manager-pki/CheckSum
{{- end }}


######################################
#             Config.yaml            #
######################################
{{- define "registry-manager-config.yaml"  }}
---
leaderElection:
  namespace: d8-system
etcd:
  addresses:
  {{- range $etcd_addresses := $.Values.systemRegistry.internal.etcd.addresses }}
  - {{ $etcd_addresses }}
  {{- end }}
registry:
  registryMode: {{ $.Values.systemRegistry.registryMode }}
  upstreamRegistry:
      upstreamRegistryHost: {{ $.Values.systemRegistry.upstreamRegistry.upstreamRegistryHost }}
      upstreamRegistryScheme: {{ $.Values.systemRegistry.upstreamRegistry.upstreamRegistryScheme }}
      upstreamRegistryCa: {{ $.Values.systemRegistry.upstreamRegistry.upstreamRegistryCa }}
      upstreamRegistryPath: {{ $.Values.systemRegistry.upstreamRegistry.upstreamRegistryPath }}
      upstreamRegistryUser: {{ $.Values.systemRegistry.upstreamRegistry.upstreamRegistryUser }}
      upstreamRegistryPassword: {{ $.Values.systemRegistry.upstreamRegistry.upstreamRegistryPassword }}
images:
  systemRegistry:
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


##########################################
#   secret system-registry-manager-pki   #
##########################################
{{- define "system-registry-manager-pki"  }}
apiVersion: v1
kind: Secret
metadata:
  name: system-registry-manager-pki
  namespace: d8-system
  {{- include "helm_lib_module_labels" (list $ (dict "app" "d8-system-registry")) | nindent 2 }}
type: Opaque
data:
  {{- range $.Values.systemRegistry.internal.pki.data }}
  {{ .key }}: {{ .value }}
  {{- end }}
{{- end }}
