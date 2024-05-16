{{- define "template-config-files-values"  }}
files:
 - templateName: registry-manager-config.yaml
   filePath: /config/config.yaml
{{- end }}


# Config.yaml
{{- define "registry-manager-config.yaml"  }}
---
leaderElection:
  namespace: d8-system

etcd:
  addresses:
  {{- range $etcd_addresses := $.Values.systemRegistry.internal.etcd.addresses }}
  - {{ $etcd_addresses }}
  {{- end }}

distribution:
  image: "{{ $.Values.global.modulesImages.registry.base }}@{{ $.Values.global.modulesImages.digests.systemRegistry.dockerDistribution }}"

auth:
  image: "{{ $.Values.global.modulesImages.registry.base }}@{{ $.Values.global.modulesImages.digests.systemRegistry.dockerAuth }}"

seaweedfs:
  image: "{{ $.Values.global.modulesImages.registry.base }}@{{ $.Values.global.modulesImages.digests.systemRegistry.seaweedfs }}"
{{- end }}
