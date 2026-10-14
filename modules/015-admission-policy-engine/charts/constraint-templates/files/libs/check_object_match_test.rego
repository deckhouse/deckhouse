package lib.check_object_match_test

import data.lib.check_object_match

# Full match

test_object_match_allowed if {
  actual := {"level": "s0", "role": "r", "type": "t", "user": "u"}
  allowed := [{"level": "s0", "role": "r", "type": "t", "user": "u"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == true
}

# A field the object does not set always matches

test_object_match_unset_fields_allowed if {
  actual := {"role": "r"}
  allowed := [{"role": "r", "type": "t", "user": "u"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == true
}

test_object_match_empty_fields_allowed if {
  actual := {"role": "r", "type": "", "user": ""}
  allowed := [{"role": "r"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == true
}

# A field the object sets must equal the pattern, so an empty pattern value is not a wildcard

test_object_match_empty_pattern_value_denied if {
  actual := {"type": "spc_t"}
  allowed := [{"type": ""}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == false
}

test_object_match_field_absent_from_pattern_denied if {
  actual := {"type": "t", "user": "u"}
  allowed := [{"type": "t"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == false
}

# A field absent from or empty in the pattern matches any value when it is listed as a wildcard field

test_object_match_wildcard_field_allowed if {
  actual := {"level": "s0:c1,c2", "type": "t"}
  allowed := [{"type": "t"}, {"level": "", "type": "t"}]
  result := check_object_match.check_partial_match_in_list_with_wildcards(actual, allowed, ["level", "role", "type", "user"], ["level"], [])
  result.allowed == true
}

test_object_match_wildcard_field_set_in_pattern_denied if {
  actual := {"level": "s0", "type": "t"}
  allowed := [{"level": "s1", "type": "t"}]
  result := check_object_match.check_partial_match_in_list_with_wildcards(actual, allowed, ["level", "role", "type", "user"], ["level"], [])
  result.allowed == false
}

# SPE entries follow the same rules

test_object_match_spe_allowed if {
  actual := {"type": "spc_t"}
  allowed := [{"type": "container_t"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [{"type": "spc_t"}])
  result.allowed == true
}

test_object_match_spe_unlisted_field_denied if {
  actual := {"type": "spc_t", "user": "system_u"}
  allowed := [{"type": "container_t"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [{"type": "spc_t"}])
  result.allowed == false
}

# No match

test_object_match_denied if {
  actual := {"level": "s0", "role": "r", "type": "t", "user": "u"}
  allowed := [{"level": "s1"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == false
  result.msg == "Object {\"level\": \"s0\", \"role\": \"r\", \"type\": \"t\", \"user\": \"u\"} does not match allowed list [{\"level\": \"s1\"}]"
  result.detail.field == "object_match"
  result.detail.actual == {"level": "s0", "role": "r", "type": "t", "user": "u"}
  result.detail.policy_allowed == [{"level": "s1"}]
  result.detail.spe_applied == false
  result.detail.spe_allowed == []
}
