package lib.range_test

import data.lib.range

# value_within_range

test_value_within_range_inside if {
  range.value_within_range({"min": 1, "max": 3}, 2)
}

test_value_within_range_boundary_min if {
  range.value_within_range({"min": 1, "max": 3}, 1)
}

test_value_within_range_boundary_max if {
  range.value_within_range({"min": 1, "max": 3}, 3)
}

test_value_within_range_outside if {
  not range.value_within_range({"min": 1, "max": 3}, 4)
}

test_value_within_range_no_min if {
  range.value_within_range({"max": 3}, 2)
}

test_value_within_range_no_max if {
  range.value_within_range({"min": 2}, 3)
}

# is_in_any_range + all_values_in_ranges

test_is_in_any_range_multiple if {
  range.is_in_any_range(5, [{"min": 1, "max": 2}, {"min": 4, "max": 6}])
}

test_all_values_in_ranges if {
  range.all_values_in_ranges([1, 2], [{"min": 1, "max": 3}])
}

# check_fields_in_ranges

test_check_fields_in_ranges_allowed if {
  obj := {"spec": {"port": 2}}
  fields := [{"path": ["spec", "port"], "name": "port"}]
  result := range.check_fields_in_ranges(obj, fields, [{"min": 1, "max": 3}], {})
  result.allowed == true
}

test_check_fields_in_ranges_denied if {
  obj := {"spec": {"port": 5}}
  fields := [{"path": ["spec", "port"], "name": "port"}]
  result := range.check_fields_in_ranges(obj, fields, [{"min": 1, "max": 3}], {})
  result.allowed == false
}

test_check_fields_in_ranges_required_missing if {
  obj := {"spec": {}}
  fields := [{"path": ["spec", "port"], "name": "port", "required": true}]
  result := range.check_fields_in_ranges(obj, fields, [{"min": 1, "max": 3}], {})
  result.allowed == false
}
