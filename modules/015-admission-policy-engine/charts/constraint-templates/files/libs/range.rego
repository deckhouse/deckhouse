# =============================================================================
# Library: lib.range
# =============================================================================
# Numeric range helpers.
# =============================================================================
package lib.range

import data.lib.common.get_field

value_within_range(range, value) if {
  range.min <= value
  range.max >= value
}

value_within_range(range, value) if {
  not range.min
  range.max >= value
}

value_within_range(range, value) if {
  range.min <= value
  not range.max
}

is_in_any_range(value, ranges) if {
  range := ranges[_]
  value_within_range(range, value)
}

all_values_in_ranges(values, ranges) if {
  every val in values {
    is_in_any_range(val, ranges)
  }
}

check_fields_in_ranges(obj, fields, ranges, _) := {"allowed": true, "msg": ""} if {
  not range_violation(obj, fields, ranges)
}

check_fields_in_ranges(obj, fields, ranges, _) := {"allowed": false, "msg": msg} if {
  v := range_violation(obj, fields, ranges)
  msg := v.msg
}

range_violation(obj, fields, ranges) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  required := object.get(field, "required", false)
  value == null
  required
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v must be set", [name])
}

range_violation(obj, fields, ranges) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  value != null
  not is_in_any_range(value, ranges)
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v with value %v is out of allowed ranges %v", [name, value, ranges])
}
