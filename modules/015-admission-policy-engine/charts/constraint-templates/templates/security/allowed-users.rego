package d8.security_policies

import rego.v1

violation contains {"msg": msg} if {
	fields := ["runAsUser", "runAsGroup", "supplementalGroups", "fsGroup"]
	field := fields[_]
	container := input_containers[_]
	msg := get_type_violation(field, container)
}

get_type_violation(field, container) := msg if {
	field == "runAsUser"
	params := input.parameters[field]
	msg := get_user_violation(params, container)
}

get_type_violation(field, container) := msg if {
	field != "runAsUser"
	params := input.parameters[field]
	msg := get_violation(field, params, container)
}

# RunAsUser (separate due to "MustRunAsNonRoot")
get_user_violation(params, container) := msg if {
	rule := params.rule
	provided_user := get_field_value("runAsUser", container, input.review)
	not accept_users(rule, provided_user)
	msg := sprintf("Container %v is attempting to run as disallowed user %v. Allowed runAsUser: %v", [container.name, provided_user, params])
}

get_user_violation(params, container) := msg if {
	not get_field_value("runAsUser", container, input.review)
	params.rule = "MustRunAs"
	msg := sprintf("Container %v is attempting to run without a required securityContext/runAsUser", [container.name])
}

get_user_violation(params, container) := msg if {
	params.rule = "MustRunAsNonRoot"
	not get_field_value("runAsUser", container, input.review)
	not get_field_value("runAsNonRoot", container, input.review)
	msg := sprintf("Container %v is attempting to run without a required securityContext/runAsNonRoot or securityContext/runAsUser != 0", [container.name])
}

accept_users("RunAsAny", provided_user) := true

accept_users("MustRunAsNonRoot", provided_user) := res if res := provided_user != 0

accept_users("MustRunAs", provided_user) := res if {
	ranges := input.parameters.runAsUser.ranges
	res := is_in_range(provided_user, ranges)
}

# Group Options
get_violation(field, params, container) := msg if {
	rule := params.rule
	provided_value := get_field_value(field, container, input.review)
	not is_array(provided_value)
	not accept_value(rule, provided_value, params.ranges)
	msg := sprintf("Container %v is attempting to run as disallowed group %v. Allowed %v: %v", [container.name, provided_value, field, params])
}

# SupplementalGroups is array value
get_violation(field, params, container) := msg if {
	rule := params.rule
	array_value := get_field_value(field, container, input.review)
	is_array(array_value)
	provided_value := array_value[_]
	not accept_value(rule, provided_value, params.ranges)
	msg := sprintf("Container %v is attempting to run with disallowed supplementalGroups %v. Allowed %v: %v", [container.name, array_value, field, params])
}

get_violation(field, params, container) := msg if {
	not get_field_value(field, container, input.review)
	params.rule == "MustRunAs"
	msg := sprintf("Container %v is attempting to run without a required securityContext/%v. Allowed %v: %v", [container.name, field, field, params])
}

accept_value("RunAsAny", provided_value, ranges) := true

accept_value("MayRunAs", provided_value, ranges) := res if res := is_in_range(provided_value, ranges)

accept_value("MustRunAs", provided_value, ranges) := res if res := is_in_range(provided_value, ranges)

# If container level is provided, that takes precedence
get_field_value(field, container, review) := out if {
	container_value := get_seccontext_field(field, container)
	out := container_value
}

# If no container level exists, use pod level
get_field_value(field, container, review) := out if {
	not has_seccontext_field(field, container)
	review.kind.kind == "Pod"
	pod_value := get_seccontext_field(field, review.object.spec)
	out := pod_value
}

# Helper Functions
is_in_range(val, ranges) := res if {
	matching := {1 | val >= ranges[j].min; val <= ranges[j].max}
	res := count(matching) > 0
}

has_seccontext_field(field, obj) if {
	get_seccontext_field(field, obj)
}

has_seccontext_field(field, obj) if {
	get_seccontext_field(field, obj) == false
}

get_seccontext_field(field, obj) := out if {
	out = obj.securityContext[field]
}

input_containers contains c if {
	c := input.review.object.spec.containers[_]
}

input_containers contains c if {
	c := input.review.object.spec.initContainers[_]
}

input_containers contains c if {
	c := input.review.object.spec.ephemeralContainers[_]
}
