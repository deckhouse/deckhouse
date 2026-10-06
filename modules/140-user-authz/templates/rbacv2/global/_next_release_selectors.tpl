{{- /*
  The next release moves the capabilities of the modules to the labels of its role model. Its
  user-authz turns d8:manage:<subsystem>:<level>, d8:manage:all:<level> and
  d8:use:role:<level>[:kubernetes] into aliases of the same names, whose selectors take those labels.
  The modules converge one by one, and user-authz can come minutes after the others. Until it does,
  such a role would select only the labels of this scheme, and its bindings would lose the rights of
  every module that has already moved.

  user_authz_rbacv2_next_release_selectors gives a role the selectors its alias has for the new
  labels, so the role collects these capabilities as soon as a module ships them. Every selector
  requires rbac.deckhouse.io/kind: capability, which no object of this release carries, and a module
  of this repository. A module installed from a module source that already ships the new labels
  stays out, and a role of this release grants what it granted before.

  The roles already have the selectors of the alias for the labels of this scheme, so these are not
  repeated. Nor are the selectors that take the roles user-authz brings in the next release, because
  those roles come in the same release as the aliases. d8:manage:all:<level> gets no selector of its
  own. It collects the subsystem roles of its level, and these collect the capabilities.
*/ -}}
{{- define "user_authz_rbacv2_next_release_selectors" -}}
{{- $name := . -}}
{{- $parts := splitList ":" $name -}}
{{- $levelsUpTo := dict "viewer" (list "viewer") "manager" (list "viewer" "manager") -}}

{{- /* The modules of this repository whose capabilities carry a lineage of the new model, as the
  aliases select them. keepalived and local-path-provisioner joined the network and storage lineages
  from infrastructure. The aliases of networking and storage leave them out, and so do these lists.
  user-authz is on them for its own capabilities. */ -}}
{{- $lineageModules := dict
  "namespace" (list "cert-manager" "cni-cilium" "deckhouse" "multitenancy-manager" "service-with-healthchecks" "user-authn" "user-authz")
  "network" (list "cilium-hubble" "cni-cilium" "cni-flannel" "cni-simple-bridge" "istio" "kube-dns" "kube-proxy" "metallb" "network-gateway" "network-policy-engine" "node-local-dns" "openvpn" "service-with-healthchecks" "user-authz")
  "observability" (list "okmeter")
  "security" (list "admission-policy-engine" "cert-manager" "secret-copier" "user-authz")
  "storage" (list "user-authz")
-}}

{{- /* The new model merged deckhouse, infrastructure and kubernetes into cluster and moved some of
  their modules to other subsystems. Their aliases select the system capabilities of the modules
  that were in the subsystem, each by the lineage of the subsystem the module is in now, and the
  subsystem capabilities of user-authz that aggregated into it. */ -}}
{{- $formerModules := dict
  "deckhouse" (dict
    "deckhouse" "cluster"
    "multitenancy-manager" "iam")
  "infrastructure" (dict
    "chrony" "cluster"
    "cloud-provider-aws" "cluster"
    "cloud-provider-azure" "cluster"
    "cloud-provider-dvp" "cluster"
    "cloud-provider-dynamix" "cluster"
    "cloud-provider-gcp" "cluster"
    "cloud-provider-huaweicloud" "cluster"
    "cloud-provider-openstack" "cluster"
    "cloud-provider-vcd" "cluster"
    "cloud-provider-vsphere" "cluster"
    "cloud-provider-yandex" "cluster"
    "cloud-provider-zvirt" "cluster"
    "csi-vsphere" "cluster"
    "descheduler" "cluster"
    "istio" "network"
    "keepalived" "network"
    "local-path-provisioner" "storage"
    "node-manager" "cluster"
    "terraform-manager" "cluster"
    "vertical-pod-autoscaler" "cluster")
  "kubernetes" (dict
    "control-plane-manager" "cluster"
    "kube-dns" "network"
    "kube-proxy" "network")
-}}
{{- $formerCapabilities := dict
  "deckhouse" (list)
  "infrastructure" (list "cluster")
  "kubernetes" (list "cluster" "network" "security" "storage")
-}}
{{- $capabilityAction := dict "viewer" "view_resources" "manager" "manage_resources" -}}

