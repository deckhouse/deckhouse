{{- define "cert_manager.yandex_dns_configured" -}}
{{- if and .Values.certManager.yandexFolderID .Values.certManager.yandexServiceAccountJSON -}}
true
{{- end -}}
{{- end -}}

{{- define "cert_manager.yandex_dns_webhook_ready" -}}
{{- if and (include "cert_manager.yandex_dns_configured" .) (hasKey .Values.certManager "internal") (hasKey .Values.certManager.internal "yandexWebhookCert") -}}
{{- $cert := .Values.certManager.internal.yandexWebhookCert -}}
{{- if and $cert.ca $cert.crt $cert.key -}}
true
{{- end -}}
{{- end -}}
{{- end -}}
