# =============================================================================
# Library: lib.bool
# =============================================================================
# Boolean field helpers.
# =============================================================================
package lib.bool

import data.lib.common.get_field

check_allowed_value(obj, fields, allowed, defaults, _) := {"allowed": true, "msg": ""} if {
  not bool_violation(obj, fields, allowed, defaults)
}

check_allowed_value(obj, fields, allowed, defaults, _) := {"allowed": false, "msg": msg} if {
  v := bool_violation(obj, fields, allowed, defaults)
  msg := v.msg
}

bool_violation(obj, fields, allowed, defaults) := {"msg": msg} if {
  field := fields[_]
  name := object.get(field, "name", field.path)
  default_value := object.get(defaults, name, null)
  allowed_value := object.get(allowed, name, allowed)
  value := get_field(obj, field.path, default_value)
  value != allowed_value
  msg := sprintf("Field %v has value %v which is not allowed (expected %v)", [name, value, allowed_value])
}
