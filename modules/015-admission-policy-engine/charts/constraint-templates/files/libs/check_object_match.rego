# =============================================================================
# Library: lib.check_object_match
# =============================================================================
# Multi-field partial matching with SPE support.
#
# Usage:
# - check_partial_match_in_list(actual_obj, allowed_list, match_fields, spe_allowed_list)
# match_fields is a list of keys to compare; empty string in a pattern field is a wildcard.
# Returns: {"allowed": bool, "msg": string, "detail": object}
# =============================================================================
package lib.check_object_match

import data.lib.object.partial_match

check_partial_match_in_list(actual_obj, allowed_list, match_fields, spe_allowed_list) := result if {
  allowed_list_match(actual_obj, allowed_list, match_fields)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_partial_match_in_list(actual_obj, allowed_list, match_fields, spe_allowed_list) := result if {
  not allowed_list_match(actual_obj, allowed_list, match_fields)
  spe_allowed_list_match(actual_obj, spe_allowed_list, match_fields)
  result := {"allowed": true, "msg": "", "detail": {}}
}

check_partial_match_in_list(actual_obj, allowed_list, match_fields, spe_allowed_list) := result if {
  not allowed_list_match(actual_obj, allowed_list, match_fields)
  not spe_allowed_list_match(actual_obj, spe_allowed_list, match_fields)
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

allowed_list_match(actual_obj, allowed_list, match_fields) if {
  allowed := allowed_list[_]
  partial_match_with_empty_wildcards(allowed, actual_obj, match_fields)
}

spe_allowed_list_match(actual_obj, spe_allowed_list, match_fields) if {
  allowed := spe_allowed_list[_]
  partial_match_with_empty_wildcards(allowed, actual_obj, match_fields)
}

partial_match_with_empty_wildcards(pattern, obj, fields) if {
  every f in fields {
    field_matches(pattern, obj, f)
  }
}

field_matches(pattern, obj, f) if {
  not has_field(pattern, f)
}

field_matches(pattern, obj, f) if {
  pattern[f] == ""
}

field_matches(pattern, obj, f) if {
  pattern[f] == obj[f]
}

has_field(obj, field) if {
  object.get(obj, field, {"__missing__": true}) != {"__missing__": true}
}
