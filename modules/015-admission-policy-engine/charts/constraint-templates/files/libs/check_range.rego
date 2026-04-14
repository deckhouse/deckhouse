# =============================================================================
# Library: lib.check_range
# =============================================================================
# Range validators with SPE support.
#
# Usage:
# - Container numeric: check_container_in_range(container, field_path, field_name, ranges, spe_path, labels, namespace)
# - Ports array: check_ports_in_ranges(ports, field_name, ranges, spe_ranges)
# Returns: {"allowed": bool, "msg": string}
# =============================================================================
package lib.check_range

import data.lib.common.get_field
import data.lib.exception.allowed_values_or_empty
import data.lib.exception.path_value_resolved
import data.lib.exception.resolve_spe_for_container
import data.lib.exception.resolve_spe_from_labels
import data.lib.range.is_in_any_range

# Check that numeric values extracted from field paths are within allowed ranges
check_container_in_range(container, field_path, field_name, ranges, spe_path, labels, namespace) := result if {
  value := get_field(container, field_path, null)
  value == null
  result := {"allowed": true, "msg": ""}
}

check_container_in_range(container, field_path, field_name, ranges, spe_path, labels, namespace) := result if {
  value := get_field(container, field_path, null)
  value != null
  is_in_any_range(value, ranges)
  result := {"allowed": true, "msg": ""}
}

check_container_in_range(container, field_path, field_name, ranges, spe_path, labels, namespace) := result if {
  value := get_field(container, field_path, null)
  value != null
  not is_in_any_range(value, ranges)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  value in spe_allowed
  result := {"allowed": true, "msg": ""}
}

check_container_in_range(container, field_path, field_name, ranges, spe_path, labels, namespace) := result if {
  value := get_field(container, field_path, null)
  value != null
  not is_in_any_range(value, ranges)
  exception := resolve_spe_for_container(container, labels, namespace)
  spe_allowed := allowed_values_or_empty(exception, spe_path)
  not value in spe_allowed
  spe_used := path_value_resolved(exception, spe_path)
  msg := range_violation_msg(field_name, value, ranges, spe_used, spe_allowed)
  result := {
    "allowed": false,
    "msg": msg
  }
}

# Check array of port values against ranges
check_ports_in_ranges(ports, field_name, ranges, spe_ranges) := result if {
  count(ports) == 0
  result := {"allowed": true, "msg": ""}
}

check_ports_in_ranges(ports, field_name, ranges, spe_ranges) := result if {
  count(ports) > 0
  every p in ports {
    is_in_any_range(p, ranges)
  }
  result := {"allowed": true, "msg": ""}
}

check_ports_in_ranges(ports, field_name, ranges, spe_ranges) := result if {
  count(ports) > 0
  bad_port := ports[_]
  not is_in_any_range(bad_port, ranges)
  not port_in_spe(bad_port, spe_ranges)
  msg := ports_range_violation_msg(field_name, bad_port, ranges, spe_ranges)
  result := {
    "allowed": false,
    "msg": msg
  }
}

range_violation_msg(field_name, value, ranges, false, _) := out if {
  out := sprintf("%v has value %v which is out of allowed ranges %v", [field_name, value, ranges])
}

range_violation_msg(field_name, value, ranges, true, spe_allowed) := out if {
  ctx := spe_range_ctx(value, ranges, spe_allowed)
  out := sprintf("%v has value %v which is out of allowed ranges %v. %v", [field_name, value, ranges, ctx])
}

ports_range_violation_msg(field_name, bad_port, ranges, spe_ranges) := out if {
  count(spe_ranges) == 0
  out := sprintf("%v: port %v is out of allowed ranges %v", [field_name, bad_port, ranges])
}

ports_range_violation_msg(field_name, bad_port, ranges, spe_ranges) := out if {
  count(spe_ranges) > 0
  ctx := spe_range_ctx(bad_port, ranges, spe_ranges)
  out := sprintf("%v: port %v is out of allowed ranges %v. %v", [field_name, bad_port, ranges, ctx])
}

spe_range_ctx(actual, policy_ranges, spe_allowed) := out if {
  out := sprintf("forbidden: %v; policy allows: %v; SPE allows: %v", [actual, policy_ranges, spe_allowed])
}

port_in_spe(port, spe_ranges) if {
  count(spe_ranges) > 0
  is_in_any_range(port, spe_ranges)
}
