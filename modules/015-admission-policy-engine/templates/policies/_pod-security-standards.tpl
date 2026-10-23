{{/*
  The Pod Security Standards as SecurityPolicy rules.

  Both standards are written in the format of `SecurityPolicy.spec.policies`, and the module uses
  the one description twice:
    - it publishes them as the SecurityPolicy objects `d8-pod-security-baseline` and
      `d8-pod-security-restricted`, so that a user reads what a standard consists of in the terms of
      the policies they write themselves;
    - it renders the constraints of the standards from them, through security_policy_constraints,
      the same code that renders a SecurityPolicy.

  pod-security-standards.yaml renders both, once per standard.

  The published objects describe the standards and enforce nothing by themselves: the hook that
  reads SecurityPolicy objects skips them, and the namespaces a standard applies to are selected
  here, by the `security.deckhouse.io/pod-policy` label and the module settings, which no
  SecurityPolicy can express.

  Rules a SecurityPolicy denies when they are omitted are set explicitly, so that the standard
  allows what it does not restrict: Baseline sets `allowPrivilegeEscalation: true`.
*/}}

{{/* pss_baseline_policies returns the Baseline rules. Usage: include "pss_baseline_policies" $context | fromYaml */}}
{{- define "pss_baseline_policies" }}
{{- $knownRanges := list }}
{{- $pss := .Values.admissionPolicyEngine.podSecurityStandards | default dict }}
{{- if $pss.policies }}
  {{- range $known := $pss.policies.hostPorts.knownRanges }}
    {{- $knownRanges = append $knownRanges $known }}
  {{- end }}
{{- end }}
allowPrivileged: false
allowPrivilegeEscalation: true
allowHostPID: false
allowHostIPC: false
allowHostNetwork: false
allowedHostPorts: {{ $knownRanges | toJson }}
allowedAppArmor:
  - runtime/default
  - localhost/*
allowedCapabilities:
  - AUDIT_WRITE
  - CHOWN
  - DAC_OVERRIDE
  - FOWNER
  - FSETID
  - KILL
  - MKNOD
  - NET_BIND_SERVICE
  - SETFCAP
  - SETGID
  - SETPCAP
  - SETUID
  - SYS_CHROOT
allowedHostPaths: []
allowedProcMount: Default
seLinux:
  - type: ""
  - type: container_t
  - type: container_init_t
  - type: container_kvm_t
  - type: container_engine_t
allowedUnsafeSysctls:
  - kernel.shm_rmid_forced
  - net.ipv4.ip_local_port_range
  - net.ipv4.ip_unprivileged_port_start
  - net.ipv4.tcp_syncookies
  - net.ipv4.ping_group_range
  - net.ipv4.ip_local_reserved_ports
  - net.ipv4.tcp_keepalive_time
  - net.ipv4.tcp_fin_timeout
  - net.ipv4.tcp_keepalive_intvl
  - net.ipv4.tcp_keepalive_probes
seccompProfiles:
  allowedProfiles:
    - RuntimeDefault
    - Localhost
    - ""
    - undefined
  allowedLocalhostFiles:
    - "*"
{{- end }}

{{/* pss_restricted_overrides returns the rules Restricted adds to Baseline or tightens in it. */}}
{{- define "pss_restricted_overrides" }}
allowPrivilegeEscalation: false
allowedCapabilities:
  - NET_BIND_SERVICE
requiredDropCapabilities:
  - ALL
allowedVolumes:
  - configMap
  - csi
  - downwardAPI
  - emptyDir
  - ephemeral
  - persistentVolumeClaim
  - projected
  - secret
runAsUser:
  rule: MustRunAsNonRoot
seccompProfiles:
  allowedProfiles:
    - RuntimeDefault
    - Localhost
  allowedLocalhostFiles:
    - "*"
{{- end }}

{{/*
  pss_policies returns the full rule set of a standard. Restricted is Baseline with the overrides
  applied, so the published object lists every rule a Restricted namespace is checked against.

  Usage: include "pss_policies" (list $context "baseline"|"restricted") | fromYaml
*/}}
{{- define "pss_policies" }}
  {{- $context := index . 0 }}
  {{- $standard := index . 1 }}
  {{- $policies := include "pss_baseline_policies" $context | fromYaml }}
  {{- if eq $standard "restricted" }}
    {{- $policies = mergeOverwrite $policies (include "pss_restricted_overrides" . | fromYaml) }}
  {{- else if ne $standard "baseline" }}
    {{- cat "Unknown policy standard" $standard | fail }}
  {{- end }}
  {{- $policies | toYaml }}
{{- end }}

