{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_custom" (list . 1000 1000) }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with custom user and group */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_custom" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
{{- /* User id */ -}}
{{- /* Group id */ -}}
securityContext:
  runAsNonRoot: true
  runAsUser: {{ index . 1 }}
  runAsGroup: {{ index . 2 }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_nobody" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with user and group nobody */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_nobody" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with user and group nobody with write access to mounted volumes */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
  fsGroup: 65534
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_root" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with user and group 0 */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_root" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
securityContext:
  runAsNonRoot: false
  runAsUser: 0
  runAsGroup: 0
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_not_allow_privilege_escalation" . }} */ -}}
{{- /* returns SecurityContext parameters for Container with allowPrivilegeEscalation false */ -}}
{{- define "helm_lib_module_container_security_context_not_allow_privilege_escalation" }}
securityContext:
  allowPrivilegeEscalation: false
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_read_only_root_filesystem" . }} */ -}}
{{- /* returns SecurityContext parameters for Container with read only root filesystem */ -}}
{{- define "helm_lib_module_container_security_context_read_only_root_filesystem" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_privileged" . }} */ -}}
{{- /* returns SecurityContext parameters for Container running privileged */ -}}
{{- define "helm_lib_module_container_security_context_privileged" }}
securityContext:
  privileged: true
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_privileged_read_only_root_filesystem" . }} */ -}}
{{- /* returns SecurityContext parameters for Container running privileged with read only root filesystem */ -}}
{{- define "helm_lib_module_container_security_context_privileged_read_only_root_filesystem" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
securityContext:
  privileged: true
  readOnlyRootFilesystem: true
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" . }} */ -}}
{{- /* returns SecurityContext for Container with read only root filesystem and all capabilities dropped  */ -}}
{{- define "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add"  (list . (list "KILL" "SYS_PTRACE")) }} */ -}}
{{- /* returns SecurityContext parameters for Container with read only root filesystem, all dropped and some added capabilities */ -}}
{{- define "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
{{- /* List of capabilities */ -}}
securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
    add: {{ index . 1 | toJson }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_capabilities_drop_all_and_add"  (list . (list "KILL" "SYS_PTRACE")) }} */ -}}
{{- /* returns SecurityContext parameters for Container with all dropped and some added capabilities */ -}}
{{- define "helm_lib_module_container_security_context_capabilities_drop_all_and_add" }}
{{- /* Dot object (.) with .Values, .Chart, etc */ -}}
{{- /* List of capabilities */ -}}
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
    add: {{ index . 1 | toJson }}
{{- end }}
