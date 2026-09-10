{{- if .Values.userAuthz.enableMultiTenancy }}
{{- /*
  The components these alerts watch - the authorization webhook and Permission Browser - are
  rendered only when multi-tenancy is on, and this file used to be plain YAML, installed either way.
  So on every cluster with the default enableMultiTenancy: false there was no target to scrape,
  absent(up{...}) stayed true, and the TargetDown alert below fired permanently five minutes after
  install. Gating the whole file is the fix: no components, no alerts about them.
*/}}
- name: d8.user-authz.webhook.rules
  rules:
  - alert: D8UserAuthzWebhookTargetDown
    expr: |
      (
        count(up{job="user-authz-webhook", namespace="d8-user-authz"} == 0) > 0
        or
        absent(up{job="user-authz-webhook", namespace="d8-user-authz"})
      )
    for: 5m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: user-authz
      d8_component: user-authz-webhook
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Prometheus is unable to scrape the user-authz webhook.
      description: |-
        Prometheus cannot collect metrics from at least one instance of `user-authz-webhook` in the `d8-user-authz` namespace. One instance without metrics is enough for the alert to fire.

        If the instance is running and only its metrics are unreachable, it keeps answering authorization requests, but the other alerts of the module cannot see its state. If it is not running or has not read the rules yet, it answers `503` to every authorization request, and the API server denies the request, because the webhook is configured with `failurePolicy: Deny`.

        Check the DaemonSet, its metrics sidecar and the scrape target:

        ```bash
        d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c kube-rbac-proxy --tail=50
        ```

  - alert: D8UserAuthzWebhookRulesQuarantined
    expr: |
      max by (namespace) (
        user_authz_webhook_rules_quarantined{job="user-authz-webhook", namespace="d8-user-authz"}
      ) > 0
    for: 10m
    labels:
      severity_level: "7"
      tier: cluster
      d8_module: user-authz
      d8_component: user-authz-webhook
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: A ClusterAuthorizationRule cannot be compiled by the webhook.
      description: |-
        For more than 10 minutes the `user-authz-webhook` has quarantined at least one `ClusterAuthorizationRule`: its `limitNamespaces` pattern or `namespaceSelector` does not compile, or the rule cannot be read at all.

        A rule with a broken filter is applied without that filter, so its subjects get a narrower scope than written and are denied where the filter was meant to allow. A rule that cannot be read is not applied at all, and its subjects are denied every namespace.

        Find the rule:

        ```bash
        d8 k get clusterauthorizationrules -o custom-columns=NAME:.metadata.name,LIMIT:.spec.limitNamespaces,SELECTOR:.spec.namespaceSelector
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100 | grep -i quarantin
        ```

  - alert: D8UserAuthzWebhookRulesWatchErrors
    expr: |
      sum by (namespace) (
        rate(user_authz_webhook_rules_watch_errors_total{job="user-authz-webhook", namespace="d8-user-authz"}[5m])
      ) > 0
    for: 10m
    labels:
      severity_level: "7"
      tier: cluster
      d8_module: user-authz
      d8_component: user-authz-webhook
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: The user-authz webhook cannot watch the ClusterAuthorizationRules.
      description: |-
        For more than 10 minutes the `user-authz-webhook` rules informer has been failing to watch the `ClusterAuthorizationRules` (a non-zero rate of watch errors). New or changed rules may not be reaching the webhook, so multi-tenancy decisions can lag behind what users configure.

        Check the webhook's access to the API and its logs:

        ```bash
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100
        d8 k auth can-i watch clusterauthorizationrules.deckhouse.io --as=system:serviceaccount:d8-user-authz:webhook
        ```

  - alert: D8UserAuthzWebhookDirectoryDiverged
    expr: |
      (
        max(user_authz_webhook_rules_max_resource_version{job="user-authz-webhook", namespace="d8-user-authz"})
        -
        min(user_authz_webhook_rules_max_resource_version{job="user-authz-webhook", namespace="d8-user-authz"})
      ) > 0
    for: 10m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: user-authz
      d8_component: user-authz-webhook
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: The masters disagree about the multi-tenancy rules.
      description: |-
        Instances of `user-authz-webhook` have been using different sets of `ClusterAuthorizationRules` for 10 minutes.

        Each master node authorizes the requests that reach it, so the same request is answered differently depending on the node: a user can be inside their namespace limits on one node and outside them on another.

        Find the instance that is behind and look at why:

        ```bash
        d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100 | grep -i 'list/watch'
        ```

        A growing `user_authz_webhook_rules_watch_errors_total` on one instance points at its watch; a `user_authz_webhook_rules_directory_updated_timestamp_seconds` that has stopped moving points at rules that are no longer updated. Deleting the lagging Pod is the last resort: the DaemonSet recreates it, and on a cluster with a single master node every request in the cluster is denied while the Pod restarts.

  - alert: D8UserAuthzRulePropagationLag
    expr: |
      (
        time() - min(user_authz_webhook_rules_directory_updated_timestamp_seconds{job="user-authz-webhook", namespace="d8-user-authz"}) > 3600
      )
      and
      (
        time() - max(user_authz_webhook_rules_directory_updated_timestamp_seconds{job="user-authz-webhook", namespace="d8-user-authz"}) < 3600
      )
    for: 15m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: user-authz
      d8_component: user-authz-webhook
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: One user-authz webhook instance stopped picking up rule changes.
      description: |-
        One instance of `user-authz-webhook` has not updated its rules for more than an hour while another instance has. The rules are changing in the cluster and this instance does not see the changes.

        Find the instance and check its log:

        ```bash
        d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100
        ```

        An instance whose watch keeps failing becomes not ready, so check the readiness of the Pod as well.
{{- end }}
