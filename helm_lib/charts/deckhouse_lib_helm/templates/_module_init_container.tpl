{{- /* ### Migration 11.12.2020: Remove this helper with all its usages after this commit reached RockSolid */ -}}
{{- /* Usage: {{ include "helm_lib_module_init_container_chown_nobody_volume" (list . "volume-name") }} */ -}}
{{- /* returns initContainer which chowns recursively all files and directories in passed volume */ -}}
{{- define "helm_lib_module_init_container_chown_nobody_volume"  }}
  {{- $context := index . 0 -}}
  {{- $volume_name := index . 1  -}}
  {{- $image := "alpine" -}}
  {{- if hasKey $context.Values.global.modulesImages.digests.common "init" -}}
    {{- $image = "init" -}}
  {{- end -}}
- name: chown-volume-{{ $volume_name }}
  image: {{ include "helm_lib_module_common_image" (list $context $image) }}
  command: ["sh", "-c", "chown -R 65534:65534 /tmp/data"]
  securityContext:
    runAsNonRoot: false
    readOnlyRootFilesystem: true
    runAsUser: 0
    runAsGroup: 0
  volumeMounts:
  - name: {{ $volume_name }}
    mountPath: /tmp/data
  resources:
    requests:
      {{- include "helm_lib_module_ephemeral_storage_only_logs" . | nindent 6 }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_init_container_chown_deckhouse_volume" (list . "volume-name") }} */ -}}
{{- /* returns initContainer which chowns recursively all files and directories in passed volume */ -}}
{{- define "helm_lib_module_init_container_chown_deckhouse_volume"  }}
  {{- $context := index . 0 -}}
  {{- $volume_name := index . 1  -}}
  {{- $image := "alpine" -}}
  {{- if hasKey $context.Values.global.modulesImages.digests.common "init" -}}
    {{- $image = "init" -}}
  {{- end -}}
- name: chown-volume-{{ $volume_name }}
  image: {{ include "helm_lib_module_common_image" (list $context $image) }}
  command: ["sh", "-c", "chown -R 64535:64535 /tmp/data"]
  securityContext:
    runAsNonRoot: false
    readOnlyRootFilesystem: true
    runAsUser: 0
    runAsGroup: 0
  volumeMounts:
  - name: {{ $volume_name }}
    mountPath: /tmp/data
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
  {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" (dict "ro" true) | nindent 2 }}
  env:
  - name: KERNEL_CONSTRAINT
    value: {{ $semver_constraint | quote }}
  resources:
    requests:
      {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 6 }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_iptables_wrapper_init_container" (list . "foo") }} */ -}}
{{- /* returns iptables-wrapper-init container */ -}}
{{- define "helm_lib_module_iptables_wrapper_init_container"  }}
  {{- $context := index . 0 -}} {{- /* Template context with .Values, .Chart, etc */ -}}
- name: iptables-wrapper-init
  {{- include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add" (list . (list "NET_ADMIN" "NET_RAW")) | nindent 2 }}
    runAsNonRoot: false
    unAsUser: 0
    runAsGroup: 0
  image: {{ include "helm_lib_module_image" (list $context "iptablesWrapperInit") }}
  command:
    - /bin/bash
    - -ec
    - "/usr/bin/cp /iptables-wrapper /sbin/ -rv && /usr/bin/cp /_sbin/* /sbin/ -rv && /usr/bin/cp /relocate/sbin/* /sbin/ -rv && /sbin/iptables --version && /usr/bin/rm /sbin/iptables-wrapper -v"
  volumeMounts:
  - mountPath: /sbin
    name: sbin
  - name: xtables-lock
    mountPath: /run/xtables.lock      
  resources:
    requests:
      {{- include "helm_lib_module_ephemeral_storage_logs_with_extra" 10 | nindent 6 }}
  {{- if not ( $context.Values.global.enabledModules | has "vertical-pod-autoscaler") }}
    {{- include "iptables_wrapper_init_resources" . | nindent 4 }}
  {{- end }}
{{- end }}

        
