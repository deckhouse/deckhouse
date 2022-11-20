{{- define "rewrites" }}
rewrite ^/(en|ru)/getting_started.html$ /$1/gs/ permanent;
rewrite ^/ru/terms-of-service\.html /ru/security-policy.html permanent;
rewrite ^/ru/cookie-policy\.html /ru/security-policy.html permanent;
rewrite ^/ru/privacy-policy\.html /ru/security-policy.html permanent;
rewrite ^/en/security-policy\.html /en/privacy-policy.html permanent;
{{- end }}
