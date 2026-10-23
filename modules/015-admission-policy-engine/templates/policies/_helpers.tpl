{{/*
  The code every policy renders through: OperationPolicy, SecurityPolicy and the Pod Security
  Standards.

  A policy template turns a policy into variants with policy_variants, and renders each rule of
  each variant with policy_constraint. A variant is the policy as one Gatekeeper constraint sees
  it: a name, an enforcement action, the rules, and the internal key `d8` with what is the same
  for every constraint of the variant:
    labels      — the rendered `metadata.labels`;
    selectors   — the rendered namespace and object selectors of `spec.match`;
    scope       — `match.scope`, when not empty;
    controllers — `enabled` and `excluded`, see policy_controllers;
    onlyKeys    — optional; only a constraint built from one of these rules is rendered.
  Labels and selectors are rendered once per variant, not once per constraint.
*/}}

{{/*
  policy_controllers decides how the constraints of a policy check the pod-creating controllers:
    enabled  — whether the controllers are matched in addition to Pods;
    warn     — whether a denying policy only warns about the controllers;
    excluded — `spec.controllerValidation.excludedPolicies`, the rules checked on Pods only.

  `spec.controllerValidation.mode` of the policy decides `enabled` and `warn`: `Enforce` checks the
  controllers with the action of the policy, `Warn` only warns about them, `Disabled` checks Pods only.
  A policy without a mode follows the module parameters `podSecurityStandards.controllerValidation`
  for `enabled` and `podSecurityStandards.controllerValidationEnforcement` for `warn`.

  #### TODO: Transition period. `controllerValidationEnforcement` defaults to `Warn` while workloads are
  being brought in line with the controller-level checks. Once it is over, make `Policy` the default.

  Usage: include "policy_controllers" (list $context $policy) | fromYaml
*/}}
{{- define "policy_controllers" }}
  {{- $context := index . 0 }}
  {{- $policy := index . 1 }}
  {{- $pss := $context.Values.admissionPolicyEngine.podSecurityStandards | default dict }}
  {{- $enabled := true }}
  {{- if hasKey $pss "controllerValidation" }}
    {{- $enabled = $pss.controllerValidation }}
  {{- end }}
  {{- $warn := eq ($pss.controllerValidationEnforcement | default "Warn") "Warn" }}
  {{- $cv := $policy.spec.controllerValidation | default dict }}
  {{- with $cv.mode }}
    {{- $enabled = ne . "Disabled" }}
    {{- $warn = eq . "Warn" }}
  {{- end }}
  {{- dict "enabled" $enabled "warn" $warn "excluded" ($cv.excludedPolicies | default list) | toYaml }}
{{- end }}

{{/*
  policy_variants expands one SecurityPolicy or OperationPolicy into the variants its constraints
  are rendered from.

  A Gatekeeper constraint carries a single enforcementAction, and a constraint cannot OR two
  namespace lists, so "deny in user namespaces, follow the module constant in system ones" needs
  more than one object. A policy with enforcementAction: Deny that spans both is split into:
    - the original name, with the system namespaces excluded;
    - `d8-system-default-<name>`, for every system namespace, with the action from the
      `system_namespaces_action` constant — `warn`, so a policy written for application namespaces
      cannot block a platform component by accident.

  No label of the namespace changes any of this. A module opting its own namespace into
  enforcement raises the Pod Security Standards there and nothing else: the platform declares its
  own namespace compliant with its own standards, which says nothing about a policy a cluster
  operator wrote for application workloads. A user policy therefore never blocks in a system
  namespace unless the constant above says so, for every system namespace at once.

  The generated names carry a prefix rather than a suffix so that none can collide with another,
  and the validating webhook `reserved_policy_names.py` keeps the prefixes free by rejecting
  policies that claim them. No prefix may be a prefix of another, or two policies could derive the
  same name.

  Nothing is split that does not have to be, because every extra constraint costs an audit pass:
    - a policy that only warns or only runs in dryrun keeps one variant, its action being already
      harmless in system namespaces;
    - a denying policy that names no system namespace, that already excludes every system namespace
      it names, or whose namespace list cannot be intersected exactly keeps one variant, scoped to
      user namespaces — a system namespace has to stay out of a constraint that still denies, or a
      policy that names `*-system` would block in `kube-system`;
    - a policy that names only system namespaces drops the user-scoped variant, and the remaining
      one keeps the original name and the action of the constant.

  Usage: include "policy_variants" (list $context $policy $labels $scope) | fromYamlArray
*/}}
{{- define "policy_variants" }}
  {{- $context := index . 0 }}
  {{- $policy := index . 1 }}
  {{- $settings := dict
        "labels" (include "helm_lib_module_labels" (list $context (index . 2)) | nindent 2)
        "scope" (index . 3)
        "controllers" (include "policy_controllers" (list $context $policy) | fromYaml) }}
  {{- $name := $policy.metadata.name }}
  {{- $action := $policy.spec.enforcementAction | default "deny" | lower }}
  {{- $match := $policy.spec.match | default dict }}
  {{- $nsSelector := $match.namespaceSelector | default dict }}
  {{- $system := include "system_namespace_scope" (list ($nsSelector.matchNames | default list) ($nsSelector.excludeNames | default list)) | fromYaml }}

  {{- $variants := list }}
  {{- if ne $action "deny" }}
    {{- $variants = list (dict "name" $name "action" $action "scope" "") }}
  {{- else if or (not $system.decidable) (not $system.names) }}
    {{- $variants = list (dict "name" $name "action" $action "scope" "user") }}
  {{- else }}
    {{- if $system.userPossible }}
      {{- $variants = append $variants (dict "name" $name "action" $action "scope" "user") }}
    {{- end }}
    {{- $variants = append $variants (dict
          "name" (ternary (printf "d8-system-default-%s" $name) $name $system.userPossible)
          "action" (include "system_namespaces_action" $context | trim)
          "scope" "system") }}
  {{- end }}

  {{- $result := list }}
  {{- range $variant := $variants }}
    {{- $selectors := include "policy_selectors" (list $context $match $variant.scope $system.names) | fromYaml }}
    {{- /* An empty dict renders as `{}`, which is not a key of `spec.match`. */}}
    {{- $result = append $result (dict
          "metadata" (dict "name" $variant.name)
          "spec" (dict "enforcementAction" $variant.action "policies" $policy.spec.policies)
          "d8" (merge (dict "selectors" (ternary "" (toYaml $selectors | nindent 4) (empty $selectors))) $settings)) }}
  {{- end }}
  {{- $result | toYaml }}
{{- end }}

