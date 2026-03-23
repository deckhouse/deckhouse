package lib.set_test

import data.lib.set

# set_contains / contains_any / contains_all

test_set_contains_present if {
  set.set_contains(["a", "b"], "a")
}

test_set_contains_absent if {
  not set.set_contains(["a", "b"], "c")
}

test_contains_any if {
  set.contains_any(["a", "b"], ["c", "b"])
}

test_contains_all if {
  set.contains_all(["a", "b"], ["a", "b"])
}

# to_lower_set / is_subset

test_to_lower_set if {
  result := set.to_lower_set(["A", "b"])
  result == {"a", "b"}
}

test_is_subset_true if {
  set.is_subset({"a"}, {"a", "b"})
}

test_is_subset_false if {
  not set.is_subset({"c"}, {"a", "b"})
}

# check_fields_in_list

test_check_fields_in_list_allowed if {
  obj := {"spec": {"mode": "a"}}
  fields := [{"path": ["spec", "mode"], "name": "mode"}]
  result := set.check_fields_in_list(obj, fields, ["a", "b"], {})
  result.allowed == true
}

test_check_fields_in_list_denied if {
  obj := {"spec": {"mode": "c"}}
  fields := [{"path": ["spec", "mode"], "name": "mode"}]
  result := set.check_fields_in_list(obj, fields, ["a", "b"], {})
  result.allowed == false
}

test_check_fields_in_list_required_missing if {
  obj := {"spec": {}}
  fields := [{"path": ["spec", "mode"], "name": "mode", "required": true}]
  result := set.check_fields_in_list(obj, fields, ["a", "b"], {})
  result.allowed == false
}
