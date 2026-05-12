# =============================================================================
# Library: lib.path
# =============================================================================
# Filesystem path helpers.
# =============================================================================
package lib.path

import data.lib.common.get_field

path_matches(prefix, path) if {
  a := path_array(prefix)
  b := path_array(path)
  prefix_matches(a, b)
}

path_array(p) := out if {
  p != "/"
  out := split(trim(p, "/"), "/")
}

path_array("/") := []

prefix_matches(a, b) if {
  count(a) <= count(b)
  not any_not_equal_upto(a, b, count(a))
}

any_not_equal_upto(a, b, n) if {
  a[i] != b[i]
  i < n
}

check_path_prefix(obj, fields, allowed_prefixes, _) := {"allowed": true, "msg": ""} if {
  not path_prefix_violation(obj, fields, allowed_prefixes)
}

check_path_prefix(obj, fields, allowed_prefixes, _) := {"allowed": false, "msg": msg} if {
  v := path_prefix_violation(obj, fields, allowed_prefixes)
  msg := v.msg
}

path_prefix_violation(obj, fields, allowed_prefixes) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  value != null
  not any_prefix_matches(allowed_prefixes, value)
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v has value %v which is not under allowed prefixes %v", [name, value, allowed_prefixes])
}

any_prefix_matches(prefixes, value) if {
  p := prefixes[_]
  path_matches(p, value)
}
