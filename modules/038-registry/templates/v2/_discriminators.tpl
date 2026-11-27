{{- /*
  What this implementation is doing, as three separate questions.

  `registry.internal.v2.enabled` answers only the first of them — that this implementation
  owns the cluster rather than the previous one. It says nothing about whether the module was
  asked to manage anything, and rendering the workloads under it conflates the two: on a
  cluster where the module sits at its default `Unmanaged`, everything got deployed, and the
  publication endpoint — whose own condition is "no upstream", which `Unmanaged` satisfies
  vacuously — failed to render for want of cert-manager and took the whole module with it.

  So the questions are asked separately, each a superset of the next:

    active   the switch has happened; the previous implementation owns nothing
    manages  ... and the configuration asks this module to own the pull path
    stores   ... and so the store runs: with the cache as one, without it as a proxy for pods
    caches   ... and it asks for the in-cluster cache

  Conditions rather than booleans, because Helm templates have no booleans:

      {{- if eq (include "registry_v2_manages" .) "true" }}
*/ -}}

{{- define "registry_v2_active" -}}
{{- if ((.Values.registry.internal).v2).enabled -}}
true
{{- end -}}
{{- end -}}

{{- define "registry_v2_manages" -}}
{{- if ((.Values.registry.internal).v2).enabled -}}
  {{- /*
    Read from the resolved configuration rather than from the ModuleConfig, because that is
    what every other part of this implementation acts on — the hook has already applied the
    default and rejected what does not make sense. An absent config is not Managed: the hook
    clears it exactly when this implementation must not act.
  */ -}}
  {{- if eq (((.Values.registry.internal.v2).registryConfig).mode | default "") "Managed" -}}
true
  {{- end -}}
{{- end -}}
{{- end -}}

{{- /*
  The store runs whenever the module manages the pull path: with the cache it is the cache, without it
  a proxy that serves pods the key through the Service (templates/v2/storage/service.yaml). Same answer
  as manages today, named apart because the store's own templates mean this one.
*/ -}}
{{- define "registry_v2_stores" -}}
{{- if eq (include "registry_v2_manages" .) "true" -}}
true
{{- end -}}
{{- end -}}

{{- define "registry_v2_caches" -}}
{{- if eq (include "registry_v2_manages" .) "true" -}}
  {{- if (((.Values.registry.internal.v2).registryConfig).storage).cache -}}
true
  {{- end -}}
{{- end -}}
{{- end -}}
