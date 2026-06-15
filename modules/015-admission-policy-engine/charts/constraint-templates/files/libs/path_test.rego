package lib.path_test

import data.lib.path

# path_matches / path_array

test_path_matches_exact if {
  path.path_matches("/var", "/var")
}

test_path_matches_prefix if {
  path.path_matches("/var", "/var/lib")
}

test_path_matches_no_match if {
  not path.path_matches("/opt", "/var/lib")
}

test_path_array_root if {
  path.path_array("/") == []
}

test_path_array_nested if {
  path.path_array("/var/lib") == ["var", "lib"]
}

# check_path_prefix

test_check_path_prefix_allowed if {
  obj := {"spec": {"path": "/var/lib"}}
  fields := [{"path": ["spec", "path"], "name": "path"}]
  result := path.check_path_prefix(obj, fields, ["/var"], {})
  result.allowed == true
}

test_check_path_prefix_denied if {
  obj := {"spec": {"path": "/opt"}}
  fields := [{"path": ["spec", "path"], "name": "path"}]
  result := path.check_path_prefix(obj, fields, ["/var"], {})
  result.allowed == false
}
