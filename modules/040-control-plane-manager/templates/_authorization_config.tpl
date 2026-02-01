{{- define "authorizationConfigTemplate" -}}
{{- if semverCompare "<1.30" .clusterConfiguration.kubernetesVersion }}
apiVersion: apiserver.config.k8s.io/v1alpha1
{{- else }}
apiVersion: apiserver.config.k8s.io/v1beta1
{{- end }}
kind: AuthorizationConfiguration
authorizers:
  - type: Node
    name: node
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
