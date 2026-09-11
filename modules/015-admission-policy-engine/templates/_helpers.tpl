{{/*
  constraint_selector renders the namespace/label selectors from the CR's match section.
  NOTE on labelSelector semantics:
  Gatekeeper's match.labelSelector evaluates against the reviewed object's own
  metadata.labels. For Pods this is the pod's labels. For controllers (Deployment,
  StatefulSet, etc.) this is the controller's TOP-LEVEL metadata.labels, NOT the
  pod template's metadata.labels (spec.template.metadata.labels). Users who
  write labelSelector thinking about pod labels should be aware that a positive
  selector not mirrored at the controller top level will result in no controller-level
  check at all, and an exclusion-style selector (NotIn/DoesNotExist) used to exempt
  a workload will stop exempting at the controller level.

  NOTE on the namespace scope:
  scoped_policy_variants injects the internal keys `d8NamespaceScope` and `d8SystemNamespaces`
  into the copy of match it hands over. They are not part of the CR API — they select the
  namespace lists rendered here, so that one user policy can enforce in user namespaces and
  only warn in system ones.
*/}}
{{- define "constraint_selector" }}
    {{- $cr := index . 0 }}
    {{- $match := $cr.spec.match | default dict }}
    {{- $scope := $match.d8NamespaceScope | default "" }}
    {{- $nsSelector := $match.namespaceSelector | default dict }}
    {{- $namespaces := $nsSelector.matchNames | default list }}
    {{- $excluded := $nsSelector.excludeNames | default list }}

    {{- if eq $scope "user" }}
      {{- $excluded = concat $excluded (include "system_namespaces" . | fromYamlArray) | uniq }}
    {{- else if or (eq $scope "system-warn") (eq $scope "system-enforce") }}
      {{- $namespaces = $match.d8SystemNamespaces }}
    {{- end }}

    {{- if $namespaces }}
    namespaces:
      {{- $namespaces | toYaml | nindent 6 }}
    {{- end }}
    {{- if $excluded }}
    excludedNamespaces:
      {{- $excluded | toYaml | nindent 6 }}
    {{- end }}

    {{- $labelSelector := dict }}
    {{- if hasKey $nsSelector "labelSelector" }}
      {{- $labelSelector = deepCopy $nsSelector.labelSelector }}
    {{- end }}
    {{- $scopeExpressions := list }}
    {{- if eq $scope "system-warn" }}
      {{- $scopeExpressions = list
            (dict "key" "security.deckhouse.io/enable-security-policy-check" "operator" "NotIn" "values" (list "true")) }}
    {{- else if eq $scope "system-enforce" }}
      {{- $scopeExpressions = list
            (dict "key" "security.deckhouse.io/enable-security-policy-check" "operator" "In" "values" (list "true")) }}
    {{- end }}
    {{- if $scopeExpressions }}
      {{- $_ := set $labelSelector "matchExpressions" (concat ($labelSelector.matchExpressions | default list) $scopeExpressions) }}
    {{- end }}
    {{- if $labelSelector }}
    namespaceSelector:
      {{- $labelSelector | toYaml | nindent 6 }}
    {{- end }}
    {{- if hasKey $match "labelSelector" }}
    labelSelector:
      {{- $match.labelSelector | toYaml | nindent 6 }}
    {{- end }}
{{- end }}

{{/*
  system_namespaces lists the namespaces that hold the components of the platform itself.

  Gatekeeper's match.namespaces and match.excludedNamespaces take these globs as they are,
  and the webhooks recognize the same names through a CEL match condition. Keep the two in
  sync: a namespace a webhook does not route never reaches the constraints listed here.
*/}}
{{- define "system_namespaces" }}
- "d8-*"
- "kube-*"
{{- end }}

{{/*
  system_namespaces_cel recognizes the same namespaces as system_namespaces, as a CEL expression
  for a webhook matchConditions entry. Keep the two definitions in sync.

  The `has()` guard is required, not defensive. For a cluster-scoped resource the AdmissionRequest
  carries no `namespace` key at all, and reading it raises `no such key: namespace`. A matchCondition
  that errors rejects the request under `failurePolicy: Fail`, which took down every apply of a
  cluster-scoped constraint until the guard was added.

  With the guard the expression is false for a cluster-scoped resource, so the negated form used by
  the main webhook stays true and such objects keep being validated there, as they were when the
  webhook selected namespaces by label.
*/}}
{{- define "system_namespaces_cel" -}}
has(request.namespace) && (request.namespace.startsWith("d8-") || request.namespace.startsWith("kube-"))
{{- end }}

