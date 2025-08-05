{{- define "authenticationConfiguration" }}
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: {{ .apiserver.oidcIssuerURL }} 
    discoveryURL: https://dex.d8-user-authn.svc.{{.clusterConfiguration.clusterDomain }}/.well-known/openid-configuration
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
{{- end }}
