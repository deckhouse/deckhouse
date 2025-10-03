{{- define "authenticationConfiguration" }}
{{- if semverCompare "< 1.30" .clusterConfiguration.kubernetesVersion }}
apiVersion: apiserver.config.k8s.io/v1alpha1
{{- else }}
apiVersion: apiserver.config.k8s.io/v1beta1
{{- end }}
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: {{ .apiserver.oidcIssuerURL }} 
    {{- if semverCompare ">= 1.30" .clusterConfiguration.kubernetesVersion }}
    discoveryURL: https://dex.d8-user-authn.svc.{{.clusterConfiguration.clusterDomain }}/.well-known/openid-configuration
    {{- end }}
    {{- if .apiserver.oidcCA }}
    certificateAuthority: |
      {{- .apiserver.oidcCA | nindent 6 }} 
    {{- end }}    
    audiences:
    - kubernetes
  claimMappings:
    username:
      claim: "email"
      prefix: ""
    groups:
      claim: "groups"
      prefix: ""
    extra:
    - key: 'user-authn.deckhouse.io/name'
      valueExpression: 'claims.name'
    - key: 'user-authn.deckhouse.io/preferred_username'
      valueExpression: 'has(claims.preferred_username) ? claims.preferred_username : null'
    - key: 'user-authn.deckhouse.io/dex-provider'
      valueExpression: "has(claims.federated_claims) && has(claims.federated_claims.connector_id) ? claims.federated_claims.connector_id : null"
  userValidationRules:
  - expression: "!user.username.startsWith('system:')"
    message: 'username cannot used reserved system: prefix'
  - expression: "user.groups.all(group, !group.startsWith('system:'))"
    message: 'groups cannot used reserved system: prefix'          
{{- end }}
