# =============================================================================
# Library: lib.check_set
# =============================================================================
# Set membership validators with SPE support.
#
# Usage:
# - Container scalar: check_container_value_in_set(container, field_path, field_name, allowed_set, spe_path, labels, namespace)
# - Pod scalar: check_pod_value_in_set(obj, field_path, field_name, allowed_set, spe_path)
# - Pod array: check_pod_array_in_set(obj, field_path, field_name, allowed_set, spe_path)
# - Scalar w/ wildcards: check_value_in_set_with_wildcards(value, allowed_set, spe_allowed, opts)
# - Glob patterns: check_value_with_glob(value, allowed_set, spe_allowed)
# - Allow/Deny lists: check_allowlist_denylist(value, allowlist, denylist, spe_allowed, opts)
# Returns: {"allowed": bool, "msg": string, "detail": object}
# =============================================================================
package lib.check_set

import data.lib.common.get_field
import data.lib.exception.allowed_values_or_empty
import data.lib.exception.path_value_resolved
import data.lib.exception.resolve_spe_for_container
import data.lib.exception.resolve_spe_from_labels
import data.lib.match.glob_any

# Check that a container field value is in an allowed set, with SPE support
check_container_value_in_set(container, field_path, field_name, allowed_set, spe_path, labels, namespace) := result if {
  value := get_field(container, field_path, null)
  value == null
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_container_value_in_set(container, field_path, field_name, allowed_set, spe_path, labels, namespace) := result if {
  value := get_field(container, field_path, null)
  value != null
  value_in_set(value, allowed_set)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_container_value_in_set(container, field_path, field_name, allowed_set, spe_path, labels, namespace) := result if {
  value := get_field(container, field_path, null)
  value != null
  not value_in_set(value, allowed_set)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  value_in_set(value, spe_allowed)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_container_value_in_set(container, field_path, field_name, allowed_set, spe_path, labels, namespace) := result if {
  value := get_field(container, field_path, null)
  value != null
  not value_in_set(value, allowed_set)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  not value_in_set(value, spe_allowed)
  spe_used := path_value_resolved(exception, spe_path)
  msg := set_violation_msg(field_name, value, allowed_set, spe_used, spe_allowed)
  detail := set_violation_detail(field_name, value, allowed_set, spe_used, spe_allowed)
  result := {
    "allowed": false,
    "msg": msg,
    "detail": detail
  }
}

# Check that a pod-level field value is in an allowed set
check_pod_value_in_set(obj, field_path, field_name, allowed_set, spe_path) := result if {
  value := get_field(obj, field_path, null)
  value == null
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_pod_value_in_set(obj, field_path, field_name, allowed_set, spe_path) := result if {
  value := get_field(obj, field_path, null)
  value != null
  value_in_set(value, allowed_set)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_pod_value_in_set(obj, field_path, field_name, allowed_set, spe_path) := result if {
  value := get_field(obj, field_path, null)
  value != null
  not value_in_set(value, allowed_set)
  labels := object.get(obj, ["metadata", "labels"], {})
  namespace := object.get(obj, ["metadata", "namespace"], "")
  exception := resolve_spe_from_labels(labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  value_in_set(value, spe_allowed)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_pod_value_in_set(obj, field_path, field_name, allowed_set, spe_path) := result if {
  value := get_field(obj, field_path, null)
  value != null
  not value_in_set(value, allowed_set)
  labels := object.get(obj, ["metadata", "labels"], {})
  namespace := object.get(obj, ["metadata", "namespace"], "")
  exception := resolve_spe_from_labels(labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  not value_in_set(value, spe_allowed)
  spe_used := path_value_resolved(exception, spe_path)
  msg := set_violation_msg(field_name, value, allowed_set, spe_used, spe_allowed)
  detail := set_violation_detail(field_name, value, allowed_set, spe_used, spe_allowed)
  result := {
    "allowed": false,
    "msg": msg,
    "detail": detail
  }
}

# Check that all values in an array field are in an allowed set (with wildcard and prefix support)
check_pod_array_in_set(obj, field_path, field_name, allowed_set, spe_path) := result if {
  values := get_field(obj, field_path, [])
  count(values) == 0
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_pod_array_in_set(obj, field_path, field_name, allowed_set, spe_path) := result if {
  values := get_field(obj, field_path, [])
  count(values) > 0
  all_in_set_or_prefix(values, allowed_set)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_pod_array_in_set(obj, field_path, field_name, allowed_set, spe_path) := result if {
  values := get_field(obj, field_path, [])
  count(values) > 0
  not all_in_set_or_prefix(values, allowed_set)
  labels := object.get(obj, ["metadata", "labels"], {})
  namespace := object.get(obj, ["metadata", "namespace"], "")
  exception := resolve_spe_from_labels(labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  all_in_set_or_prefix(values, spe_allowed)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_pod_array_in_set(obj, field_path, field_name, allowed_set, spe_path) := result if {
  values := get_field(obj, field_path, [])
  count(values) > 0
  not all_in_set_or_prefix(values, allowed_set)
  labels := object.get(obj, ["metadata", "labels"], {})
  namespace := object.get(obj, ["metadata", "namespace"], "")
  exception := resolve_spe_from_labels(labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  not all_in_set_or_prefix(values, spe_allowed)
  spe_used := path_value_resolved(exception, spe_path)
  msg := array_set_violation_msg(field_name, values, allowed_set, spe_used, spe_allowed)
  detail := set_violation_detail(field_name, values, allowed_set, spe_used, spe_allowed)
  result := {
    "allowed": false,
    "msg": msg,
    "detail": detail
  }
}

# Check a scalar value in set with wildcard/prefix and optional localhost file matching
check_value_in_set_with_wildcards(value, allowed_set, spe_allowed, opts) := result if {
  value == null
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_value_in_set_with_wildcards(value, allowed_set, spe_allowed, opts) := result if {
  value != null
  value_in_set_or_prefix(value, allowed_set)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_value_in_set_with_wildcards(value, allowed_set, spe_allowed, opts) := result if {
  value != null
  not value_in_set_or_prefix(value, allowed_set)
  value_in_set_or_prefix(value, spe_allowed)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_value_in_set_with_wildcards(value, allowed_set, spe_allowed, opts) := result if {
  value != null
  not value_in_set_or_prefix(value, allowed_set)
  not value_in_set_or_prefix(value, spe_allowed)
  msg := value_set_violation_msg(value, allowed_set, spe_allowed)
  detail := {
    "field": "value",
    "actual": value,
    "policy_allowed": allowed_set,
    "spe_applied": count(spe_allowed) > 0,
    "spe_allowed": spe_allowed,
  }
  result := {
    "allowed": false,
    "msg": msg,
    "detail": detail
  }
}

# Check a scalar value against glob patterns with SPE fallback
check_value_with_glob(value, allowed_set, spe_allowed) := result if {
  value == null
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_value_with_glob(value, allowed_set, spe_allowed) := result if {
  value != null
  glob_any(allowed_set, value)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_value_with_glob(value, allowed_set, spe_allowed) := result if {
  value != null
  not glob_any(allowed_set, value)
  glob_any(spe_allowed, value)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_value_with_glob(value, allowed_set, spe_allowed) := result if {
  value != null
  not glob_any(allowed_set, value)
  not glob_any(spe_allowed, value)
  msg := value_glob_violation_msg(value, allowed_set, spe_allowed)
  detail := {
    "field": "value",
    "actual": value,
    "policy_allowed": allowed_set,
    "spe_applied": count(spe_allowed) > 0,
    "spe_allowed": spe_allowed,
  }
  result := {
    "allowed": false,
    "msg": msg,
    "detail": detail
  }
}

# Check allowlist/denylist with wildcard/prefix
check_allowlist_denylist(value, allowlist, denylist, spe_allowed, opts) := result if {
  value == null
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_allowlist_denylist(value, allowlist, denylist, spe_allowed, opts) := result if {
  value != null
  denied(value, denylist)
  not value_in_set_or_prefix(value, spe_allowed)
  msg := sprintf("Value %v is forbidden by %v", [value, denylist])
  detail := {
    "field": "value",
    "actual": value,
    "policy_allowed": {"allowlist": allowlist, "denylist": denylist},
    "spe_applied": count(spe_allowed) > 0,
    "spe_allowed": spe_allowed,
  }
  result := {"allowed": false, "msg": msg, "detail": detail}
}

check_allowlist_denylist(value, allowlist, denylist, spe_allowed, opts) := result if {
  value != null
  denied(value, denylist)
  value_in_set_or_prefix(value, spe_allowed)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_allowlist_denylist(value, allowlist, denylist, spe_allowed, opts) := result if {
  value != null
  not denied(value, denylist)
  value_in_set_or_prefix(value, allowlist)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_allowlist_denylist(value, allowlist, denylist, spe_allowed, opts) := result if {
  value != null
  not denied(value, denylist)
  not value_in_set_or_prefix(value, allowlist)
  value_in_set_or_prefix(value, spe_allowed)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_allowlist_denylist(value, allowlist, denylist, spe_allowed, opts) := result if {
  value != null
  not denied(value, denylist)
  not value_in_set_or_prefix(value, allowlist)
  not value_in_set_or_prefix(value, spe_allowed)
  msg := sprintf("Value %v is not allowed by %v", [value, allowlist])
  detail := {
    "field": "value",
    "actual": value,
    "policy_allowed": {"allowlist": allowlist, "denylist": denylist},
    "spe_applied": count(spe_allowed) > 0,
    "spe_allowed": spe_allowed,
  }
  result := {"allowed": false, "msg": msg, "detail": detail}
}

value_in_set(value, set) if {
  set[_] == value
}

value_in_set(_, set) if {
  set[_] == "*"
}

all_in_set_or_prefix(values, allowed) if {
  every v in values {
    value_in_set_or_prefix(v, allowed)
  }
}

value_in_set_or_prefix(value, allowed) if {
  value_in_set(value, allowed)
}

value_in_set_or_prefix(value, allowed) if {
  a := allowed[_]
  endswith(a, "*")
  startswith(value, trim_suffix(a, "*"))
}

value_in_set_or_prefix(value, allowed) if {
  a := allowed[_]
  endswith(a, "/*")
  startswith(value, trim_suffix(a, "*"))
}

set_violation_msg(field_name, value, allowed_set, false, _) := out if {
  out := sprintf("%v has value %v which is not in allowed set %v. %v", [field_name, value, allowed_set, ""])
}

set_violation_msg(field_name, value, allowed_set, true, spe_allowed) := out if {
  ctx := spe_set_ctx(value, allowed_set, spe_allowed)
  out := sprintf("%v has value %v which is not in allowed set %v. %v", [field_name, value, allowed_set, ctx])
}

array_set_violation_msg(field_name, values, allowed_set, false, _) := out if {
  out := sprintf("%v has values %v which are not in allowed set %v. %v", [field_name, values, allowed_set, ""])
}

array_set_violation_msg(field_name, values, allowed_set, true, spe_allowed) := out if {
  bad_values := [v | v := values[_]; not value_in_set_or_prefix(v, allowed_set); not value_in_set_or_prefix(v, spe_allowed)]
  bad_value := sort(bad_values)[0]
  ctx := spe_set_ctx(bad_value, allowed_set, spe_allowed)
  out := sprintf("%v has values %v which are not in allowed set %v. %v", [field_name, values, allowed_set, ctx])
}

spe_set_ctx(actual, policy_allowed, spe_allowed) := out if {
  out := sprintf("forbidden: %v; policy allows: %v; SPE allows: %v", [actual, policy_allowed, spe_allowed])
}

value_set_violation_msg(value, allowed_set, spe_allowed) := out if {
  count(spe_allowed) == 0
  out := sprintf("Value %v is not in allowed set %v. %v", [value, allowed_set, ""])
}

value_set_violation_msg(value, allowed_set, spe_allowed) := out if {
  count(spe_allowed) > 0
  ctx := spe_set_ctx(value, allowed_set, spe_allowed)
  out := sprintf("Value %v is not in allowed set %v. %v", [value, allowed_set, ctx])
}

value_glob_violation_msg(value, allowed_set, spe_allowed) := out if {
  count(spe_allowed) == 0
  out := sprintf("Value %v does not match allowed patterns %v. %v", [value, allowed_set, ""])
}

value_glob_violation_msg(value, allowed_set, spe_allowed) := out if {
  count(spe_allowed) > 0
  ctx := spe_set_ctx(value, allowed_set, spe_allowed)
  out := sprintf("Value %v does not match allowed patterns %v. %v", [value, allowed_set, ctx])
}

value_set_violation_msg(value, allowed_set, spe_allowed) := out if {
  count(spe_allowed) == 0
  out := sprintf("Value %v is not in allowed set %v. %v", [value, allowed_set, ""])
}

value_set_violation_msg(value, allowed_set, spe_allowed) := out if {
  count(spe_allowed) > 0
  ctx := spe_set_ctx(value, allowed_set, spe_allowed)
  out := sprintf("Value %v is not in allowed set %v. %v", [value, allowed_set, ctx])
}

value_glob_violation_msg(value, allowed_set, spe_allowed) := out if {
  count(spe_allowed) == 0
  out := sprintf("Value %v does not match allowed patterns %v. %v", [value, allowed_set, ""])
}

set_violation_detail(field, actual, policy_allowed, spe_applied, spe_allowed) := {
  "field": field,
  "actual": actual,
  "policy_allowed": policy_allowed,
  "spe_applied": spe_applied,
  "spe_allowed": spe_allowed,
}

value_glob_violation_msg(value, allowed_set, spe_allowed) := out if {
  count(spe_allowed) > 0
  ctx := spe_set_ctx(value, allowed_set, spe_allowed)
  out := sprintf("Value %v does not match allowed patterns %v. %v", [value, allowed_set, ctx])
}

denied(value, denylist) if {
  denylist[_] == "*"
}

denied(value, denylist) if {
  denylist[_] == value
}

denied(value, denylist) if {
  d := denylist[_]
  endswith(d, "*")
  startswith(value, trim_suffix(d, "*"))
}
