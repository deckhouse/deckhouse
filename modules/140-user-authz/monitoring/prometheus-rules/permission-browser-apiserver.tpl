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

{{- end }}
