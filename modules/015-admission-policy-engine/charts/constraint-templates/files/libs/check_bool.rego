# =============================================================================
# Library: lib.check_bool
# =============================================================================
# Boolean field validators with SPE support.
#
# Usage:
# - Container field: check_container_bool(container, field_path, field_name, expected, default_val, spe_path, labels, namespace)
# - Pod field: check_pod_bool(obj, field_path, field_name, expected, default_val, spe_path)
# Returns: {"allowed": bool, "msg": string, "detail": object}
# =============================================================================
package lib.check_bool

import data.lib.common.get_field
import data.lib.exception.allowed_values_or_empty
import data.lib.exception.path_value_resolved
import data.lib.exception.resolve_spe_for_container
import data.lib.exception.resolve_spe_from_labels

# Check a boolean field on a container against expected value, with SPE support
# Returns: {"allowed": bool, "msg": string, "detail": object}
check_container_bool(container, field_path, field_name, expected, default_val, spe_path, labels, namespace) := result if {
  actual := get_field(container, field_path, default_val)
  actual == expected
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_container_bool(container, field_path, field_name, expected, default_val, spe_path, labels, namespace) := result if {
  actual := get_field(container, field_path, default_val)
  actual != expected
  exception := resolve_spe_for_container(container, labels, namespace)
  allowed_values := allowed_values_or_empty(exception, spe_path)
  count(allowed_values) > 0
  allowed_value := allowed_values[0]
  allowed_value == actual
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_container_bool(container, field_path, field_name, expected, default_val, spe_path, labels, namespace) := result if {
  actual := get_field(container, field_path, default_val)
  actual != expected
  exception := resolve_spe_for_container(container, labels, namespace)
  allowed_values := allowed_values_or_empty(exception, spe_path)
  not spe_allows(allowed_values, actual)
  spe_used := path_value_resolved(exception, spe_path)
  msg := bool_violation_msg(field_name, actual, expected, spe_used, allowed_values)
  detail := bool_violation_detail(field_name, actual, expected, spe_used, allowed_values)
  result := {
    "allowed": false,
    "msg": msg,
    "detail": detail
  }
}

spe_allows(allowed_values, actual) if {
  count(allowed_values) > 0
  allowed_values[0] == actual
}

bool_violation_msg(field_name, actual, expected, false, _) := out if {
  out := sprintf("%v has value %v, expected %v. %v", [field_name, actual, expected, ""])
}

bool_violation_msg(field_name, actual, expected, true, spe_allowed) := out if {
  ctx := bool_spe_ctx(actual, expected, spe_allowed)
  out := sprintf("%v has value %v, expected %v. %v", [field_name, actual, expected, ctx])
}

# Check a boolean field on a pod spec against expected value, with SPE support
check_pod_bool(obj, field_path, field_name, expected, default_val, spe_path) := result if {
  actual := get_field(obj, field_path, default_val)
  actual == expected
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_pod_bool(obj, field_path, field_name, expected, default_val, spe_path) := result if {
  actual := get_field(obj, field_path, default_val)
  actual != expected
  labels := object.get(obj, ["metadata", "labels"], {})
  namespace := object.get(obj, ["metadata", "namespace"], "")
  exception := resolve_spe_from_labels(labels, namespace)
  spe_val := object.get(exception, spe_path, null)
  spe_val != null
  actual == spe_val
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_pod_bool(obj, field_path, field_name, expected, default_val, spe_path) := result if {
  actual := get_field(obj, field_path, default_val)
  actual != expected
  labels := object.get(obj, ["metadata", "labels"], {})
  namespace := object.get(obj, ["metadata", "namespace"], "")
  exception := resolve_spe_from_labels(labels, namespace)
  spe_val := object.get(exception, spe_path, null)
  not spe_matches(spe_val, actual)
  spe_used := path_value_resolved(exception, spe_path)
  msg := pod_bool_violation_msg(field_name, actual, expected, spe_used, spe_val)
  detail := bool_violation_detail(field_name, actual, expected, spe_used, spe_val)
  result := {
    "allowed": false,
    "msg": msg,
    "detail": detail
  }
}

spe_matches(spe_val, actual) if {
  spe_val != null
  spe_val == actual
}

pod_bool_violation_msg(field_name, actual, expected, false, _) := out if {
  out := sprintf("%v has value %v, expected %v. %v", [field_name, actual, expected, ""])
}

pod_bool_violation_msg(field_name, actual, expected, true, spe_val) := out if {
  ctx := bool_spe_ctx(actual, expected, spe_val)
  out := sprintf("%v has value %v, expected %v. %v", [field_name, actual, expected, ctx])
}

bool_spe_ctx(actual, policy_allowed, spe_allowed) := out if {
  out := sprintf("forbidden: %v; policy allows: %v; SPE allows: %v", [actual, policy_allowed, spe_allowed])
}

base_bool_violation_msg(field_name, actual, expected) := out if {
  out := sprintf("%v has value %v, expected %v.", [field_name, actual, expected])
}

bool_violation_detail(field_name, actual, expected, false, _) := detail if {
  detail := {
    "msg": base_bool_violation_msg(field_name, actual, expected),
    "spe_applied": false,
    "field": field_name,
    "actual": actual,
    "policy_allowed": expected
  }
}

bool_violation_detail(field_name, actual, expected, true, spe_allowed) := detail if {
  detail := {
    "msg": base_bool_violation_msg(field_name, actual, expected),
    "spe_applied": true,
    "field": field_name,
    "actual": actual,
    "policy_allowed": expected,
    "spe_allowed": spe_allowed,
    "forbidden": actual,
    "policy_allows": expected,
    "spe_allows": spe_allowed
  }
}

