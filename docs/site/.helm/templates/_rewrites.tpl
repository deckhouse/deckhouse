{{- define "rewrites" }}
rewrite ^/$ /en/ permanent;
rewrite ^/(en|ru)/getting_started.html$ /$1/gs/ permanent;
{{- end }}
