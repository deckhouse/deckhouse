{{- /*
  Which implementation owns the cluster's registry.

  The two implementations create objects under the same names — `Service/registry`, which
  every image reference in the cluster is built from, and the `registry-push` publication
  endpoint. Two definitions of one object are not merged; whichever the release renders
  last wins, and nothing about that outcome reflects intent.

  The switch to the controller-based implementation is gated on the legacy one having
  reached `Unmanaged`, and its `Unmanaged` transitions do disable both of those objects. So
  in practice the two never coincide. But that is a property of a state machine several
  files away, and these particular objects are too central to rest on it — so ownership is
  also asserted here, where the objects are written.

  Used as a condition rather than a boolean, because Helm templates have no booleans:

      {{- if eq (include "registry_legacy_owns_the_cluster" .) "true" }}
*/ -}}

{{- define "registry_legacy_owns_the_cluster" -}}
{{- if ((.Values.registry.internal).v2).enabled -}}
false
{{- else -}}
true
{{- end -}}
{{- end -}}