{{/*
  policy_selectors builds the selectors of `spec.match` from the `match` section of a policy.
  The scope narrows it to the user namespaces or to the given system namespaces; an empty scope
  keeps the policy as it was written.

  NOTE on labelSelector semantics:
  Gatekeeper's match.labelSelector evaluates against the reviewed object's own metadata.labels.
  For Pods this is the pod's labels. For controllers (Deployment, StatefulSet, etc.) this is the
  controller's TOP-LEVEL metadata.labels, NOT the pod template's metadata.labels
  (spec.template.metadata.labels). Users who write labelSelector thinking about pod labels should
  be aware that a positive selector not mirrored at the controller top level will result in no
  controller-level check at all, and an exclusion-style selector (NotIn/DoesNotExist) used to
  exempt a workload will stop exempting at the controller level.

  Usage: include "policy_selectors" (list $context $match "user"|"system"|"" $systemNames) | fromYaml
*/}}
{{- define "policy_selectors" }}
  {{- $context := index . 0 }}
  {{- $match := index . 1 }}
  {{- $scope := index . 2 }}
  {{- $nsSelector := $match.namespaceSelector | default dict }}
  {{- $namespaces := $nsSelector.matchNames | default list }}
  {{- $excluded := $nsSelector.excludeNames | default list }}
  {{- $labelSelector := deepCopy ($nsSelector.labelSelector | default dict) }}
  {{- if eq $scope "user" }}
    {{- $excluded = concat $excluded (include "system_namespaces" $context | fromYamlArray) | uniq }}
    {{- /* #### TODO: Backward compatibility, remove once every platform namespace is named `d8-*`
           or `kube-*` and the name lists alone define a system namespace.
           Excluding the system namespaces by name is not enough while a namespace the platform
           labels as its own may be called something else: it stays outside user policies, as it
           did when that label was the whole definition of a system namespace. */}}
    {{- $_ := set $labelSelector "matchExpressions" (append ($labelSelector.matchExpressions | default list)
          (dict "key" "heritage" "operator" "NotIn" "values" (list "deckhouse"))) }}
  {{- else if eq $scope "system" }}
    {{- /* A system-scoped constraint carries no label of its own. The opt-in label a module puts on
           its namespace raises the Pod Security Standards there, and a user policy is not one of
           them. */}}
    {{- $namespaces = index . 3 }}
  {{- end }}
  {{- $selectors := dict }}
  {{- with $namespaces }}
    {{- $_ := set $selectors "namespaces" . }}
  {{- end }}
  {{- with $excluded }}
    {{- $_ := set $selectors "excludedNamespaces" . }}
  {{- end }}
  {{- with $labelSelector }}
    {{- $_ := set $selectors "namespaceSelector" . }}
  {{- end }}
  {{- if hasKey $match "labelSelector" }}
    {{- $_ := set $selectors "labelSelector" $match.labelSelector }}
  {{- end }}
  {{- $selectors | toYaml }}
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
  {{- $excludeNames := index . 1 }}
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
  policy_constraint renders the Gatekeeper constraints of one rule group of a policy variant.

  Usage: include "policy_constraint" (list $context $cr $kind $keys $parameters [$options])
    $keys       — the rules of the policy the constraint is built from;
    $parameters — `spec.parameters`, omitted when empty;
    $options    — `matchKinds`: the kinds to match instead of Pods and the pod-creating controllers;
                  `perKeyExclusion`: see below.

  A constraint matches Pods, and also the controllers unless controller validation is disabled for
  the policy or one of the rules is excluded from it. Several rules of one constraint
  (`allowHostPID` and `allowHostIPC` in D8HostProcesses) are excluded together. With
  `perKeyExclusion` they are excluded one by one instead: the constraint keeps matching controllers
  and passes the excluded rules in `skipControllerChecks`, which its Rego checks on Pods only.

  A constraint carries a single enforcementAction. When a denying policy only warns about the
  controllers (`warn`, see policy_controllers), the controllers move to a second constraint,
  `d8-controllers-<name>`, and the first one matches Pods only. The prefix is reserved by
  `reserved_policy_names.py`, so the derived name cannot collide with another policy.

  RS (replica set) is intentionally not matched: it is generated by a Deployment, so a denial
  surfaces only in the Deployment status and gives none of the early feedback the controller
  validation aims for.
