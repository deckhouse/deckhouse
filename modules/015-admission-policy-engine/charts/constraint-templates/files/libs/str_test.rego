package lib.str_test

import data.lib.str

# basic helpers

test_has_prefix if {
  str.has_prefix("abc", "a")
}

test_has_suffix if {
  str.has_suffix("abc", "c")
}

test_any_prefix_matches if {
  str.any_prefix_matches(["a", "b"], "abc")
}

test_any_suffix_matches if {
  str.any_suffix_matches(["b", "c"], "abc")
}

test_contains_any if {
  str.contains_any("abc", ["z", "b"])
}

test_has_wildcard if {
  str.has_wildcard("a*")
}

# check_prefix / check_suffix / check_contains

test_check_prefix_allowed if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name"}]
  result := str.check_prefix(obj, fields, ["a"], {})
  result.allowed == true
}

test_check_prefix_denied if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name"}]
  result := str.check_prefix(obj, fields, ["x"], {})
  result.allowed == false
}

test_check_suffix_allowed if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name"}]
  result := str.check_suffix(obj, fields, ["c"], {})
  result.allowed == true
}

test_check_suffix_denied if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name"}]
  result := str.check_suffix(obj, fields, ["x"], {})
  result.allowed == false
}

test_check_contains_allowed if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name"}]
  result := str.check_contains(obj, fields, ["b"], {})
  result.allowed == true
}

test_check_contains_denied if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name"}]
  result := str.check_contains(obj, fields, ["z"], {})
  result.allowed == false
}