{{/*
  pss_settings returns the settings every constraint of a standard shares, in the format of the
  `d8` key of a policy variant (see policies/_helpers.tpl) without the selectors, which differ per
  variant. Controller validation follows the module settings, `controllerValidation` and
  `controllerValidationEnforcement`, the same way as for a policy that sets no mode, with
  `podSecurityStandards.controllerValidationExcludedPolicies` as its exclusions.

  A Restricted namespace is selected by the Baseline constraints too, so Restricted renders only
  the rules it overrides (`onlyKeys`).

  Usage: include "pss_settings" (list $context "baseline"|"restricted") | fromYaml
*/}}
{{- define "pss_settings" }}
  {{- $context := index . 0 }}
  {{- $standard := index . 1 }}
  {{- $pss := $context.Values.admissionPolicyEngine.podSecurityStandards | default dict }}
  {{- $policy := dict "spec" (dict "controllerValidation" (dict "excludedPolicies" ($pss.controllerValidationExcludedPolicies | default list))) }}
  {{- $settings := dict
        "labels" (include "helm_lib_module_labels" (list $context (dict "security.deckhouse.io/pod-standard" $standard)) | nindent 2)
        "scope" "Namespaced"
        "controllers" (include "policy_controllers" (list $context $policy) | fromYaml) }}
  {{- if eq $standard "restricted" }}
    {{- $_ := set $settings "onlyKeys" (include "pss_restricted_overrides" . | fromYaml | keys | sortAlpha) }}
  {{- end }}
  {{- $settings | toYaml }}
{{- end }}

{{/* pss_object_selector selects the objects every Pod Security Standards constraint applies to. */}}
{{- define "pss_object_selector" }}
matchExpressions:
  - key: security.deckhouse.io/skip-pss-check
    operator: NotIn
    values: ["true"]
  - key: gatekeeper.sh/operation
    operator: NotIn
    values: ["webhook"]
{{- end }}

{{- define "pss_default_action" -}}
{{ .Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower }}
{{- end }}

{{/*
  pss_action_expressions selects the namespaces a constraint with the given action applies to by the
  `security.deckhouse.io/pod-policy-action` label. The constraint of the module's own action takes
  every namespace that asks for no other action.

  Usage: include "pss_action_expressions" (list $context $action) | fromYamlArray
*/}}
{{- define "pss_action_expressions" }}
  {{- $context := index . 0 }}
  {{- $action := index . 1 }}
  {{- $actions := $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions | default list }}
  {{- $expressions := list }}
  {{- if ne $action (include "pss_default_action" $context) }}
    {{- $expressions = list (dict "key" "security.deckhouse.io/pod-policy-action" "operator" "In" "values" (list $action)) }}
  {{- else if gt (len $actions) 1 }}
    {{- $expressions = list (dict "key" "security.deckhouse.io/pod-policy-action" "operator" "NotIn" "values" (without $actions $action)) }}
  {{- end }}
  {{- $expressions | toYaml }}
{{- end }}

{{/*
  pss_user_namespace_selector selects the application namespaces a standard applies to: the ones
  whose `security.deckhouse.io/pod-policy` label names this standard or a stricter one, and, when
  defaultPolicy is this standard or a stricter one, the ones without the label.

  Usage: include "pss_user_namespace_selector" (list $context $standard $action) | fromYaml
*/}}
{{- define "pss_user_namespace_selector" }}
  {{- $context := index . 0 }}
  {{- $standard := index . 1 }}
  {{- $levels := list "privileged" "baseline" "restricted" }}
  {{- $index := 0 }}
  {{- range $i, $level := $levels }}
    {{- if eq $level $standard }}
      {{- $index = $i }}
    {{- end }}
  {{- end }}
  {{- $stricter := slice $levels $index }}
  {{- $defaultPolicy := $context.Values.admissionPolicyEngine.podSecurityStandards.defaultPolicy | default "baseline" | lower }}
  {{- $levelExpression := dict "key" "security.deckhouse.io/pod-policy" "operator" "In" "values" $stricter }}
  {{- if has $defaultPolicy $stricter }}
    {{- $levelExpression = dict "key" "security.deckhouse.io/pod-policy" "operator" "NotIn" "values" (slice $levels 0 $index) }}
  {{- end }}
  {{- /* #### TODO: Backward compatibility, remove once every platform namespace is named `d8-*`
         or `kube-*` and the name list alone keeps them out of the user constraints.
         A namespace the platform labels as its own is never a user namespace, whatever it is
         called. The label was the whole definition of "system" before the names took over, and
         honouring it keeps a namespace that carries it outside user policies, as it was. */}}
  {{- $expressions := list (dict "key" "heritage" "operator" "NotIn" "values" (list "deckhouse")) $levelExpression }}
  {{- $expressions = concat $expressions (include "pss_action_expressions" (list $context (index . 2)) | fromYamlArray) }}
  {{- dict "matchExpressions" $expressions | toYaml }}
{{- end }}

