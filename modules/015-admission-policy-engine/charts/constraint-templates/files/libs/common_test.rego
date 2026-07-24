package lib.common_test

import data.lib.common

# input_containers_from collects all container types from a normalized pod spec

test_input_containers_from_all_types if {
  pod_spec := {
    "containers": [{"name": "c1"}],
    "initContainers": [{"name": "i1"}],
    "ephemeralContainers": [{"name": "e1"}]
  }
  result := common.input_containers_from(pod_spec)
  count(result) == 3
}

# pod_spec resolves for Pod kind

test_pod_spec_for_pod if {
  common.pod_spec.containers[0].name == "pod-c" with input as {
    "review": {
      "object": {
        "kind": "Pod",
        "spec": {
          "containers": [{"name": "pod-c"}]
        }
      }
    }
  }
}

# pod_spec resolves for controller kinds with spec.template.spec

test_pod_spec_for_deployment if {
  common.pod_spec.containers[0].name == "dep-c" with input as {
    "review": {
      "object": {
        "kind": "Deployment",
        "spec": {
          "template": {
            "spec": {
              "containers": [{"name": "dep-c"}]
            }
          }
        }
      }
    }
  }
}

# pod_spec resolves for CronJob with spec.jobTemplate.spec.template.spec

test_pod_spec_for_cronjob if {
  common.pod_spec.containers[0].name == "cron-c" with input as {
    "review": {
      "object": {
        "kind": "CronJob",
        "spec": {
          "jobTemplate": {
            "spec": {
              "template": {
                "spec": {
                  "containers": [{"name": "cron-c"}]
                }
              }
            }
          }
        }
      }
    }
  }
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
    "security.deckhouse.io/security-policy-exception.container.app": "spe-1",
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
