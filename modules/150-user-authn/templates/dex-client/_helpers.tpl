{{- /* The fields of the OAuth2Client of a DexClient for the sign-out through the sessions of dex: */ -}}
{{- /* the addresses to return to, the back-channel notification and how long the refresh tokens live. */ -}}
{{- /* Usage: include "dex_client_logout_fields" $crd.spec */ -}}
{{- define "dex_client_logout_fields" -}}
  {{- if .postLogoutRedirectURIs }}
postLogoutRedirectURIs:
    {{- range $uri := .postLogoutRedirectURIs }}
- {{ $uri | quote }}
    {{- end }}
  {{- end }}
  {{- with .backchannelLogoutURI }}
backchannelLogoutURI: {{ . | quote }}
  {{- end }}
  {{- with .refreshTokenLifetime }}
refreshTokenLifetime: {{ . | lower | quote }}
  {{- end }}
{{- end -}}
