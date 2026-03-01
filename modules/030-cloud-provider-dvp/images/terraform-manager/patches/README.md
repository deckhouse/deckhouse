# 001-go-mod.patch
cve fixes

# 002-add-config-data-base64.patch
Add argument config_data_base64 to the provider configuration
Add RestMapper provider to provider metadata

# 003-add-resource-ready-resource.patch
Add `kubernetes_resource_ready_v1` resource for checking another resource ready.
kubernetes provider has `wait` block, but we have bad situation.
Provider creates resource (resource now present in cluster) but if resource not 
ready with wait block, provider returns error and terraform does not save resource
in state. Now, we have situation when we cannot revert or 
in some cases recreate resource automatically and client should use manual actions
for reverts and restarts, especially in commander.
Also, this patch contains huge testing for new resource. For testing, we can use
`./run_resource_ready_tests.sh` because: 
- unfortunately  parallel tests cannot work with panic in testing framework internal. 
  It is uncomfortable with running tests in IDE's
- script contains some initialization for run tests with `kind` cluster.

`kubernetes_resource_ready_v1` resource provided next attributes:

- `api_version` - apiVersion of depended on resource. Required, should not empty.
  Provider tests try to parse apiVersion on validation and if it cannot parse
  (for example, passed `v1/v2/v3` string) will fail on validation.
  Please use reference to another resource like `kubernetes_manifest.additional_disk.object.apiVersion`
  to recreate resource on recreate depended on resource for start readiness check
  on new resource.
  Warning! Change apiVersion version, for example
  `virtualization.deckhouse.io/v1alpha2` -> `virtualization.deckhouse.io/v1`
  will recreate resource and start readiness check. This case is huge for 
  handling in provider, and we skip this case for simplify code and developer 
  of this resource believes this case is valid for re-testing readiness.

  Marked as `ForceNew` for recreate resource on change field.

- `kind` - kind of depended on resource. Required, should not empty.
  Not have additional checks for this field.
  Please use reference to another resource like `kubernetes_manifest.additional_disk.object.kind`
  to recreate resource on recreate depended on resource for start readiness check
  on new resource.

  Marked as `ForceNew` for recreate resource on change field.

- `name` - name of depended on resource. Required, should not empty.
  Not have additional checks for this field.
  Please use reference to another resource like `kubernetes_manifest.additional_disk.object.metadata.name`
  to recreate resource on recreate depended on resource for start readiness check
  on new resource. 

  Marked as `ForceNew` for recreate resource on change field.

- `namespace` - name of depended on resource. Optional, can be empty.
  Not have additional checks for this field.
  Please use reference to another resource like `kubernetes_manifest.additional_disk.object.metadata.namespace`
  to recreate resource on recreate depended on resource for start readiness check
  on new resource.

  If namespace is empty, provider creates non-namespaced dynamic client for checking.
  This case tested in `TestAccResourceReadyApplyNotNamespaced` test.

  Marked as `ForceNew` for recreate resource on change field.

- `wait_timeout` - golang `time.Duration` string for time to check readiness. 
  Required. Should not empty.
  Provider try to parse this on validation and if it cannot parse
  (for example, passed `irfirgir` string) will fail on validation.

  This attribute can be changed on any time, change will not provide new plan and
  will not start readiness check.

- `skip_check_on_create_with_resource_lifetime` - golang `time.Duration`.
  Optional. Can be empty.
  Provider try to parse this duration on validation and if it cannot parse
  (for example, passed `irfirgir` string) will fail on validation.

  If this attribute provided, provider will check lifetime of resource, 
  and if lifetime > this duration, provider will not check readiness.
  It needs for migrate to new resource and not produce readiness check.
  Provider will output warning if lifetime check was accepted. It is normal
  and does for better observability.
  This check doing only on first attempt.
  
  If you want to skip this check you can provide `0` value or not set it.

  This attribute can be changed and deleted on any time, change will not provide new plan and
  will not start readiness check.

