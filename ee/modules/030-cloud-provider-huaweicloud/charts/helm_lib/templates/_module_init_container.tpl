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
  resources:
    requests:
      {{- include "helm_lib_module_ephemeral_storage_only_logs" . | nindent 6 }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_init_container_chown_deckhouse_volume" (list . "volume-name") }} */ -}}
{{- /* returns initContainer which chowns recursively all files and directories in passed volume */ -}}
{{- define "helm_lib_module_init_container_chown_deckhouse_volume"  }}
  {{- $context := index . 0 -}}
  {{- $volume_name := index . 1  -}}
- name: chown-volume-{{ $volume_name }}
  image: {{ include "helm_lib_module_common_image" (list $context "alpine") }}
  command: ["sh", "-c", "chown -R 64535:64535 /tmp/{{ $volume_name }}"]
  securityContext:
    runAsNonRoot: false
    runAsUser: 0
    runAsGroup: 0
  volumeMounts:
  - name: {{ $volume_name }}
    mountPath: /tmp/{{ $volume_name }}
  resources:
    requests:
      {{- include "helm_lib_module_ephemeral_storage_only_logs" . | nindent 6 }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_init_container_check_linux_kernel" (list . ">= 4.9.17") }} */ -}}
{{- /* returns initContainer which checks the kernel version on the node for compliance to semver constraint */ -}}
{{- define "helm_lib_module_init_container_check_linux_kernel"  }}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $semver_constraint := index . 1  -}} {{- /* Semver constraint */ -}}
- name: check-linux-kernel
  image: {{ include "helm_lib_module_common_image" (list $context "checkKernelVersion") }}
  {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 2 }}
  env:
  - name: KERNEL_CONSTRAINT
    value: {{ $semver_constraint | quote }}
  resources:
    requests:
      {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 6 }}
{{- end }}
