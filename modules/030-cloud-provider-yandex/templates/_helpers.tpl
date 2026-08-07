{{- /*
  Returns one field of a credential Secret collected into internal.credentialSecrets, or an
  empty string when the Secret is absent or does not carry the field.

  Usage: {{ include "yandex_credential_secret" (list . "d8-credentials" "secret") }}
    0 — template context with .Values
    1 — Secret name, the key under internal.credentialSecrets
    2 — field to read: "secret" or "identity"

  The lookup has to tolerate an empty credentialSecrets map: the Secret is created by the
  operator (or projected from the legacy YandexClusterConfiguration by
  hooks/yandex_cluster_configuration.go), so it is missing while a cluster is being
  bootstrapped, and the workloads still have to render.
*/ -}}
{{- define "yandex_credential_secret" -}}
{{- $context := index . 0 -}}
{{- $secretName := index . 1 -}}
{{- $field := index . 2 -}}
{{- $credSecret := index $context.Values.cloudProviderYandex.internal.credentialSecrets $secretName -}}
{{- if $credSecret -}}
{{- dig $field "" $credSecret -}}
{{- end -}}
{{- end -}}
