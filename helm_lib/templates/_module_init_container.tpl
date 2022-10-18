{{- /* ### Migration 11.12.2020: Remove this helper with all its usages after this commit reached RockSolid */ -}}
{{- /* Usage: {{ include "helm_lib_module_init_container_chown_nobody_volume" (list . "volume-name") }} */ -}}
{{- /* returns initContainer which chowns recursively all files and directories in passed volume */ -}}
{{- define "helm_lib_module_init_container_chown_nobody_volume"  }}
  {{- $context := index . 0 -}}
  {{- $volume_name := index . 1  -}}
- name: chown-volume-{{ $volume_name }}
  image: {{ include "helm_lib_module_common_image" (list $context "alpine") }}
  command: ["sh", "-c", "chown -R 65534:65534 /tmp/{{ $volume_name }}"]
  securityContext:
    runAsNonRoot: false
    runAsUser: 0
    runAsGroup: 0
  volumeMounts:
  - name: {{ $volume_name }}
    mountPath: /tmp/{{ $volume_name }}
{{- end }}
