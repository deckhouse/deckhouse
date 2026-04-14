# =============================================================================
# Library: lib.set
# =============================================================================
# Set membership helpers.
# =============================================================================
package lib.set

import data.lib.common.get_field

set_contains(list, elem) if {
  list[_] = elem
}

contains_any(list, elems) if {
  e := elems[_]
  set_contains(list, e)
}

contains_all(list, elems) if {
  every e in elems {
    set_contains(list, e)
  }
}

to_lower_set(list) := out if {
  out := {v | v := lower(list[_])}
}

is_subset(a, b) if {
  count(a - b) == 0
}

check_fields_in_list(obj, fields, allowed, _) := {"allowed": true, "msg": ""} if {
  not list_violation(obj, fields, allowed)
}

check_fields_in_list(obj, fields, allowed, _) := {"allowed": false, "msg": msg} if {
  v := list_violation(obj, fields, allowed)
  msg := v.msg
}

list_violation(obj, fields, allowed) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  required := object.get(field, "required", false)
  value == null
  required
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v must be set", [name])
}

list_violation(obj, fields, allowed) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  value != null
  not set_contains(allowed, value)
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v has value %v which is not allowed (%v)", [name, value, allowed])
}
