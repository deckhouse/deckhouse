# =============================================================================
# Library: lib.match
# =============================================================================
# Regex and glob helpers.
# =============================================================================
package lib.match

import data.lib.common.get_field

regex_any(patterns, value) if {
  p := patterns[_]
  regex.match(p, value)
}

glob_any(patterns, value) if {
  p := patterns[_]
  glob.match(p, [], value)
}

check_regex(obj, fields, _) := {"allowed": true, "msg": ""} if {
  not regex_violation(obj, fields)
}

check_regex(obj, fields, _) := {"allowed": false, "msg": msg} if {
  v := regex_violation(obj, fields)
  msg := v.msg
}

regex_violation(obj, fields) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  pattern := object.get(field, "pattern", "")
  pattern != ""
  not regex.match(pattern, value)
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v with value %v does not match regex %v", [name, value, pattern])
}

check_glob(obj, fields, _) := {"allowed": true, "msg": ""} if {
  not glob_violation(obj, fields)
}

check_glob(obj, fields, _) := {"allowed": false, "msg": msg} if {
  v := glob_violation(obj, fields)
  msg := v.msg
}

glob_violation(obj, fields) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  patterns := object.get(field, "patterns", [])
  count(patterns) > 0
  not glob_any(patterns, value)
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v with value %v does not match allowed patterns %v", [name, value, patterns])
}
