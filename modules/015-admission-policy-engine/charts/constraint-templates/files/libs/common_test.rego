package lib.common_test

import data.lib.common

# input_containers_from collects all container types

test_input_containers_from_all_types if {
  obj := {
    "spec": {
      "containers": [{"name": "c1"}],
      "initContainers": [{"name": "i1"}],
      "ephemeralContainers": [{"name": "e1"}]
    }
  }
  result := common.input_containers_from(obj)
  count(result) == 3
}

# has_field checks direct field existence

test_has_field_present if {
  common.has_field({"a": 1}, "a")
}

test_has_field_absent if {
  not common.has_field({"a": 1}, "b")
}

# has_path checks nested path existence

test_has_path_present if {
  obj := {"a": {"b": 1}}
  common.has_path(obj, ["a", "b"])
}

test_has_path_absent if {
  obj := {"a": {"b": 1}}
  not common.has_path(obj, ["a", "c"])
}

# get_field returns default when missing

test_get_field_default if {
  obj := {"a": {"b": 1}}
  common.get_field(obj, ["a", "c"], 42) == 42
}

# get_exception_label_from_labels prefers container-specific label

test_get_exception_label_container_specific if {
  labels := {
    "security.deckhouse.io/security-policy-exception/app": "spe-1",
    "security.deckhouse.io/security-policy-exception": "spe-global"
  }
  container := {"name": "app"}
  common.get_exception_label_from_labels(container, labels) == "spe-1"
}

# get_exception_label_from_labels falls back to global label

test_get_exception_label_global_fallback if {
  labels := {
    "security.deckhouse.io/security-policy-exception": "spe-global"
  }
  container := {"name": "app"}
  common.get_exception_label_from_labels(container, labels) == "spe-global"
}

# get_exception_label_from_labels returns empty when missing

test_get_exception_label_missing if {
  labels := {"other": "x"}
  container := {"name": "app"}
  common.get_exception_label_from_labels(container, labels) == ""
}