{{/*
  system_namespace_scope intersects the namespaces a policy names with the system namespaces,
  and reports whether the policy can reach a user namespace at all.

  Gatekeeper cannot AND two include lists, so the intersection has to be resolved here.
  It is exact for literal names, for prefix globs, and for a policy that names no namespace.
  A suffix glob such as `*-system` intersects `d8-*` in a set that no single Gatekeeper glob
  describes, so the result is reported as undecidable and the caller leaves the policy alone
  rather than guessing a wider or a narrower match.

  Names the policy already excludes are dropped from the result. Gatekeeper applies
  excludedNamespaces over namespaces, so keeping them would render constraints that match
  nothing while still costing an audit pass each.

  Usage: include "system_namespace_scope" (list $matchNames $excludeNames) | fromYaml
  Returns: {decidable: bool, names: [glob...], userPossible: bool}
*/}}
{{- define "system_namespace_scope" }}
  {{- $matchNames := index . 0 }}
  {{- $excludeNames := list }}
  {{- if gt (len .) 1 }}
    {{- $excludeNames = index . 1 }}
  {{- end }}
  {{- $prefixes := list "d8-" "kube-" }}
  {{- $names := list }}
  {{- $decidable := true }}
  {{- $userPossible := not $matchNames }}
  {{- if not $matchNames }}
    {{- range $prefix := $prefixes }}
      {{- $names = append $names (printf "%s*" $prefix) }}
    {{- end }}
  {{- end }}
  {{- range $entry := $matchNames }}
    {{- if eq $entry "*" }}
      {{- $userPossible = true }}
      {{- range $prefix := $prefixes }}
        {{- $names = append $names (printf "%s*" $prefix) }}
      {{- end }}
    {{- else if hasPrefix "*" $entry }}
      {{- $decidable = false }}
    {{- else if hasSuffix "*" $entry }}
      {{- $stem := trimSuffix "*" $entry }}
      {{- $covered := false }}
      {{- range $prefix := $prefixes }}
        {{- if hasPrefix $prefix $stem }}
          {{- $names = append $names $entry }}
          {{- $covered = true }}
        {{- else if hasPrefix $stem $prefix }}
          {{- $names = append $names (printf "%s*" $prefix) }}
        {{- end }}
      {{- end }}
      {{- if not $covered }}
        {{- $userPossible = true }}
      {{- end }}
    {{- else if contains "*" $entry }}
      {{/* Gatekeeper globs only anchor at one end, so anything else is not ours to interpret. */}}
      {{- $decidable = false }}
    {{- else }}
      {{- $covered := false }}
      {{- range $prefix := $prefixes }}
        {{- if hasPrefix $prefix $entry }}
          {{- $names = append $names $entry }}
          {{- $covered = true }}
        {{- end }}
      {{- end }}
      {{- if not $covered }}
        {{- $userPossible = true }}
      {{- end }}
    {{- end }}
  {{- end }}
  {{- $kept := list }}
  {{- range $name := ($names | uniq) }}
    {{- $covered := false }}
    {{- range $exclude := $excludeNames }}
      {{- if eq $exclude "*" }}
        {{- $covered = true }}
      {{- else if hasSuffix "*" $exclude }}
        {{- if hasPrefix (trimSuffix "*" $exclude) $name }}
          {{- $covered = true }}
        {{- end }}
      {{- else if hasPrefix "*" $exclude }}
        {{/* A suffix glob decides a literal name exactly, and says nothing about a glob. */}}
        {{- if and (not (contains "*" $name)) (hasSuffix (trimPrefix "*" $exclude) $name) }}
          {{- $covered = true }}
        {{- end }}
      {{- else if eq $exclude $name }}
        {{- $covered = true }}
      {{- end }}
    {{- end }}
    {{- if not $covered }}
      {{- $kept = append $kept $name }}
    {{- end }}
  {{- end }}
  {{- dict "decidable" $decidable "names" $kept "userPossible" $userPossible | toYaml }}
{{- end }}

