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
        # Bypass authz webhook for core control-plane identities to avoid deadlocks
        # and reduce blast radius if webhook becomes unavailable.
        - expression: '!(request.user in ["system:aggregator", "system:kube-aggregator", "system:kube-controller-manager", "system:kube-scheduler", "kubernetes-admin"])'
        - expression: '!(request.user.startsWith("system:node:"))'
        - expression: '!(request.user.startsWith("system:serviceaccount:kube-system:"))'
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
