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

# =============================================================================
# SPE label resolution for controllers (object_labels, effective_labels)
# =============================================================================

# object_labels returns Pod's own labels for Pod kind

test_object_labels_pod if {
  result := common.object_labels with input as {
    "review": {
      "object": {
        "kind": "Pod",
        "metadata": {
          "labels": {"security.deckhouse.io/security-policy-exception": "spe-pod"},
          "namespace": "default"
        }
      }
    }
  }
  result["security.deckhouse.io/security-policy-exception"] == "spe-pod"
}

# object_labels returns only pod template labels for Deployment (not top-level)

test_object_labels_deployment_template_only if {
  result := common.object_labels with input as {
    "review": {
      "object": {
        "kind": "Deployment",
        "metadata": {
          "labels": {"app": "myapp", "security.deckhouse.io/security-policy-exception": "spe-top"},
          "namespace": "default"
        },
        "spec": {
          "template": {
            "metadata": {
              "labels": {"security.deckhouse.io/security-policy-exception.container.nginx": "spe-container"}
            }
          }
        }
      }
    }
  }
  not result["app"]
  not result["security.deckhouse.io/security-policy-exception"]
  result["security.deckhouse.io/security-policy-exception.container.nginx"] == "spe-container"
}

# object_labels: pod template labels take precedence over top-level labels

test_object_labels_template_precedence if {
  result := common.object_labels with input as {
    "review": {
      "object": {
        "kind": "Deployment",
        "metadata": {
          "labels": {"security.deckhouse.io/security-policy-exception": "spe-top"},
          "namespace": "default"
        },
        "spec": {
          "template": {
            "metadata": {
              "labels": {"security.deckhouse.io/security-policy-exception": "spe-template"}
            }
          }
        }
      }
    }
  }
  result["security.deckhouse.io/security-policy-exception"] == "spe-template"
}

# object_labels returns only pod template labels for CronJob (not top-level)

test_object_labels_cronjob_template_only if {
  result := common.object_labels with input as {
    "review": {
      "object": {
        "kind": "CronJob",
        "metadata": {
          "labels": {"app": "cron"},
          "namespace": "default"
        },
        "spec": {
          "jobTemplate": {
            "spec": {
              "template": {
                "metadata": {
                  "labels": {"security.deckhouse.io/security-policy-exception": "spe-cron"}
                }
              }
            }
          }
        }
      }
    }
  }
  not result["app"]
  result["security.deckhouse.io/security-policy-exception"] == "spe-cron"
}

# object_labels returns empty map when no labels present

test_object_labels_empty if {
  result := common.object_labels with input as {
    "review": {
      "object": {
        "kind": "Pod",
        "metadata": {"namespace": "default"}
      }
    }
  }
  count(result) == 0
}

# object_namespace returns the namespace from the review object

test_object_namespace if {
  result := common.object_namespace with input as {
    "review": {
      "object": {
        "kind": "Pod",
        "metadata": {"namespace": "my-namespace"}
      }
    }
  }
  result == "my-namespace"
}

# object_namespace returns empty string when namespace not set

test_object_namespace_empty if {
  result := common.object_namespace with input as {
    "review": {
      "object": {
        "kind": "Pod",
        "metadata": {}
      }
    }
  }
  result == ""
}

# effective_labels returns only pod template labels for controllers

test_effective_labels_deployment if {
  obj := {
    "kind": "Deployment",
    "metadata": {
      "labels": {"app": "test"},
      "namespace": "default"
    },
    "spec": {
      "template": {
        "metadata": {
          "labels": {"security.deckhouse.io/security-policy-exception": "spe-dep"}
        }
      }
    }
  }
  result := common.effective_labels(obj)
  not result["app"]
  result["security.deckhouse.io/security-policy-exception"] == "spe-dep"
}

# effective_labels returns Pod's own labels (no pod template for Pod kind)

test_effective_labels_pod if {
  obj := {
    "kind": "Pod",
    "metadata": {
      "labels": {"security.deckhouse.io/security-policy-exception": "spe-pod"},
      "namespace": "default"
    }
  }
  result := common.effective_labels(obj)
  result["security.deckhouse.io/security-policy-exception"] == "spe-pod"
  count(result) == 1
}

# effective_labels: pod template labels for controllers (top-level not merged)

test_effective_labels_template_precedence if {
  obj := {
    "kind": "StatefulSet",
    "metadata": {
      "labels": {"app": "sts", "security.deckhouse.io/security-policy-exception": "spe-top"},
      "namespace": "default"
    },
    "spec": {
      "template": {
        "metadata": {
          "labels": {"security.deckhouse.io/security-policy-exception": "spe-template"}
        }
      }
    }
  }
  result := common.effective_labels(obj)
  not result["app"]
  result["security.deckhouse.io/security-policy-exception"] == "spe-template"
}

# merge_labels: override takes precedence over base

test_merge_labels_override_precedence if {
  base := {"a": "base-a", "b": "base-b"}
  override := {"b": "override-b", "c": "override-c"}
  result := common.merge_labels(base, override)
  result["a"] == "base-a"
  result["b"] == "override-b"
  result["c"] == "override-c"
}

# merge_labels: works with empty base

test_merge_labels_empty_base if {
  result := common.merge_labels({}, {"key": "val"})
  result["key"] == "val"
}

# merge_labels: works with empty override

test_merge_labels_empty_override if {
  result := common.merge_labels({"key": "val"}, {})
  result["key"] == "val"
}

# merge_labels: works with both empty

test_merge_labels_both_empty if {
  result := common.merge_labels({}, {})
  count(result) == 0
}
