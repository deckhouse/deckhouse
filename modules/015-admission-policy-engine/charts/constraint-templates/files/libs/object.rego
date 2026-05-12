# =============================================================================
# Library: lib.object
# =============================================================================
# Partial object matching helpers.
# =============================================================================
package lib.object

partial_match(pattern, obj, fields) if {
  every f in fields {
    field_matches(pattern, obj, f)
  }
}

field_matches(pattern, obj, f) if {
  not pattern[f]
}

field_matches(pattern, obj, f) if {
  pattern[f] == obj[f]
}

partial_match_any(list, pattern, fields) if {
  obj := list[_]
  partial_match(pattern, obj, fields)
}
