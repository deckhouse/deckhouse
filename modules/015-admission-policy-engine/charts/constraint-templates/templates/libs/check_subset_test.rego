package lib.check_subset_test

import data.lib.check_subset

# Subset match

test_subset_allowed if {
  container := {"securityContext": {"capabilities": {"add": ["NET_ADMIN"]}}}
  result := check_subset.check_container_subset(
    container,
    ["securityContext", "capabilities", "add"],
    "capabilities.add",
    ["NET_ADMIN", "NET_BIND_SERVICE"],
    ["spec", "securityContext", "capabilities", "allowedValues", "add"],
    {},
    "default",
    {"case_insensitive": true}
  )
  result.allowed == true
}

# Subset violation

test_subset_denied if {
  container := {"securityContext": {"capabilities": {"add": ["SYS_ADMIN"]}}}
  result := check_subset.check_container_subset(
    container,
    ["securityContext", "capabilities", "add"],
    "capabilities.add",
    ["NET_ADMIN"],
    ["spec", "securityContext", "capabilities", "allowedValues", "add"],
    {},
    "default",
    {"case_insensitive": true}
  )
  result.allowed == false
}

# Superset match

test_superset_allowed if {
  container := {"securityContext": {"capabilities": {"drop": ["ALL", "NET_RAW"]}}}
  result := check_subset.check_container_superset(
    container,
    ["securityContext", "capabilities", "drop"],
    "capabilities.drop",
    ["ALL"],
    ["spec", "securityContext", "capabilities", "allowedValues", "drop"],
    {},
    "default",
    {"case_insensitive": true}
  )
  result.allowed == true
}

# Superset violation

test_superset_denied if {
  container := {"securityContext": {"capabilities": {"drop": ["NET_RAW"]}}}
  result := check_subset.check_container_superset(
    container,
    ["securityContext", "capabilities", "drop"],
    "capabilities.drop",
    ["ALL"],
    ["spec", "securityContext", "capabilities", "allowedValues", "drop"],
    {},
    "default",
    {"case_insensitive": true}
  )
  result.allowed == false
}
