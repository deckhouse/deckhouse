{{- if .Values.userAuthz.enableMultiTenancy }}
{{- /*
  The components these alerts watch - the authorization webhook and Permission Browser - are
  rendered only when multi-tenancy is on, and this file used to be plain YAML, installed either way.
  So on every cluster with the default enableMultiTenancy: false there was no target to scrape,
  absent(up{...}) stayed true, and the TargetDown alert below fired permanently five minutes after
  install. Gating the whole file is the fix: no components, no alerts about them.
*/}}
- name: d8.user-authz.permission-browser-apiserver.availability
  rules:
  - alert: D8UserAuthzPermissionBrowserUnavailable
    expr: |
      (
        kube_deployment_spec_replicas{
          namespace="d8-user-authz",
          deployment="permission-browser-apiserver"
        }
        -
        kube_deployment_status_replicas_available{
          namespace="d8-user-authz",
          deployment="permission-browser-apiserver"
        }
      ) > 0
    for: 5m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: user-authz
      d8_component: permission-browser-apiserver
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_malfunctioning: "D8UserAuthzMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_malfunctioning: "D8UserAuthzMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Permission Browser API server has unavailable replicas.
      description: |-
        Deckhouse has detected unavailable replicas in the `permission-browser-apiserver` deployment in the `d8-user-authz` namespace.
        Permission Browser requests can fail while this alert is active.

        Check deployment and pod states:
        ```bash
        d8 k -n d8-user-authz get deploy permission-browser-apiserver
        d8 k -n d8-user-authz get pods -l app=permission-browser-apiserver
        ```

  - alert: D8UserAuthzPermissionBrowserTargetDown
    expr: |
      (
        sum(up{job="permission-browser-apiserver", namespace="d8-user-authz"}) == 0
        or
        absent(up{job="permission-browser-apiserver", namespace="d8-user-authz"})
      )
    for: 5m
    labels:
      severity_level: "7"
      tier: cluster
      d8_module: user-authz
      d8_component: permission-browser-apiserver
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_malfunctioning: "D8UserAuthzMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_malfunctioning: "D8UserAuthzMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Prometheus is unable to scrape the Permission Browser API server.
      description: |-
        Prometheus cannot collect metrics from `permission-browser-apiserver` in the `d8-user-authz` namespace.

        Permission Browser keeps answering requests, but while this alert is active it cannot be told whether it has the current `ClusterAuthorizationRules`, so the access it reports may silently differ from the access the API server enforces.

        Check the Deployment, its metrics sidecar and the scrape target:

        ```bash
        d8 k -n d8-user-authz get pods -l app=permission-browser-apiserver -o wide
        d8 k -n d8-user-authz logs -l app=permission-browser-apiserver -c kube-rbac-proxy --tail=50
        ```

  - alert: D8UserAuthzPermissionBrowserRulesNotSynced
    expr: |
      count by (namespace) (
        user_authz_permission_browser_rules_informer_synced{job="permission-browser-apiserver", namespace="d8-user-authz"} == 0
      ) > 0
    for: 10m
    labels:
      severity_level: "7"
      tier: cluster
      d8_module: user-authz
      d8_component: permission-browser-apiserver
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_malfunctioning: "D8UserAuthzMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_malfunctioning: "D8UserAuthzMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Permission Browser has not listed the ClusterAuthorizationRules.
      description: |-
        For more than 10 minutes at least one `permission-browser-apiserver` replica has not listed the `ClusterAuthorizationRules` (its rules informer reports `synced=0`).

        While the rules are not listed, every subject bound by a controller-managed ClusterRoleBinding is reported as having no namespace access at all, so users see an empty or heavily truncated list of accessible namespaces. The report corrects itself once the informer syncs.

        A cluster where the `ClusterAuthorizationRule` CRD does not exist also reports `synced=0`; there are then no rules to report on.

        Check the replicas and their access to the CRD:

        ```bash
        d8 k -n d8-user-authz get pods -l app=permission-browser-apiserver
        d8 k -n d8-user-authz logs -l app=permission-browser-apiserver -c apiserver --tail=100
        d8 k get crd clusterauthorizationrules.deckhouse.io
        ```
{{- end }}
