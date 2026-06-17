package lib.bool_test

import data.lib.bool

# check_allowed_value

test_check_allowed_value_match if {
  obj := {"spec": {"enabled": true}}
  fields := [{"path": ["spec", "enabled"], "name": "enabled"}]
  result := bool.check_allowed_value(obj, fields, {"enabled": true}, {"enabled": false}, {})
  result.allowed == true
}

test_check_allowed_value_violation if {
  obj := {"spec": {"enabled": false}}
  fields := [{"path": ["spec", "enabled"], "name": "enabled"}]
  result := bool.check_allowed_value(obj, fields, {"enabled": true}, {"enabled": true}, {})
  result.allowed == false
}

test_check_allowed_value_default if {
  obj := {"spec": {}}
  fields := [{"path": ["spec", "enabled"], "name": "enabled"}]
  result := bool.check_allowed_value(obj, fields, {"enabled": true}, {"enabled": true}, {})
  result.allowed == true
}
