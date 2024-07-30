{{- /* Usage: {{ include "helm_lib_module_init_container_check_linux_kernel" (list . ">= 4.9.17") }} */ -}}
{{- /* returns initContainer which checks the kernel version on the node for compliance to semver constraint */ -}}
{{- define "module_init_container_check_linux_kernel"  }}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
  {{- $semver_constraint := index . 1  -}} {{- /* Semver constraint */ -}}
- name: check-linux-kernel
  image: {{ include "helm_lib_module_image" (list $context "checkKernelVersion") }}
  {{- include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . | nindent 2 }}
  env:
  - name: KERNEL_CONSTRAINT
    value: {{ $semver_constraint | quote }}
  resources:
    requests:
      {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 6 }}
{{- end }}
