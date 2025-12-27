{{- define "authorizationConfigTemplate" -}}
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthorizationConfiguration
authorizers:
  - type: Node
  - type: Webhook
    name: user-authz-webhook
    webhook:
      subjectAccessReviewVersion: v1
      matchConditionSubjectAccessReviewVersion: v1
      authorizedTTL: 5m
      unauthorizedTTL: 30s
      timeout: 3s
      # Fail closed if webhook is unavailable/returns errors.
      failurePolicy: Deny
      connectionInfo:
        type: KubeConfig
        kubeConfigFile: /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml
  - type: RBAC
{{- end -}}