{{/*
  pss_constraints renders the constraints of one standard for every enforcement action in use.
  Each rule group of the standard becomes up to three constraints of its kind:

    - `d8-pod-security-<standard>-<action>[-default]` — application namespaces, selected by the
      `security.deckhouse.io/pod-policy` label and defaultPolicy. `-default` marks the module's own
      action, which takes every namespace without a `security.deckhouse.io/pod-policy-action` label.

    - `d8-pod-security-<standard>-system` — every namespace named `d8-*` or `kube-*`, selected by
      the name alone: no namespace label and no defaultPolicy changes it. The action comes from
      the `system_namespaces_action` constant and is `warn`, which reports violations without
      blocking a system component. Neither is a module setting: how the platform treats its own
      namespaces is decided in the module, not by a cluster operator. The constraint does not
      depend on the module's action, so it is rendered with the default action only, one object
      per standard instead of one per action. The two standards together give system namespaces
      the full Restricted set of checks.
      A namespace that opted into enforcement is checked by this constraint and by the `-d8` one
      below at once: the warning keeps every system namespace in the same report, and the
      enforcing constraint decides whether the workload is admitted.

    - `d8-pod-security-<standard>-<action>-d8[-default]` — system namespaces that opted into
      enforcement with `security.deckhouse.io/enable-security-policy-check: "true"`, at the
      configured action. Rendered for both standards and for every defaultPolicy, which applies to
      non-system namespaces only: gating it on defaultPolicy, as it once was, disabled a standard
      exactly where the namespace had asked for it. The workloads this reaches carry a
      SecurityPolicyException wherever they need one.
      #### TODO: Remove after full migration to securityPolicyExceptions in all modules.
      It is selected by the two labels alone, without the name list the system constraint uses:
      both are written by the module that owns the namespace, so a namespace that opted in keeps
      being enforced even if it is not called `d8-*` or `kube-*`. Drop the `heritage` expression
      and match by name once every platform namespace is named `d8-*` or `kube-*`.

  Usage: include "pss_constraints" (list $context $standard $policies $settings)
*/}}
{{- define "pss_constraints" }}
  {{- $context := index . 0 }}
  {{- $standard := index . 1 }}
  {{- $policies := index . 2 }}
  {{- $settings := index . 3 }}
  {{- $defaultAction := include "pss_default_action" $context }}
  {{- $objectSelector := include "pss_object_selector" . | fromYaml }}
  {{- $systemNamespaces := include "system_namespaces" $context | fromYamlArray }}

  {{- range $action := $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions }}
    {{- $suffix := ternary "-default" "" (eq $action $defaultAction) }}
    {{- $variants := list
          (dict
            "name" (printf "d8-pod-security-%s-%s%s" $standard $action $suffix)
            "action" $action
            "selectors" (dict
              "excludedNamespaces" $systemNamespaces
              "labelSelector" $objectSelector
              "namespaceSelector" (include "pss_user_namespace_selector" (list $context $standard $action) | fromYaml)))
          (dict
            "name" (printf "d8-pod-security-%s-%s-d8%s" $standard $action $suffix)
            "action" $action
            "selectors" (dict
              "labelSelector" $objectSelector
              "namespaceSelector" (dict "matchExpressions" (concat
                (list
                  (dict "key" "heritage" "operator" "In" "values" (list "deckhouse"))
                  (dict "key" "security.deckhouse.io/enable-security-policy-check" "operator" "In" "values" (list "true")))
                (include "pss_action_expressions" (list $context $action) | fromYamlArray))))) }}
    {{- if eq $action $defaultAction }}
      {{- $variants = append $variants (dict
            "name" (printf "d8-pod-security-%s-system" $standard)
            "action" (include "system_namespaces_action" $context | trim)
            "selectors" (dict
              "namespaces" $systemNamespaces
              "labelSelector" $objectSelector)) }}
    {{- end }}

    {{- range $variant := $variants }}
      {{- include "security_policy_constraints" (list $context (dict
            "metadata" (dict "name" $variant.name)
            "spec" (dict "enforcementAction" $variant.action "policies" $policies)
            "d8" (merge (dict "selectors" (toYaml $variant.selectors | nindent 4)) $settings))) }}
    {{- end }}
  {{- end }}
{{- end }}
