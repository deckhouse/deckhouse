package lib.message_test

import data.lib.message

# With exception info

test_message_with_exception if {
  msg := message.build("Violation", "pod", "actual", "allowed", "exception")
  msg == "Violation: pod | actual | allowed | exception"
}

test_violation_message_without_spe_detail if {
  msg := message.violation_message("Violation", "pod-a", {
    "field": "hostPID",
    "actual": true,
    "policy_allowed": false,
    "spe_applied": false,
    "spe_allowed": false,
  })
  msg == "Violation, pod-a | hostPID: true | policy allows: false"
}

test_violation_message_with_spe_detail if {
  msg := message.violation_message("Violation", "pod-a", {
    "field": "hostPID",
    "actual": true,
    "policy_allowed": false,
    "spe_applied": true,
    "spe_allowed": true,
  })
  msg == "Violation, pod-a | hostPID: true | policy allows: false | SPE allows: true"
}

test_violation_message_with_legacy_detail_keys if {
  msg := message.violation_message("Violation", "pod-a", {
    "msg": "hostPID has value true, expected false.",
    "spe_applied": true,
    "forbidden": true,
    "policy_allows": false,
    "spe_allows": true,
  })
  msg == "Violation, pod-a | value: true | policy allows: false | SPE allows: true"
}

# Without exception info

test_message_without_exception if {
  msg := message.build("Violation", "pod", "actual", "allowed", "")
  msg == "Violation: pod | actual | allowed"
}