{{/*
  scoped_policy_variants expands one SecurityPolicy or OperationPolicy into the set of CRs
  the constraint templates must render.

  A Gatekeeper constraint carries a single enforcementAction, and a constraint cannot OR two
  namespace lists, so "deny in user namespaces, warn in system ones" needs more than one
  object. A policy with enforcementAction: Deny that spans both is split into three CRs:
    - the original name, with the system namespaces excluded;
    - `d8-system-warn-<name>`, warn-only, for system namespaces that have not opted into
      policy enforcement;
    - `d8-system-enforce-<name>`, the original action, for system namespaces labeled
      `security.deckhouse.io/enable-security-policy-check: "true"`.

  The generated names carry a prefix rather than a suffix so that neither can collide with the
  other, and the validating webhook `reserved_policy_names.py` keeps both prefixes free by
  rejecting policies that claim them.

  Nothing is split that does not have to be, because every extra constraint costs an audit pass:
    - a policy that only warns or only runs in dryrun keeps one CR, its action being already
      harmless in system namespaces;
    - a policy that names no system namespace, or that already excludes every system namespace
      it names, keeps one CR, and so does a policy whose namespace list cannot be
      intersected exactly;
    - a policy that names only system namespaces gets two CRs, the enforcing one keeping the
      original name.

  Usage: include "scoped_policy_variants" (list $policy) | fromYamlArray
*/}}
{{- define "scoped_policy_variants" }}
  {{- $policy := index . 0 }}
  {{- $action := $policy.spec.enforcementAction | default "deny" | lower }}
  {{- $match := $policy.spec.match | default dict }}
  {{- $nsSelector := $match.namespaceSelector | default dict }}
  {{- $scope := include "system_namespace_scope" (list ($nsSelector.matchNames | default list) ($nsSelector.excludeNames | default list)) | fromYaml }}

  {{- if or (ne $action "deny") (not $scope.decidable) (not $scope.names) }}
    {{- list $policy | toYaml }}
  {{- else }}
    {{- $variants := list }}
    {{- if $scope.userPossible }}
      {{- $userScoped := deepCopy $policy }}
      {{- $_ := set $userScoped.spec.match "d8NamespaceScope" "user" }}
      {{- $variants = append $variants $userScoped }}
    {{- end }}

    {{- $systemWarn := deepCopy $policy }}
    {{- $_ := set $systemWarn.metadata "name" (printf "d8-system-warn-%s" $policy.metadata.name) }}
    {{- $_ := set $systemWarn.spec "enforcementAction" "warn" }}
    {{- $_ := set $systemWarn.spec.match "d8NamespaceScope" "system-warn" }}
    {{- $_ := set $systemWarn.spec.match "d8SystemNamespaces" $scope.names }}
    {{- $variants = append $variants $systemWarn }}

    {{- $systemEnforce := deepCopy $policy }}
    {{- if $scope.userPossible }}
      {{- $_ := set $systemEnforce.metadata "name" (printf "d8-system-enforce-%s" $policy.metadata.name) }}
    {{- end }}
    {{- $_ := set $systemEnforce.spec.match "d8NamespaceScope" "system-enforce" }}
    {{- $_ := set $systemEnforce.spec.match "d8SystemNamespaces" $scope.names }}
    {{- $variants = append $variants $systemEnforce }}

    {{- $variants | toYaml }}
  {{- end }}
{{- end }}

{{- define "pod_security_standard_baseline" }}
  {{- $context := index . 0 }}
  {{- $policyCRDName := index . 1 }}
  {{- $policyAction := index . 2 }}
  {{- $parameters := dict }}
  {{- if gt (len .) 3 }}
  {{- $parameters = index . 3}}
  {{- end}}

{{- include "pod_security_standard_base" (list $context "baseline" $policyCRDName $policyAction $parameters) }}
{{- end }}

{{- define "pod_security_standard_restricted" }}
  {{- $context := index . 0 }}
  {{- $policyCRDName := index . 1 }}
  {{- $policyAction := index . 2 }}
  {{- $parameters := dict }}
  {{- if gt (len .) 3 }}
  {{- $parameters = index . 3}}
  {{- end}}

{{- include "pod_security_standard_base" (list $context "restricted" $policyCRDName $policyAction $parameters) }}
{{- end }}

{{- define "pod_security_standard_base" }}
  {{- $context := index . 0 }}
  {{- $standard := index . 1 }}
  {{- $policyCRDName := index . 2 }}
  {{- $policyAction := index . 3 }}
  {{- $parameters := index . 4 }}
  {{- $defaultPolicy := ($context.Values.admissionPolicyEngine.podSecurityStandards.defaultPolicy | default "privileged" | lower) }}

