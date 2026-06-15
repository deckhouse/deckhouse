package lib.match_test

import data.lib.match

# regex_any / glob_any

test_regex_any_match if {
  match.regex_any(["a.*"], "abc")
}

test_regex_any_no_match if {
  not match.regex_any(["z.*"], "abc")
}

test_glob_any_match if {
  match.glob_any(["a*"], "abc")
}

test_glob_any_no_match if {
  not match.glob_any(["z*"], "abc")
}

# check_regex / check_glob

test_check_regex_allowed if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name", "pattern": "a.*"}]
  result := match.check_regex(obj, fields, {})
  result.allowed == true
}

test_check_regex_denied if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name", "pattern": "z.*"}]
  result := match.check_regex(obj, fields, {})
  result.allowed == false
}

test_check_glob_allowed if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name", "patterns": ["a*"]}]
  result := match.check_glob(obj, fields, {})
  result.allowed == true
}

test_check_glob_denied if {
  obj := {"spec": {"name": "abc"}}
  fields := [{"path": ["spec", "name"], "name": "name", "patterns": ["z*"]}]
  result := match.check_glob(obj, fields, {})
  result.allowed == false
}
