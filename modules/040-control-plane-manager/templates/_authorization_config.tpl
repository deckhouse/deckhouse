{{- define "authorizationConfigTemplate" -}}
apiVersion: apiserver.config.k8s.io/v1
kind: AuthorizationConfiguration
authorizers:
  - type: Node
    name: node
  {{- if .apiserver.revocationWebhookURL }}
  # The revocation check of user-authn, served by the user-authz webhook: it denies a request made
  # with a Dex token whose credentials were revoked, and has no opinion about anything else - it
  # never allows. It fails open: an error or a timeout is no opinion, and the request goes on to
  # the authorizers below as it would without this one. daemonset.yaml sets the URL only from
  # Kubernetes 1.34 on, where the answers can be left uncached.
  - type: Webhook
    name: user-authn-revocation
    webhook:
      subjectAccessReviewVersion: v1
      matchConditionSubjectAccessReviewVersion: v1
      matchConditions:
        # Only the tokens of Dex carry this extra: everything else skips the call.
        - expression: "'user-authn.deckhouse.io/sub' in request.extra"
      timeout: 1s
      failurePolicy: NoOpinion
      # Both TTLs are required, and above zero, even with the cache off.
      authorizedTTL: 1s
      unauthorizedTTL: 1s
      # A revocation applies at the next request, not when a cached answer expires.
      cacheAuthorizedRequests: false
      cacheUnauthorizedRequests: false
      connectionInfo:
        type: KubeConfigFile
        kubeConfigFile: /etc/kubernetes/deckhouse/extra-files/revocation-webhook-config.yaml
  {{- end }}
  - type: Webhook
    name: user-authz-webhook
    webhook:
      subjectAccessReviewVersion: v1
      matchConditionSubjectAccessReviewVersion: v1
      matchConditions:
        # Exclude core control-plane identities from authz-webhook to avoid deadlocks:
        # if kube-apiserver cannot authorize these components, the cluster may not be able to recover.
        # Includes CAPI controllers observed in SubjectAccessReview logs (e.g. "capi-controller-manager").
        - expression: '!(request.user in ["system:aggregator", "system:kube-aggregator", "system:kube-controller-manager", "system:kube-scheduler", "kubernetes-admin", "kube-apiserver-kubelet-client", "capi-controller-manager", "system:volume-scheduler"])'

        # Nodes should not depend on the webhook: kubelet requests go via Node authorizer and are required
        # for node registration/heartbeats; blocking them may break the cluster.
        - expression: '!(request.user.startsWith("system:node:"))'

        # kube-system contains core in-cluster serviceaccounts (controllers, DNS, etc.);
        # exclude them to avoid self-blocking during bootstrap/upgrade.
        - expression: '!(request.user.startsWith("system:serviceaccount:kube-system:"))'

        # Deckhouse modules run under d8-* namespaces; exclude their serviceaccounts so Deckhouse can
        # always reconcile and restore the webhook if it becomes unhealthy.
        - expression: '!(request.user.startsWith("system:serviceaccount:d8-"))'

        # kube-apiserver health probes are anonymous: they must not depend on the webhook pod,
        # otherwise a webhook that cannot start keeps the API server failing its own probes.
        # RBAC (system:public-info-viewer) allows these paths for system:unauthenticated.
        - expression: '!(request.user == "system:anonymous" && has(request.nonResourceAttributes) && request.nonResourceAttributes.path in ["/livez", "/readyz", "/healthz"])'
      authorizedTTL: 5m
      unauthorizedTTL: 30s
      timeout: 3s
      # Fail closed if webhook is unavailable/returns errors.
      failurePolicy: Deny
      connectionInfo:
        type: KubeConfigFile
        kubeConfigFile: /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml
  - type: RBAC
    name: rbac
{{- end -}}
