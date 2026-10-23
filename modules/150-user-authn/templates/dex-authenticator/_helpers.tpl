{{- /* The id of the dex client of a DexAuthenticator: "@" joins name and namespace, which can both */ -}}
{{- /* contain a dash but not "@". DexAuthenticatorClientID of the hook and of user-authn-controller */ -}}
{{- /* is the same. Usage: include "dex_authenticator_client_id" $crd */ -}}
{{- define "dex_authenticator_client_id" -}}
{{ printf "dex-authenticator-%s@%s" .name .namespace }}
{{- end -}}

{{- /* Labels that find every dex client of the DexAuthenticators and the resource it belongs to. */ -}}
{{- /* Usage: include "dex_authenticator_client_labels" $crd */ -}}
{{- define "dex_authenticator_client_labels" -}}
deckhouse.io/dex-authenticator-for: {{ .name | quote }}
deckhouse.io/dex-authenticator-namespace: {{ .namespace | quote }}
{{- end -}}

{{- /* The annotation that names the dex client on the objects of a DexAuthenticator: Service, */ -}}
{{- /* Ingress and HTTPRoute. Usage: include "dex_authenticator_client_id_annotation" $crd */ -}}
{{- define "dex_authenticator_client_id_annotation" -}}
dexauthenticator.deckhouse.io/client-id: {{ include "dex_authenticator_client_id" . | quote }}
{{- end -}}

{{- /* The OAuth2Client that registers a DexAuthenticator in dex. Every field of the client is here, */ -}}
{{- /* so a client of a shared authenticator, the post-logout redirect URIs and the session-bound */ -}}
{{- /* refresh tokens of the sign-out through dex are added in one place. */ -}}
{{- /* Usage: include "dex_authenticator_oauth2client" (list $context $crd) */ -}}
{{- define "dex_authenticator_oauth2client" -}}
  {{- $context := index . 0 -}}
  {{- $crd := index . 1 -}}
  {{- $clientID := include "dex_authenticator_client_id" $crd -}}
  {{- $labels := include "dex_authenticator_client_labels" $crd | fromYaml -}}
  {{- $_ := set $labels "app" "dex" }}
apiVersion: dex.coreos.com/v1
kind: OAuth2Client
metadata:
  name: {{ $crd.encodedName }}
  namespace: d8-{{ $context.Chart.Name }}
  {{- include "helm_lib_module_labels" (list $context $labels) | nindent 2 }}
id: {{ $clientID | quote }}
name: {{ $clientID | quote }}
secret: {{ $crd.credentials.appDexSecret | quote }}
  {{- if $crd.spec.allowedEmails }}
allowedEmails:
    {{- range $email := $crd.spec.allowedEmails }}
  - {{ $email | quote }}
    {{- end }}
  {{- end }}
  {{- if $crd.spec.allowedGroups }}
allowedGroups:
    {{- range $group := $crd.spec.allowedGroups }}
  - {{ $group | quote }}
    {{- end }}
  {{- end }}
redirectURIs:
  {{- range $app := $crd.spec.applications }}
- {{ printf "https://%s/dex-authenticator/callback" $app.domain | quote }}
  {{- end }}
{{- /* Sign-out through dex returns to the root of the application the user signed out of. */}}
postLogoutRedirectURIs:
  {{- range $app := $crd.spec.applications }}
- {{ printf "https://%s/" $app.domain | quote }}
  {{- end }}
  {{- if dig "sessions" "enabled" false $context.Values.userAuthn }}
{{- /* The session of the application ends with the session of dex it signed in through. */}}
refreshTokenLifetime: session
    {{- if eq (include "dex_authenticator_shares_sso" (list $context $crd)) "true" }}
ssoSharedWith:
      {{- range $peer := $context.Values.userAuthn.internal.dexAuthenticatorCRDs }}
        {{- if and (ne (include "dex_authenticator_client_id" $peer) $clientID) (eq (include "dex_authenticator_shares_sso" (list $context $peer)) "true") }}
- {{ include "dex_authenticator_client_id" $peer | quote }}
        {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end -}}

{{- /* Whether a DexAuthenticator shares the sign-in through the sessions of dex with the other web */ -}}
{{- /* interfaces of the platform: with sessions.sso Platform, the DexAuthenticators in the d8-* namespaces. */ -}}
{{- /* Usage: include "dex_authenticator_shares_sso" (list $context $crd) */ -}}
{{- define "dex_authenticator_shares_sso" -}}
  {{- $context := index . 0 -}}
  {{- $crd := index . 1 -}}
  {{- $sso := dig "sessions" "sso" "Platform" $context.Values.userAuthn -}}
{{ and (eq $sso "Platform") (hasPrefix "d8-" $crd.namespace) }}
{{- end -}}

{{- /* The address of the Redis session store of the DexAuthenticators. */ -}}
{{- define "dex_authenticator_redis_url" -}}
{{ .Values.userAuthn.internal.dexAuthenticatorRedisURL | default "redis://127.0.0.1/" }}
{{- end -}}

{{- /* Returns "true" when the Redis session store is the sidecar of the pod: a plain redis:// address
       without credentials on the local host. The sidecar has neither TLS nor a password. */ -}}
{{- define "dex_authenticator_redis_is_sidecar" -}}
  {{- $url := urlParse (include "dex_authenticator_redis_url" .) -}}
  {{- if and (eq $url.scheme "redis") (empty $url.userinfo) (regexMatch "^(127\\.0\\.0\\.1|localhost)(:[0-9]+)?$" $url.host) -}}
true
  {{- end -}}
{{- end -}}

{{- /* The port of the Redis sidecar: the port of the local address, if it has one. */ -}}
{{- define "dex_authenticator_redis_sidecar_port" -}}
  {{- $host := (urlParse (include "dex_authenticator_redis_url" .)).host -}}
  {{- if contains ":" $host -}}
{{ last (splitList ":" $host) }}
  {{- end -}}
{{- end -}}

{{- /* The session store arguments of oauth2-proxy. Usage: include "dex_authenticator_session_store_args" $context */ -}}
{{- define "dex_authenticator_session_store_args" -}}
- --session-store-type=redis
- --redis-connection-url={{ include "dex_authenticator_redis_url" . }}
{{- end -}}
