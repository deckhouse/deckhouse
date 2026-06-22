{{- /*
  registry.mode — returns "new" when the phase is New or CleanupPending; "legacy"
  otherwise (Legacy and TakingOver both return "legacy" here).
*/ -}}
{{- define "registry.mode" -}}
{{- $phase := .Values.registry.internal.takeover.phase | default "Legacy" -}}
{{- if or (eq $phase "New") (eq $phase "CleanupPending") -}}
new
{{- else -}}
legacy
{{- end -}}
{{- end -}}

{{- /*
  registry.isLegacy — non-empty ("true") when the mode is legacy (Legacy or
  TakingOver). Used by old templates that predate the renderNew split.
*/ -}}
{{- define "registry.isLegacy" -}}
{{- if eq (include "registry.mode" .) "legacy" -}}
true
{{- end -}}
{{- end -}}

{{- /*
  registry.renderNew — non-empty ("true") when the NEW-arch workloads must
  render: phase TakingOver, New, or CleanupPending (i.e. not Legacy). During
  TakingOver the new stack runs alongside the old so the verify-gate can prove it
  ready before the flip to New. Distinct from registry.isLegacy, which stays true
  through TakingOver for the OLD templates.
*/ -}}
{{- define "registry.renderNew" -}}
{{- $phase := .Values.registry.internal.takeover.phase | default "Legacy" -}}
{{- if ne $phase "Legacy" -}}
true
{{- end -}}
{{- end -}}

{{- /*
  registry.isPureLegacy — non-empty ("true") only in the Legacy phase (NOT
  TakingOver). Used to stop the legacy bashible config and node-level workloads
  during TakingOver so the agent failover seed (same secret name) wins without a
  collision, and the agent can bind the registry port without contention.
*/ -}}
{{- define "registry.isPureLegacy" -}}
{{- $phase := .Values.registry.internal.takeover.phase | default "Legacy" -}}
{{- if eq $phase "Legacy" -}}
true
{{- end -}}
{{- end -}}
