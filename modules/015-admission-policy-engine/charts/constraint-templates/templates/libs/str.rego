# =============================================================================
# Library: lib.str
# =============================================================================
# String helpers.
# =============================================================================
package lib.str

import data.lib.common.get_field

has_prefix(value, prefix) if {
  startswith(value, prefix)
}

has_suffix(value, suffix) if {
  endswith(value, suffix)
}

any_prefix_matches(prefixes, value) if {
  p := prefixes[_]
  has_prefix(value, p)
}

any_suffix_matches(suffixes, value) if {
  s := suffixes[_]
  has_suffix(value, s)
}

contains_any(value, substrings) if {
  s := substrings[_]
  contains(value, s)
}

has_wildcard(value) if {
  contains(value, "*")
}

check_prefix(obj, fields, prefixes, _) := {"allowed": true, "msg": ""} if {
  not prefix_violation(obj, fields, prefixes)
}

check_prefix(obj, fields, prefixes, _) := {"allowed": false, "msg": msg} if {
  v := prefix_violation(obj, fields, prefixes)
  msg := v.msg
}

prefix_violation(obj, fields, prefixes) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  value != null
  not any_prefix_matches(prefixes, value)
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v has value %v which does not match allowed prefixes %v", [name, value, prefixes])
}

check_suffix(obj, fields, suffixes, _) := {"allowed": true, "msg": ""} if {
  not suffix_violation(obj, fields, suffixes)
}

check_suffix(obj, fields, suffixes, _) := {"allowed": false, "msg": msg} if {
  v := suffix_violation(obj, fields, suffixes)
  msg := v.msg
}

suffix_violation(obj, fields, suffixes) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  value != null
  not any_suffix_matches(suffixes, value)
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v has value %v which does not match allowed suffixes %v", [name, value, suffixes])
}

check_contains(obj, fields, substrings, _) := {"allowed": true, "msg": ""} if {
  not contains_violation(obj, fields, substrings)
}

check_contains(obj, fields, substrings, _) := {"allowed": false, "msg": msg} if {
  v := contains_violation(obj, fields, substrings)
  msg := v.msg
}

contains_violation(obj, fields, substrings) := {"msg": msg} if {
  field := fields[_]
  value := get_field(obj, field.path, null)
  value != null
  not contains_any(value, substrings)
  name := object.get(field, "name", field.path)
  msg := sprintf("Field %v has value %v which does not contain required substrings %v", [name, value, substrings])
}
