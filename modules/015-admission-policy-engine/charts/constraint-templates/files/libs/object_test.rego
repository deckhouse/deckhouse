package lib.object_test

import data.lib.object

# partial_match / partial_match_any

test_partial_match_all_fields if {
  pattern := {"a": 1, "b": 2}
  obj := {"a": 1, "b": 2}
  object.partial_match(pattern, obj, ["a", "b"])
}

test_partial_match_missing_field if {
  pattern := {"a": 1}
  obj := {"a": 1, "b": 2}
  object.partial_match(pattern, obj, ["a", "b"])
}

test_partial_match_mismatch if {
  pattern := {"a": 1, "b": 2}
  obj := {"a": 1, "b": 3}
  not object.partial_match(pattern, obj, ["a", "b"])
}

test_partial_match_any_found if {
  list := [{"a": 1}, {"a": 2}]
  object.partial_match_any(list, {"a": 2}, ["a"])
}

test_partial_match_any_not_found if {
  list := [{"a": 1}, {"a": 2}]
  not object.partial_match_any(list, {"a": 3}, ["a"])
}
