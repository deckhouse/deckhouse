{{- /*
  Which implementation owns the cluster's registry.

  The two implementations create objects under the same names — `Service/registry`, which
  every image reference in the cluster is built from, and the `registry-push` publication
  endpoint. Two definitions of one object are not merged; whichever the release renders
  last wins, and nothing about that outcome reflects intent.

  Every legacy template carries this condition, not only the two colliding objects. The
  legacy hook already clears the values everything of it renders from as soon as the
  current implementation is active, so in a running cluster this is the second lock on the
  same door — but it is the lock at the layer that actually produces objects, and it is
  what a template test can assert. A fresh cluster with the cache configured came up with
  a `registry-incluster-proxy` pod Pending beside a working store, and nothing between the
  values and the API said no.

  Used as a condition rather than a boolean, because Helm templates have no booleans:

      {{- if eq (include "registry_legacy_owns_the_cluster" .) "true" }}
*/ -}}

{{- /*
  Always false, by decision: of the previous implementation only `Unmanaged` is supported, and none of
  its pieces may reach a cluster or a node again.

  This used to answer the switch gate — `registry.internal.v2.enabled` — which meant the previous
  implementation was one absent secret away from owning a cluster again. Nothing else stood in the way:
  the module's own schema cannot express its configuration any more (`mode` accepts only Managed and
  Unmanaged, and there are no `direct`/`proxy` sections), so what remained was not a decision but a
  coincidence of circumstances.

  The code itself is left in place deliberately, not deleted: the templates, the hooks and the bashible
  steps stay readable, and the modes `Local` and `Direct` are still used INSIDE dhctl as the model of a
  bootstrap — `Local` in particular is how an installation from a bundle is expressed, so a gate on it
  would break air-gap. What is closed here is the one door through which the previous implementation's
  OBJECTS could appear.

  A cluster still running that implementation therefore loses those objects on the next render. That is
  the intent of the decision rather than a side effect of this line: such a cluster is expected to be
  switched over, and `Unmanaged` is what is supported for anything that is not.
*/ -}}
{{- define "registry_legacy_owns_the_cluster" -}}
false
{{- end -}}
