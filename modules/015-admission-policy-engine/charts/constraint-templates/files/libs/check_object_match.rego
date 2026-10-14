# =============================================================================
# Library: lib.check_object_match
# =============================================================================
# Multi-field partial matching with SPE support.
#
# Usage:
# - check_partial_match_in_list(actual_obj, allowed_list, match_fields, spe_allowed_list)
# - check_partial_match_in_list_with_wildcards(actual_obj, allowed_list, match_fields, wildcard_fields, spe_allowed_list)
# match_fields is a list of keys to compare. A field the object leaves unset or
# empty always matches. A field the object sets must equal the same field of the
# pattern, so a pattern does not allow a field it omits or leaves empty.
# wildcard_fields lists the fields that a pattern leaving them unset or empty
# allows with any value.
# Returns: {"allowed": bool, "msg": string, "detail": object}
# =============================================================================
package lib.check_object_match

check_partial_match_in_list(actual_obj, allowed_list, match_fields, spe_allowed_list) := check_partial_match_in_list_with_wildcards(actual_obj, allowed_list, match_fields, [], spe_allowed_list)

check_partial_match_in_list_with_wildcards(actual_obj, allowed_list, match_fields, wildcard_fields, spe_allowed_list) := result if {
  list_match(actual_obj, allowed_list, match_fields, wildcard_fields)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_partial_match_in_list_with_wildcards(actual_obj, allowed_list, match_fields, wildcard_fields, spe_allowed_list) := result if {
  not list_match(actual_obj, allowed_list, match_fields, wildcard_fields)
  list_match(actual_obj, spe_allowed_list, match_fields, wildcard_fields)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_partial_match_in_list_with_wildcards(actual_obj, allowed_list, match_fields, wildcard_fields, spe_allowed_list) := result if {
  not list_match(actual_obj, allowed_list, match_fields, wildcard_fields)
  not list_match(actual_obj, spe_allowed_list, match_fields, wildcard_fields)
  msg := sprintf("Object %v does not match allowed list %v", [actual_obj, allowed_list])
  result := {
    "allowed": false,
    "msg": msg,
    "detail": {
      "field": "object_match",
      "actual": actual_obj,
      "policy_allowed": allowed_list,
      "spe_applied": count(spe_allowed_list) > 0,
      "spe_allowed": spe_allowed_list,
    }
  }
}

list_match(actual_obj, patterns, match_fields, wildcard_fields) if {
  pattern := patterns[_]
  every f in match_fields {
    field_matches(pattern, actual_obj, f, wildcard_fields)
  }
}

field_matches(pattern, obj, f, wildcard_fields) if {
  object.get(obj, f, "") == ""
}

field_matches(pattern, obj, f, wildcard_fields) if {
  object.get(obj, f, "") != ""
  object.get(pattern, f, "") == obj[f]
}

field_matches(pattern, obj, f, wildcard_fields) if {
  f in wildcard_fields
  object.get(pattern, f, "") == ""
}
