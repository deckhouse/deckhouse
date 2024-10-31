{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_custom" (list . 1000 1000) }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with custom user and group */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_custom" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
{{- /* User id */ -}}
{{- /* Group id */ -}}
securityContext:
  runAsNonRoot: true
  runAsUser: {{ index . 1 }}
  runAsGroup: {{ index . 2 }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_nobody" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with user and group "nobody" */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_nobody" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with user and group "nobody" with write access to mounted volumes */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_nobody_with_writable_fs" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
  fsGroup: 65534
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_deckhouse" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with user and group "deckhouse" */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_deckhouse" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 64535
  runAsGroup: 64535
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_deckhouse_with_writable_fs" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with user and group "deckhouse" with write access to mounted volumes */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_deckhouse_with_writable_fs" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 64535
  runAsGroup: 64535
  fsGroup: 64535
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_run_as_user_deckhouse_pss_restricted" . }} */ -}}
{{- /* returns SecurityContext parameters for Container with user and group "deckhouse" plus minimal required settings to comply with the Restricted mode of the Pod Security Standards */ -}}
{{- define "helm_lib_module_container_security_context_run_as_user_deckhouse_pss_restricted" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
  drop:
  - all
  runAsGroup: 64535
  runAsNonRoot: true
  runAsUser: 64535
  seccompProfile:
    type: RuntimeDefault
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_run_as_user_root" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with user and group 0 */ -}}
{{- define "helm_lib_module_pod_security_context_run_as_user_root" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  runAsNonRoot: false
  runAsUser: 0
  runAsGroup: 0
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_pod_security_context_runtime_default" . }} */ -}}
{{- /* returns PodSecurityContext parameters for Pod with seccomp profile RuntimeDefault */ -}}
{{- define "helm_lib_module_pod_security_context_runtime_default" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  seccompProfile:
    type: RuntimeDefault
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_not_allow_privilege_escalation" . }} */ -}}
{{- /* returns SecurityContext parameters for Container with allowPrivilegeEscalation false */ -}}
{{- define "helm_lib_module_container_security_context_not_allow_privilege_escalation" -}}
securityContext:
  allowPrivilegeEscalation: false
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_read_only_root_filesystem_with_selinux" . }} */ -}}
{{- /* returns SecurityContext parameters for Container with read only root filesystem and options for SELinux compatibility*/ -}}
{{- define "helm_lib_module_container_security_context_read_only_root_filesystem_with_selinux" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  seLinuxOptions:
    level: 's0'
    type: 'spc_t'
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_read_only_root_filesystem" . }} */ -}}
{{- /* returns SecurityContext parameters for Container with read only root filesystem */ -}}
{{- define "helm_lib_module_container_security_context_read_only_root_filesystem" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_privileged" . }} */ -}}
{{- /* returns SecurityContext parameters for Container running privileged */ -}}
{{- define "helm_lib_module_container_security_context_privileged" -}}
securityContext:
  privileged: true
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_escalated_sys_admin_privileged" . }} */ -}}
{{- /* returns SecurityContext parameters for Container running privileged with escalation and sys_admin */ -}}
{{- define "helm_lib_module_container_security_context_escalated_sys_admin_privileged" -}}
securityContext:
  allowPrivilegeEscalation: true
  capabilities:
    add:
    - SYS_ADMIN
  privileged: true
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_privileged_read_only_root_filesystem" . }} */ -}}
{{- /* returns SecurityContext parameters for Container running privileged with read only root filesystem */ -}}
{{- define "helm_lib_module_container_security_context_privileged_read_only_root_filesystem" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  privileged: true
  readOnlyRootFilesystem: true
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" . }} */ -}}
{{- /* returns SecurityContext for Container with read only root filesystem and all capabilities dropped  */ -}}
{{- define "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add"  (list . (list "KILL" "SYS_PTRACE")) }} */ -}}
{{- /* returns SecurityContext parameters for Container with read only root filesystem, all dropped and some added capabilities */ -}}
{{- define "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
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
{{- define "helm_lib_module_container_security_context_capabilities_drop_all_and_add" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
{{- /* List of capabilities */ -}}
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
    add: {{ index . 1 | toJson }}
{{- end }}

{{- /* Usage: {{ include "helm_lib_module_container_security_context_capabilities_drop_all_and_run_as_user_custom" (list . 1000 1000) }} */ -}}
{{- /* returns SecurityContext parameters for Container with read only root filesystem, all dropped, and custom user ID */ -}}
{{- define "helm_lib_module_container_security_context_capabilities_drop_all_and_run_as_user_custom" -}}
{{- /* Template context with .Values, .Chart, etc */ -}}
{{- /* User id */ -}}
{{- /* Group id */ -}}
securityContext:
  runAsUser: {{ index . 1 }}
  runAsGroup: {{ index . 2 }}
  runAsNonRoot: true
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
{{- end }}
