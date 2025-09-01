{{- define "rewrites" }}
rewrite ^/documentation/(.*)$ /products/kubernetes-platform/documentation/$1 permanent;
rewrite ^/gs/(.*)$ /products/kubernetes-platform/gs/$1 permanent;
rewrite ^/guides/(.*)$ /products/kubernetes-platform/guides/$1 permanent;
rewrite ^/products/kubernetes-platform/documentation$ /products/kubernetes-platform/documentation/ permanent;
rewrite ^/products/kubernetes-platform/gs$ /products/kubernetes-platform/gs/ permanent;
rewrite ^/products/kubernetes-platform/guides$ /products/kubernetes-platform/guides/ permanent;
rewrite ^/products/kubernetes-platform/platform/(.*)$ /products/kubernetes-platform/documentation/v1/$1 redirect;
#rewrite ^/modules/(.*)$ /products/kubernetes-platform/modules/$1 permanent;
#rewrite ^/source/modules/(.*)$ /modules/$1 redirect;
rewrite ^/platform/(.*)$ /products/kubernetes-platform/documentation/v1/$1 redirect;
rewrite ^.*/documentation/v1/modules/490-virtualization/(examples|configuration|cr|faq).html(.*)$ /modules/virtualization/stable/$1.html$2 permanent;
rewrite ^.*/documentation/v1/modules/490-virtualization/.*$ /modules/virtualization/stable/ permanent;
rewrite ^/products/kubernetes-platform/modules/csi-yadro/(.*)?$ /products/kubernetes-platform/modules/csi-yadro-tatlin-unified/$1 permanent;
rewrite ^/products/kubernetes-platform/modules/sds-drbd/(.*)?$ /products/kubernetes-platform/modules/sds-replicated-volume/$1 permanent;
#rewrite ^/modules/([^./]+)/?$ /modules/$1/stable/ permanent;
#rewrite ^/modules/([^./]+)/((?!(alpha|beta|early-access|stable|rock-solid)).+)$ /modules/$1/stable/$2 permanent;
# Redirect to stable version for module.
rewrite ^/modules/([^\./]+)/((?!(v[0-9]+\.[0-9]+|alpha|beta|early-access|stable|rock-solid|latest)).*)$ /modules/$1/stable/$2 redirect;
rewrite ^(/en|/ru)?(/documentation/v1\.[0-9]+)\.[0-9]+(/.*)$ /products/kubernetes-platform$2$3 permanent;
rewrite ^/ru/terms-of-service\.html /ru/security-policy.html permanent;
rewrite ^/ru/cookie-policy\.html /ru/security-policy.html permanent;
rewrite ^/ru/privacy-policy\.html /ru/security-policy.html permanent;
rewrite ^/en/security-policy\.html /en/privacy-policy.html permanent;
{{- end }}
