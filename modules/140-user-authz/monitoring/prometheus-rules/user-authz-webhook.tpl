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
        sum(up{job="user-authz-webhook", namespace="d8-user-authz"}) == 0
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
        Prometheus cannot collect metrics from `user-authz-webhook` in the `d8-user-authz` namespace.

        The webhook keeps authorizing requests, but while this alert is active the other webhook alerts cannot fire: a webhook that has not listed the `ClusterAuthorizationRules`, or that quarantined a rule, will go unnoticed.

        Check the DaemonSet, its metrics sidecar and the scrape target:

        ```bash
        d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c kube-rbac-proxy --tail=50
        ```

  - alert: D8UserAuthzWebhookRulesNotSynced
    expr: |
      count by (namespace) (
        user_authz_webhook_rules_informer_synced{job="user-authz-webhook", namespace="d8-user-authz"} == 0
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
      summary: The user-authz webhook has not listed the ClusterAuthorizationRules.
      description: |-
        For more than 10 minutes at least one `user-authz-webhook` instance has not listed the `ClusterAuthorizationRules` (its rules informer reports `synced=0`).

        While the rules are not listed, the webhook keeps every subject that a controller-managed ClusterRoleBinding binds maximally restricted: those users are denied namespaced and cluster-scoped access until the rules arrive. The restriction lifts on its own once the informer syncs.

        A cluster where the `ClusterAuthorizationRule` CRD does not exist at all also reports `synced=0`; in that case there are no rules and no rule bindings, so nobody is restricted.

        Check the webhook and its access to the CRD:

        ```bash
        d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100
        d8 k get crd clusterauthorizationrules.deckhouse.io
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
        For more than 10 minutes the `user-authz-webhook` has quarantined at least one `ClusterAuthorizationRule`: its `limitNamespaces` pattern or `namespaceSelector` could not be compiled. The rule is kept and the rest of it still applies, but the broken filter is left out, so its subjects get a narrower scope than written and are denied where the filter was meant to allow.

        Find the offending rule (an invalid regular expression in `limitNamespaces` or a malformed `namespaceSelector`):

        ```bash
        d8 k get clusterauthorizationrules -o custom-columns=NAME:.metadata.name,LIMIT:.spec.limitNamespaces,SELECTOR:.spec.namespaceSelector
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100 | grep -i quarantin
        ```

  - alert: D8UserAuthzWebhookRulesWatchErrors
    expr: |
      sum by (namespace) (
        rate(user_authz_webhook_rules_watch_errors_total{job="user-authz-webhook", namespace="d8-user-authz"}[5m])
      ) > 0
    for: 15m
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
        For more than 15 minutes the `user-authz-webhook` rules informer has been failing to watch the `ClusterAuthorizationRules` (a non-zero rate of watch errors). New or changed rules may not be reaching the webhook, so multi-tenancy decisions can lag behind what users configure.

        Check the webhook's access to the API and its logs:

        ```bash
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100
        d8 k auth can-i watch clusterauthorizationrules.deckhouse.io --as=system:serviceaccount:d8-user-authz:webhook
        ```

  - alert: D8UserAuthzWebhookRulesStale
    expr: |
      min by (namespace) (
        time() - user_authz_webhook_rules_directory_updated_timestamp_seconds{job="user-authz-webhook", namespace="d8-user-authz"}
      ) > 86400
    for: 30m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: user-authz
      d8_component: user-authz-webhook
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_user_authz_webhook_malfunctioning: "D8UserAuthzWebhookMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: The user-authz webhook has not rebuilt its rules directory for a day.
      description: |-
        The `user-authz-webhook` has not rebuilt its `ClusterAuthorizationRule` directory for more than 24 hours. That is normal in a cluster where nobody changes the rules, and expected where there are none.

        It is worth a look only together with a change that should have arrived: if a rule was edited and this timestamp did not move, the webhook is serving a frozen directory and is enforcing the rules as they were.

        Compare what the webhook holds with what the cluster has:

        ```bash
        d8 k get clusterauthorizationrules -o custom-columns=NAME:.metadata.name,RV:.metadata.resourceVersion
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100
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
        Instances of `user-authz-webhook` have been built from different sets of `ClusterAuthorizationRules` for 10 minutes. Every instance publishes the highest `resourceVersion` it has observed, and those numbers have not converged.

        Each master authorizes the requests that reach it, so this means the same request is answered differently depending on which master takes it: a user can be inside their namespace limits on one and outside them on another, seemingly at random. It normally resolves in seconds — the alert waits ten minutes precisely so that ordinary propagation does not fire it.

        Find the instance that is behind and look at why:

        ```bash
        d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100 | grep -i 'list/watch'
        ```

        A growing `user_authz_webhook_rules_watch_errors_total` on one instance points at its watch; a `user_authz_webhook_rules_directory_updated_timestamp_seconds` that has stopped moving points at a directory that is no longer being rebuilt. Deleting the lagging Pod is the blunt fix, and the DaemonSet will bring it back — on a single-master cluster that briefly denies every request in the cluster, because this webhook is fail-closed.

  - alert: D8UserAuthzRulePropagationLag
    expr: |
      (
        max(rate(user_authz_webhook_rules_directory_rebuilds_total{job="user-authz-webhook", namespace="d8-user-authz"}[1h])) > 0
      )
      and
      (
        min(rate(user_authz_webhook_rules_directory_rebuilds_total{job="user-authz-webhook", namespace="d8-user-authz"}[1h])) == 0
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
        One instance of `user-authz-webhook` has not rebuilt its directory for over an hour while another one has. The rules are changing in the cluster and this instance is not seeing the changes.

        This is the asymmetric case that `D8UserAuthzWebhookDirectoryDiverged` can miss: the instances can agree on the highest `resourceVersion` they have seen and still differ, if the one that is behind simply stopped receiving events rather than falling behind on a particular object.

        Both halves are rates rather than counter values, and that is not cosmetic: `rebuilds_total` is a per-process counter that starts at zero, so comparing the values directly meant that one Pod restart left `max - min` permanently positive and the alert permanently on. The rate over an hour says what was actually meant — somebody is rebuilding, somebody is not — and it survives restarts. A cluster whose rules genuinely never change has every instance at rate zero and stays silent.

        ```bash
        d8 k -n d8-user-authz get pods -l app=user-authz-webhook -o wide
        d8 k -n d8-user-authz logs -l app=user-authz-webhook -c webhook --tail=100
        ```

        The instance's `/readyz` reports a directory that has stopped tracking the cluster once its watch has failed enough times in a row, so check whether the Pod is also unready — if it is, the rollout is already held and the cause is in the log above.
{{- end }}
