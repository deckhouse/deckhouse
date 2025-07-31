{{- define "authenticationConfiguration" }}
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: {{ .apiserver.oidcIssuerURL }} 
    discoveryURL: https://dex.d8-user-authn.svc.{{.clusterConfiguration.clusterDomain }}/.well-known/openid-configuration
    {{- if .apiserver.oidcCA }}
    certificateAuthority: /etc/kubernetes/deckhouse/extra-files/oidc-ca.crt
    {{- end }}
    {{- if .apiserver.oidcIssuerURL }}
    audiences:
    - kubernetes
    {{- end }}    
  claimMappings:
    username:
      claim: "email"
      prefix: ""
    groups:
      claim: "groups"
      prefix: ""
{{- end }}