*/}}
{{- define "policy_constraint" }}
  {{- $cr := index . 1 }}
  {{- $kind := index . 2 }}
  {{- $keys := index . 3 }}
  {{- $parameters := index . 4 }}
  {{- $options := dict }}
  {{- if gt (len .) 5 }}
    {{- $options = index . 5 }}
  {{- end }}
  {{- $d8 := $cr.d8 }}
  {{- $selected := not $d8.onlyKeys }}
  {{- $excluded := list }}
  {{- range $key := $keys }}
    {{- if has $key ($d8.onlyKeys | default list) }}
      {{- $selected = true }}
    {{- end }}
    {{- if has $key $d8.controllers.excluded }}
      {{- $excluded = append $excluded $key }}
    {{- end }}
  {{- end }}
  {{- if $selected }}
    {{- $action := $cr.spec.enforcementAction | lower }}
    {{- if hasKey $options "matchKinds" }}
      {{- include "policy_constraint_object" (list $cr $kind $action $options.matchKinds $parameters) }}
    {{- else }}
      {{- $controllers := $d8.controllers.enabled }}
      {{- $controllerParameters := $parameters }}
      {{- if and $controllers $excluded }}
        {{- if and $options.perKeyExclusion (lt (len $excluded) (len $keys)) }}
          {{- $controllerParameters = merge (dict "skipControllerChecks" $excluded) $parameters }}
        {{- else }}
          {{- $controllers = false }}
        {{- end }}
      {{- end }}
      {{- $podKinds := list (dict "apiGroups" (list "") "kinds" (list "Pod")) }}
      {{- $controllerKinds := list
            (dict "apiGroups" (list "apps") "kinds" (list "Deployment" "StatefulSet" "DaemonSet"))
            (dict "apiGroups" (list "") "kinds" (list "ReplicationController"))
            (dict "apiGroups" (list "batch") "kinds" (list "Job" "CronJob")) }}
      {{- /* `dryrun` stays quieter than a warning, so only a denying policy is softened. */}}
      {{- $controllerAction := ternary "warn" $action (and $d8.controllers.warn (eq $action "deny")) }}
      {{- if not $controllers }}
        {{- include "policy_constraint_object" (list $cr $kind $action $podKinds $parameters) }}
      {{- else if eq $controllerAction $action }}
        {{- include "policy_constraint_object" (list $cr $kind $action (concat $podKinds $controllerKinds) $controllerParameters) }}
      {{- else }}
        {{- include "policy_constraint_object" (list $cr $kind $action $podKinds $parameters) }}
        {{- $controllerCR := dict "metadata" (dict "name" (printf "d8-controllers-%s" $cr.metadata.name)) "d8" $d8 }}
        {{- include "policy_constraint_object" (list $controllerCR $kind $controllerAction $controllerKinds $controllerParameters) }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
  policy_constraint_object renders one Gatekeeper constraint object.

  Usage: include "policy_constraint_object" (list $cr $kind $action $matchKinds $parameters)
*/}}
{{- define "policy_constraint_object" }}
  {{- $cr := index . 0 }}
  {{- $d8 := $cr.d8 }}
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: {{ index . 1 }}
metadata:
  name: {{ $cr.metadata.name }}
  {{- $d8.labels }}
spec:
  enforcementAction: {{ index . 2 }}
  match:
    kinds:
    {{- with index . 3 }}
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- with $d8.scope }}
    scope: {{ . }}
    {{- end }}
    {{- $d8.selectors }}
  {{- with index . 4 }}
  parameters:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}
