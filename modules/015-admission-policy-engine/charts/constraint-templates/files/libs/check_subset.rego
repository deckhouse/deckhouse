# =============================================================================
# Library: lib.check_subset
# =============================================================================
# Subset/superset validators with SPE support.
#
# Usage:
# - Subset: check_container_subset(container, field_path, field_name, allowed, spe_path, labels, namespace, opts)
# - Superset: check_container_superset(container, field_path, field_name, required, spe_path, labels, namespace, opts)
# opts: {"case_insensitive": true} (optional)
# Returns: {"allowed": bool, "msg": string}
# =============================================================================
package lib.check_subset

import data.lib.common.get_field
import data.lib.exception.allowed_values_or_empty
import data.lib.exception.resolve_spe_for_container
import data.lib.set.to_lower_set
import data.lib.set.is_subset

check_container_subset(container, field_path, field_name, allowed, spe_path, labels, namespace, opts) := result if {
  actual := get_field(container, field_path, [])
  allowed_set := normalize_set(allowed, opts)
  actual_set := normalize_set(actual, opts)
  is_subset(actual_set, allowed_set)
  result := {"allowed": true, "msg": ""}
}

check_container_subset(container, field_path, field_name, allowed, spe_path, labels, namespace, opts) := result if {
  actual := get_field(container, field_path, [])
  allowed_set := normalize_set(allowed, opts)
  actual_set := normalize_set(actual, opts)
  not is_subset(actual_set, allowed_set)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  spe_set := normalize_set(spe_allowed, opts)
  is_subset(actual_set, spe_set)
  result := {"allowed": true, "msg": ""}
}

check_container_subset(container, field_path, field_name, allowed, spe_path, labels, namespace, opts) := result if {
  actual := get_field(container, field_path, [])
  allowed_set := normalize_set(allowed, opts)
  actual_set := normalize_set(actual, opts)
  not is_subset(actual_set, allowed_set)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  spe_set := normalize_set(spe_allowed, opts)
  not is_subset(actual_set, spe_set)
  result := {
    "allowed": false,
    "msg": sprintf("%v must be subset of %v", [field_name, allowed])
  }
}

check_container_superset(container, field_path, field_name, required, spe_path, labels, namespace, opts) := result if {
  actual := get_field(container, field_path, [])
  required_set := normalize_set(required, opts)
  actual_set := normalize_set(actual, opts)
  is_subset(required_set, actual_set)
  result := {"allowed": true, "msg": ""}
}

check_container_superset(container, field_path, field_name, required, spe_path, labels, namespace, opts) := result if {
  actual := get_field(container, field_path, [])
  required_set := normalize_set(required, opts)
  actual_set := normalize_set(actual, opts)
  not is_subset(required_set, actual_set)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_required := allowed_values_or_empty(exception, spe_path)
  count(spe_required) > 0
  spe_set := normalize_set(spe_required, opts)
  is_subset(spe_set, actual_set)
  result := {"allowed": true, "msg": ""}
}

check_container_superset(container, field_path, field_name, required, spe_path, labels, namespace, opts) := result if {
  actual := get_field(container, field_path, [])
  required_set := normalize_set(required, opts)
  actual_set := normalize_set(actual, opts)
  not is_subset(required_set, actual_set)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_required := allowed_values_or_empty(exception, spe_path)
  count(spe_required) == 0
  result := {
    "allowed": false,
    "msg": sprintf("%v must contain %v", [field_name, required])
  }
}

check_container_superset(container, field_path, field_name, required, spe_path, labels, namespace, opts) := result if {
  actual := get_field(container, field_path, [])
  required_set := normalize_set(required, opts)
  actual_set := normalize_set(actual, opts)
  not is_subset(required_set, actual_set)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_required := allowed_values_or_empty(exception, spe_path)
  count(spe_required) > 0
  spe_set := normalize_set(spe_required, opts)
  not is_subset(spe_set, actual_set)
  result := {
    "allowed": false,
    "msg": sprintf("%v must contain %v", [field_name, required])
  }
}

normalize_set(list, opts) := out if {
  object.get(opts, "case_insensitive", false)
  out := to_lower_set(list)
}

normalize_set(list, opts) := out if {
  not object.get(opts, "case_insensitive", false)
  out := {v | v := list[_]}
}
