package lib.message_test

import data.lib.message

# With exception info

test_message_with_exception if {
  msg := message.build("Violation", "pod", "actual", "allowed", "exception")
  msg == "Violation: pod. | actual | allowed | exception"
}

# Without exception info

test_message_without_exception if {
  msg := message.build("Violation", "pod", "actual", "allowed", "")
  msg == "Violation: pod. | actual | allowed"
}