{{- if $context.Values.admissionPolicyEngine.internal.bootstrapped }}
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: {{ $policyCRDName }}
metadata:
{{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
  name: d8-pod-security-{{$standard}}-{{$policyAction}}-default
{{- else }}
  name: d8-pod-security-{{$standard}}-{{$policyAction}}
{{- end }}
  {{- include "helm_lib_module_labels" (list $context (dict "security.deckhouse.io/pod-standard" $standard)) | nindent 2 }}
spec:
  enforcementAction: {{ $policyAction }}
  match:
    scope: Namespaced
    kinds:
{{- include "workload_kinds" . }}
    # System namespaces are covered by the constraints below, never by this one.
    excludedNamespaces:
      {{- include "system_namespaces" . | fromYamlArray | toYaml | nindent 6 }}
    labelSelector:
      matchExpressions:
        - key: security.deckhouse.io/skip-pss-check
          operator: NotIn
          values: ["true"]
        - key: gatekeeper.sh/operation
          operator: NotIn
          values: ["webhook"]
    namespaceSelector:
      matchExpressions:
      {{- if eq $standard "baseline" }}
        {{- if eq $defaultPolicy "privileged" }}
        - { key: security.deckhouse.io/pod-policy, operator: In, values: [ baseline, restricted ] }
        {{- else }}
        - { key: security.deckhouse.io/pod-policy, operator: NotIn, values: [ privileged ] }
        {{- end }}
      {{- else if eq $standard "restricted" }}
        {{- if eq $defaultPolicy "restricted" }}
        - { key: security.deckhouse.io/pod-policy, operator: NotIn, values: [ privileged, baseline ] }
        {{- else }}
        - { key: security.deckhouse.io/pod-policy, operator: In, values: [ restricted ] }
        {{- end }}
      {{- else}}
        {{ cat "Unknown policy standard" | fail }}
      {{- end }}
      # matches default enforcement action
      {{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
        # if there are other policy actions apart from the default one, we add all of them to NotIn list, so that the namespaces with such labels aren't subject to the default policy
        {{- if gt (len $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions) 1 }}
        - { key: security.deckhouse.io/pod-policy-action, operator: NotIn, values: [{{ (without $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions $policyAction | join ",") }}] }
        {{- end }}
      # matches another action (non-default)
      {{- else }}
        - { key: security.deckhouse.io/pod-policy-action, operator: In, values: [{{ $policyAction }}] }
      {{- end }}
  {{- if $parameters }}
  parameters:
    {{ $parameters | toYaml | nindent 4 }}
  {{- end }}
{{/*
  Pod Security Standards in warn mode for system namespaces.

  Every namespace named `d8-*` or `kube-*` is checked against this standard regardless of its
  `security.deckhouse.io/pod-policy` label and of the module's defaultPolicy, which applies to
  non-system namespaces only. The block runs for both standards, so the two together give system
  namespaces the full `restricted` set of checks. The action is always `warn`: the check exists to
  make violations visible in the audit and in Grafana, never to block a system component.

  Namespaces that opted into enforcement with `security.deckhouse.io/enable-security-policy-check`
  are excluded, because the constraint below already covers them with the configured action.
  When that constraint is not rendered for this standard, the exclusion is dropped so that opted-in
  namespaces still get the warning.

  The block does not depend on the enforcement action, so it is rendered on the iteration of the
  default action, which is always present in internal.podSecurityStandards.enforcementActions.
  That keeps it at one object per standard instead of one per action.
*/}}
{{- $d8EnforceRendered := or (and (eq $standard "baseline") (ne $defaultPolicy "privileged")) (and (eq $standard "restricted") (ne $defaultPolicy "restricted")) }}
{{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: {{ $policyCRDName }}
metadata:
  name: d8-pod-security-{{$standard}}-warn-system
  {{- include "helm_lib_module_labels" (list $context (dict "security.deckhouse.io/pod-standard" $standard)) | nindent 2 }}
spec:
  enforcementAction: warn
  match:
    scope: Namespaced
    kinds:
{{- include "workload_kinds" . }}
    namespaces:
      {{- include "system_namespaces" . | fromYamlArray | toYaml | nindent 6 }}
    labelSelector:
      matchExpressions:
        - key: security.deckhouse.io/skip-pss-check
          operator: NotIn
          values: ["true"]
        - key: gatekeeper.sh/operation
          operator: NotIn
          values: ["webhook"]
  {{- if $d8EnforceRendered }}
    namespaceSelector:
      matchExpressions:
        - { key: security.deckhouse.io/enable-security-policy-check, operator: NotIn, values: [ "true" ] }
  {{- end }}
  {{- if $parameters }}
  parameters:
    {{ $parameters | toYaml | nindent 4 }}
  {{- end }}
{{- end }}
{{/* #### TODO: Remove after full migration to securityPolicyExceptions in all modules */}}
{{- if $d8EnforceRendered }}
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: {{ $policyCRDName }}
metadata:
{{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
  name: d8-pod-security-{{$standard}}-{{$policyAction}}-d8-default
{{- else }}
  name: d8-pod-security-{{$standard}}-{{$policyAction}}-d8
{{- end }}
  {{- include "helm_lib_module_labels" (list $context (dict "security.deckhouse.io/pod-standard" $standard)) | nindent 2 }}
spec:
  enforcementAction: {{ $policyAction }}
  match:
    scope: Namespaced
    kinds:
{{- include "workload_kinds" . }}
    namespaces:
      {{- include "system_namespaces" . | fromYamlArray | toYaml | nindent 6 }}
    labelSelector:
      matchExpressions:
        - key: security.deckhouse.io/skip-pss-check
          operator: NotIn
          values: ["true"]
        - key: gatekeeper.sh/operation
          operator: NotIn
          values: ["webhook"]
    namespaceSelector:
      matchExpressions:
        - { key: security.deckhouse.io/enable-security-policy-check, operator: In, values: [ "true" ] }
      # matches default enforcement action
      {{- if eq $policyAction ($context.Values.admissionPolicyEngine.podSecurityStandards.enforcementAction | default "deny" | lower) }}
        # if there are other policy actions apart from the default one, we add all of them to NotIn list, so that the namespaces with such labels aren't subject to the default policy
        {{- if gt (len $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions) 1 }}
        - { key: security.deckhouse.io/pod-policy-action, operator: NotIn, values: [{{ (without $context.Values.admissionPolicyEngine.internal.podSecurityStandards.enforcementActions $policyAction | join ",") }}] }
        {{- end }}
      # matches another action (non-default)
      {{- else }}
        - { key: security.deckhouse.io/pod-policy-action, operator: In, values: [{{ $policyAction }}] }
      {{- end }}
  {{- if $parameters }}
  parameters:
    {{ $parameters | toYaml | nindent 4 }}
  {{- end }} 
{{- end }} 
{{/* #### end of TODO */}}
{{- end }}
{{- end }}

{{- define "trivy.provider.enabled" }}
  {{- $context := . }}
  {{- $denyEnabled := dig "operatorTrivy" "denyVulnerableImages" "enabled" false ($context.Values | merge (dict)) }}
  {{- if and ($context.Values.global.enabledModules | has "operator-trivy") $denyEnabled }}
    {{- print "true" }}
  {{- end }}
  {{- print "" }}
{{- end }}

{{/* workload_kinds outputs the standard Gatekeeper match.kinds blocks for pod-creating workloads. */}}
{{/* RS (replica set) is intentionally excluded — generated by Deployment, so a denial */}}
{{/* surfaces only in Deployment status and gives none of the early feedback the feature aims for. */}}
{{/* Usage: include "workload_kinds" . — indents with 6 spaces for match.kinds context. */}}
{{/* The first element of the passed list is the context, used to read controllerValidation. */}}
{{- define "workload_kinds" }}
  {{- $context := index . 0 }}
  {{- $controllerValidation := true }}
  {{- if hasKey $context.Values.admissionPolicyEngine "podSecurityStandards" }}
    {{- if hasKey $context.Values.admissionPolicyEngine.podSecurityStandards "controllerValidation" }}
      {{- $controllerValidation = $context.Values.admissionPolicyEngine.podSecurityStandards.controllerValidation }}
    {{- end }}
  {{- end }}
      - apiGroups: [""]
        kinds: ["Pod"]
  {{- if $controllerValidation }}
      - apiGroups: [apps]
        kinds: [Deployment, StatefulSet, DaemonSet]
      - apiGroups: [""]
        kinds: [ReplicationController]
      - apiGroups: [batch]
        kinds: [Job, CronJob]
  {{- end }}
{{- end }}
