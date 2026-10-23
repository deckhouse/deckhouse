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
  system_namespaces_action is what happens to a violation in any system namespace, whether or not
  a module opted it into enforcement. An opted-in namespace is also checked by the enforcing
  Pod Security Standards constraints at the module's action, so raising this constant changes
  the outcome there as well.

  It is a constant rather than a module setting on purpose. How the platform treats its own
  namespaces is not a cluster operator's decision: raising it to `deny` blocks the converge of
  every module whose workloads do not yet comply, and lowering it would hide violations the
  platform wants to see. Change it here, together with the exclusions below, and ship the two as
  one release.

  #### TODO: Before raising the constant to `deny`, check the following, since the constant then
  overrides whatever action a system namespace asks for:
    - Which system namespaces set `security.deckhouse.io/pod-policy-action` to a weaker action,
      and whether their workloads comply or carry a SecurityPolicyException. The label stops
      weakening the action in `d8-*` and `kube-*` namespaces.
    - Whether the `-<action>-d8[-default]` constraints of the Pod Security Standards are still
      needed for namespaces named `d8-*` or `kube-*`, where `-system` would decide alone, and
      whether opted-in namespaces get two identical denials.
    - Whether `PodSecurityStandardsViolation` should fire on violations in system namespaces: the
      audit reports the `-system` constraints with `violation_enforcement="deny"`.
    - The `failurePolicy` and the timeout of the system-namespace webhooks, which rest on these
      namespaces being warn-only (see validatingwebhookconfiguration.yaml).
*/}}
{{- define "system_namespaces_action" -}}
warn
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
  webhook_delete_dependencies outputs the nelm annotations that hold back the deletion of a resource
  the webhooks depend on until both webhook configurations of the module are gone.
  Disabling the module uninstalls its release, and nelm deletes the resources in parallel. If the
  Gatekeeper pods leave first, the webhooks with `failurePolicy: Fail` reject every request they
  intercept, the deletion of the constraints and roles among them, and the uninstall never finishes.
  The bindings of the Gatekeeper service account are held back too: without them Gatekeeper cannot
  read an object missing from its cache, and a webhook with `failurePolicy: Fail` rejects the request.
  external=false keeps a dependency on a webhook configuration the release does not contain ignored:
  with the default external=auto, nelm treats it as an external one and fails the release.
  Usage: include "webhook_delete_dependencies" . — indents with 4 spaces for metadata.annotations.
*/}}
{{- define "webhook_delete_dependencies" }}
    werf.io/delete-dependency-validating-webhook: state=absent,kind=ValidatingWebhookConfiguration,name=d8-admission-policy-engine-config,external=false
    werf.io/delete-dependency-mutating-webhook: state=absent,kind=MutatingWebhookConfiguration,name=d8-admission-policy-engine-config,external=false
{{- end }}

{{- /*
  ape.only_ready_constraints prints the YAML documents of a rendered stream, dropping every Gatekeeper
  constraint whose kind is not in internal.readyConstraintKinds.

  Gatekeeper creates the CRD of a constraint kind from its ConstraintTemplate, which is part of the same
  release, and a constraint of a kind without an established CRD fails the whole release. The filter lets the
  constraints of a template that is not ready yet wait for it, while every other constraint and both
  webhook configurations stay. Without the value, as in a render without the bootstrap hook (tests,
  linters), nothing is dropped.

  Usage: include "ape.only_ready_constraints" (list . (include "<RENDERING_TEMPLATE>" .))
*/ -}}
{{- define "ape.only_ready_constraints" }}
  {{- $context := index . 0 }}
  {{- $rendered := index . 1 }}
  {{- $internal := $context.Values.admissionPolicyEngine.internal }}
  {{- $filter := hasKey $internal "readyConstraintKinds" }}
  {{- $ready := $internal.readyConstraintKinds | default list }}
  {{- range $doc := splitList "\n---" $rendered }}
    {{- if trim $doc }}
      {{- $isConstraint := regexMatch "(?m)^apiVersion: *constraints\\.gatekeeper\\.sh/" $doc }}
      {{- $kind := regexFind "(?m)^kind: *[A-Za-z0-9]+" $doc | trimPrefix "kind:" | trim }}
      {{- if or (not $filter) (not $isConstraint) (has $kind $ready) }}
{{ printf "---%s" $doc }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