{{- /* A selector is a dict whose "labels" are its matchLabels and whose optional "modules" are the
  values of the module label it requires. */ -}}
{{- $selectors := list -}}
{{- if eq (index $parts 1) "use" -}}
  {{- /* d8:use:role:<level> and d8:use:role:<level>:kubernetes select the namespace lineage at
    their level. The lower levels come through the chain of the kubernetes roles of this scheme. */ -}}
  {{- $selectors = append $selectors (dict "labels" (dict "rbac.deckhouse.io/kind" "capability" "rbac.deckhouse.io/aggregate-to-namespace-as" (index $parts 3)) "modules" (index $lineageModules "namespace")) -}}
{{- else -}}
  {{- $subsystem := index $parts 2 -}}
  {{- $level := index $parts 3 -}}
  {{- $lineage := dict "networking" "network" "observability" "observability" "security" "security" "storage" "storage" -}}
  {{- if hasKey $lineage $subsystem -}}
    {{- /* The aliases of networking and storage select their lineage at their level and below,
      the others at their level only. */ -}}
    {{- $levels := list $level -}}
    {{- if has $subsystem (list "networking" "storage") -}}
      {{- $levels = index $levelsUpTo $level -}}
    {{- end -}}
    {{- range $lower := $levels -}}
      {{- $selectors = append $selectors (dict "labels" (dict "rbac.deckhouse.io/kind" "capability" (printf "rbac.deckhouse.io/aggregate-to-%s-as" (index $lineage $subsystem)) $lower) "modules" (index $lineageModules (index $lineage $subsystem))) -}}
    {{- end -}}
    {{- if eq $subsystem "security" -}}
      {{- /* user-authn and user-authz moved from security to iam, and the alias of security keeps
        their system capabilities. */ -}}
      {{- range $module := list "user-authn" "user-authz" -}}
        {{- range $lower := index $levelsUpTo $level -}}
          {{- $selectors = append $selectors (dict "labels" (dict "module" $module "rbac.deckhouse.io/kind" "capability" "rbac.deckhouse.io/scope" "system" "rbac.deckhouse.io/aggregate-to-iam-as" $lower)) -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- else if hasKey $formerModules $subsystem -}}
    {{- range $module, $moduleLineage := index $formerModules $subsystem -}}
      {{- range $lower := index $levelsUpTo $level -}}
        {{- $selectors = append $selectors (dict "labels" (dict "module" $module "rbac.deckhouse.io/kind" "capability" "rbac.deckhouse.io/scope" "system" (printf "rbac.deckhouse.io/aggregate-to-%s-as" $moduleLineage) $lower)) -}}
      {{- end -}}
    {{- end -}}
    {{- range $capabilitySubsystem := index $formerCapabilities $subsystem -}}
      {{- range $lower := index $levelsUpTo $level -}}
        {{- $selectors = append $selectors (dict "labels" (dict "module" "user-authz" "rbac.deckhouse.io/kind" "capability" "rbac.deckhouse.io/capability" (printf "subsystem-capability.%s.%s" $capabilitySubsystem (index $capabilityAction $lower)))) -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- range $selector := $selectors }}
- matchLabels:
    {{- range $key, $value := $selector.labels }}
    {{ $key }}: {{ $value | quote }}
    {{- end }}
  {{- with $selector.modules }}
  matchExpressions:
    - key: module
      operator: In
      values:
        {{- range $module := . }}
        - {{ $module | quote }}
        {{- end }}
  {{- end }}
{{- end }}
{{- end -}}
