package lib.check_object_match_test

import data.lib.check_object_match

# Full match

test_object_match_allowed if {
  actual := {"level": "s0", "role": "r", "type": "t", "user": "u"}
  allowed := [{"level": "s0", "role": "r", "type": "t", "user": "u"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == true
}

# Partial match with wildcards

test_object_match_wildcard_allowed if {
  actual := {"level": "s0", "role": "r", "type": "t", "user": "u"}
  allowed := [{"level": "", "role": "r"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == true
}

# No match

test_object_match_denied if {
  actual := {"level": "s0", "role": "r", "type": "t", "user": "u"}
  allowed := [{"level": "s1"}]
  result := check_object_match.check_partial_match_in_list(actual, allowed, ["level", "role", "type", "user"], [])
  result.allowed == false
}
