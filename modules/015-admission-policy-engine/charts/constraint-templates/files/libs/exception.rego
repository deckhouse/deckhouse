# =============================================================================
# Library: lib.exception
# =============================================================================
# SecurityPolicyException (SPE) resolution utilities.
# =============================================================================
package lib.exception

import data.lib.common.get_exception_label_from_labels

resolve_spe_from_labels(labels, namespace) := spe if {
  label := object.get(labels, "security.deckhouse.io/security-policy-exception", "")
  label != ""
  spe := data.inventory.namespace[namespace]["deckhouse.io/v1alpha1"].SecurityPolicyException[label]
} else := {} if {
  true
}

resolve_spe_for_container(container, labels, namespace) := spe if {
  label := get_exception_label_from_labels(container, labels)
  label != ""
  spe := data.inventory.namespace[namespace]["deckhouse.io/v1alpha1"].SecurityPolicyException[label]
} else := {} if {
  true
}

path_value_resolved(spe, path) := true if {
  object.get(spe, path, null) != null
} else := false if {
  true
}

allowed_values_or_empty(spe, path) := out if {
  val := object.get(spe, path, null)
  val == null
  out := []
} else := out if {
  val := object.get(spe, path, null)
  is_array(val)
  out := val
} else := out if {
  val := object.get(spe, path, null)
  not is_array(val)
  out := [val]
}