- `fields` - map string -> string for checking resource fields on readiness.
  Optional, but one condition (see description below) or/and field in map can be provided.
  If you not provide at least one condition or field in map, provider will
  fail on create resource. Unfortunately, validate this case on schema validation
  is huge for implementing and provider will fail on create.

  Warning! because this check work only on create, if you remove all conditions
  and fields, provider allow this change, but we get error on recreate.

  Key should be path in json object in [gjson](https://github.com/tidwall/gjson) format.
  Unfortunately, `gjson` library does not provide validation function for path,
  and we cannot validate it. Set key with this warning and check path before set,
  otherwise provider continue checks without fail.
  Value can be string or golang regexp. Provider will try to parse all values string as regexp,
  and if not parsed will fail on validation.
  Warning! If value string contains meta symbols, like `.` or/and `[` or/and another,
  you should escape it with backslash, otherwise provider fail on validation.
  `gjson` can returns key as `jaon` string if field is composite type (object, array).
  Numbers and booleans will return as string, and we can check value by regexp. 
  Only warning, be carefully with float values.

  Provider waiting to **ALL** fields are matched to its expressions, 
  and **ALL** conditions all are matched for all provided attributes.
 
  This attribute can be changed and deleted on any time, change will not provide new plan and
  will not start readiness check.

- `condition` - is terraform set for checking conditions for ready.
  Optional, but one condition or/and field in map can be provided.
  If you not provide at least one condition or field in map, provider will
  fail on create resource. Unfortunately, validate this case on schema validation
  is huge for implementing and provider will fail on create.
  Warning! because this check work only on create, if you remove all conditions
  and fields, provider allow this change, but we get error on recreate.

  Condition provide next attributes:
  - `type` - Type of condition. Required. Cannot be empty.
    Provider will find this condition by full string check (without any preparations).
  - `status` - Status of condition. Required. Cannot be empty.
    Provider will find condition by type and full check status string (without any preparations).
  - `reason` - Reason of condition. Optional. Can be empty.
    This attribute can be golang regexp.
    Provider will try to parse reason string as regexp,
    and if you parsed incorrect regexp will fail on validation.
    Warning! If reason string contains meta symbols, like `.` or/and `[` or/and another,
    you should escape it with backslash, otherwise provider fail on validation.
    If empty or not provided, provider always returns valid match.
  - `message` - Message of condition. Optional. Can be empty.
    This attribute can be golang regexp.
    Provider will try to parse reason string as regexp,
    and if you parsed incorrect regexp will fail on validation.
    Warning! If reason string contains meta symbols, like `.` or/and `[` or/and another,
    you should escape it with backslash, otherwise provider fail on validation.
    If empty or not provided, provider always returns valid match.
    Warning! K8s controller can set multiline string on this field,
    and you can handle this case in regexp (for example, add `(?s)(?m)` in start of regexp).
  
  Provider waiting to **ALL** conditions all are matched for all provided attributes,
  and **ALL** fields are matched to its expressions.

  You can use same fail (see below) and ready condition type, like:
  ```
  condition {
    type = "Ready"
    status = "True"
  }
  fail_condition {
    type = "Ready"
    status = "False"
    reason = "^(HealthCheck|Mount)$"
  }
  ```
  but provider validate that same fail and ready condition type has different attributes.

  This attribute can be changed and deleted on any time, change will not provide new plan and
  will not start readiness check.

- `fail_condition` - is terraform set for checking conditions for fail fast.
  Fail condition provides same attributes with same rules as `condition` set. 
  If one of fail condition is match to it expression, provider returns not ready error.
  You can use same fail and ready condition type, like:
  ```
  condition {
    type = "Ready"
    status = "True"
  }
  fail_condition {
    type = "Ready"
    status = "False"
    reason = "^(HealthCheck|Mount)$"
  }
  ```
  but provider validate that same fail and ready condition type has different attributes.

  Provider will wait `fail_conditions_appearance_duration` (see below) for appearance the fail condition,
  If no any conditions was found, provider will attempt for appearance the fail condition.

  This attribute can be changed and deleted on any time, change will not provide new plan and
  will not start readiness check.

- `fail_conditions_appearance_duration` - golang `time.Duration`.
  Optional. Can be empty, but has `15s` as default.
  Provider try to parse this on validation and if it cannot parse
  (for example, passed `irfirgir` string) will fail on validation.

  Provider will wait this duration to for appearance the fail conditions (all).
  If fail conditions not present, provider will consider that resource not fail,
  and will not produce next attempts to check readiness.
 
We described logic of creating `kubernetes_resource_ready_v1` in attributes descriptions,
but repeat in this place:
- provider extract all setting from terraform resource data. If extraction was failed
  provider will return all errors.
- if fields and conditions not present, provider will fail with error on creation.
- provide dynamic client for getting depended on resource. Provider only get resource in cluster,
  does not do any changes.
- create waiter and start process with context timeout with `wait_timeout` duration.
- get resource from cluster without retries. If getting was failed (include resource not found)
  will continue check process.
- on first attempt, check lifetime of resource if `skip_check_on_create_with_resource_lifetime`
  provided and not zero. Resource is old, stop checking process without error
  and output warning.
- next, extract conditions from resource. Check fail conditions and ready conditions. If fail
  conditions were provided and conditions not present wait `fail_conditions_appearance_duration`
  for appearance the fail conditions.
  If ready conditions and fail conditions not provided will continue with `fields` checks.
  If one of ready conditions not match will continue check attempts without checking
  `fields`.
  If one of fail conditions is matched, stop check with error.
- next, check fields. If fields not provided, and ready conditions are matched,
  stop check without error.
  If provided, check all fields to its conditions. If one of field is not present
  or not matched - continue check.
- if ready conditions or/and fields not matched, sleep two seconds and continue check.
- if all ready checks were passed, set terraform resource id as `ApiVersion;Kind;Name;Namespace`,
  and set attribute `ready` as true. Returns without errors, but can return warnings
  from check procedure.

Checking resource readiness doing only on create operation, only, if you provide
non-zero or empty `skip_check_on_create_with_resource_lifetime`, provider will check
depended on resource lifetime and if lifetime is old, returns warning without
readiness procedure.

Attributes `api_version`, `kind`, `name` and `namespace` marked as `ForceNew`:
any changes in these attributes will produce recreate plan and provider will
do readiness check.

Changes in another attributes will check in provider on presenting `id` attribute.
If `id` attribute is not empty (update resource), provider suppress diff for these
fields. It means that you can change this fields any time without provide update plan,
and update operation will not start.

Read operation only check that terraform resource has correct `id` attribute and
returns same `kubernetes_resource_ready_v1` resource.

Delete operation also only check that terraform resource has correct `id` attribute,
and set `id` attribute to empty (it needs for terraform protocol).

Check existing operation is not implemented.

Resource ready resource handlers produces a lot of trace and debug logs.
You can pass `TF_RESOURCE_READY_TRACE_AND_DEBUG_AS_INFO=true` env to switch
debug and trace logs to info level.

# 099-node-taint-resource-test-fix.patch
This patch uses for improve developer experience and not affect provider logic. 
Fix `kubernetes/resource_kubernetes_node_taint_test.go` file.
This test uses `k8s.io/kubernetes` (and only one in tests).
When we try to run `go list -json -m -u -mod=readonly` command it fails with errors:
```
...
go: k8s.io/cloud-provider@v0.0.0: invalid version: unknown revision v0.0.0
go: k8s.io/cluster-bootstrap@v0.0.0: invalid version: unknown revision v0.0.0
go: k8s.io/controller-manager@v0.0.0: invalid version: unknown revision v0.0.0
...
```

because `k8s.io/kubernetes` cannot be used as module. For example, IDE's cannot
resolve dependencies and fail.
