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
