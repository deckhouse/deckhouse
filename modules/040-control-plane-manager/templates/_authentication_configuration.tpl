{{- define "authenticationConfiguration" }}
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
{{- if .apiserver.oidcIssuerURL }}
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
    # An expression that fails rejects the whole token, so every claim is read behind has(): a
    # claim the token does not carry maps to null, which leaves the key out of the user's extras.
    extra:
    - key: 'user-authn.deckhouse.io/name'
      valueExpression: 'has(claims.name) ? claims.name : null'
    - key: 'user-authn.deckhouse.io/preferred_username'
      valueExpression: 'has(claims.preferred_username) ? claims.preferred_username : null'
    - key: 'user-authn.deckhouse.io/dex-provider'
      valueExpression: "has(claims.federated_claims) && has(claims.federated_claims.connector_id) ? claims.federated_claims.connector_id : null"
    # The pair (user and connector) the token was issued to, and the generation of its credentials:
    # the revocation check of user-authz refuses a token of a generation below the revoked one.
    # Numbers of the claims are doubles in CEL, hence int() before string(). Dex leaves d8_gen out
    # while the generation is 0. The empty string, not null: string() is typed, and the two
    # branches of a conditional must be of one type; "" leaves the key out just the same.
    - key: 'user-authn.deckhouse.io/sub'
      valueExpression: 'has(claims.sub) ? claims.sub : null'
    - key: 'user-authn.deckhouse.io/generation'
      valueExpression: 'has(claims.d8_gen) ? string(int(claims.d8_gen)) : ""'
  userValidationRules:
  - expression: "!user.username.startsWith('system:')"
    message: 'username cannot used reserved system: prefix'
  - expression: "user.groups.all(group, !group.startsWith('system:'))"
    message: 'groups cannot used reserved system: prefix'
{{- end }}
anonymous:
  enabled: true
  conditions:
  - path: /livez
  - path: /readyz
  - path: /healthz
{{- end }}
