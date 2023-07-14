- name: kubernetes.user-authz.deprecated-spec
  rules:
  - alert: UserAuthzDeprecatedCARSpec
    expr: >-
      d8_deprecated_car_spec > 0
    labels:
      severity_level: "9"
    annotations:
      description: |-
        There is a cluster authorization rule with [deprecated]({{ include "helm_lib_module_documentation_uri" (list . "/modules/140-user-authz/#implementation-nuances") }}) spec parameters - either 'spec.limitNamespaces' or 'spec.AllowAccessToSystemNamespaces', or both. Migrate to '.spec.namespaceSelector'.

        Use the following command to get the list of the affected clusterAuthorizationRules:
        `kubectl  get clusterauthorizationrules.deckhouse.io -o json | jq '.items[] | select(.spec.limitNamespaces != null or .spec.allowAccessToSystemNamespaces != null) | .metadata.name'`
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_extended_monitoring_deprecated_annotation: "D8UserAuthzDeprecatedSpec,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_extended_monitoring_deprecated_annotation: "D8UserAuthzDeprecatedSpec,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: There is a cluster authorization rule with deprecated '.spec.limitNamespaces' parameter.
