## Фикс для уязвимости CVE-2025-1974 выявленной в Ingress-nginx:

### TL;DR: Данный фикс отключает ValidationWebhook в подах Ingress-nginx контроллера с помощью MutatingWebhookConfiguration.

Для применения фикса необходимо выполнить команду `` на хосте с доступом к кластеру kubernetes с правами пользователя ClusterAdmin.

После развертывания фикса необходимо убедиться, что поды d8-ingress-validation-cve-fixer запущены - `kubectl -n d8-system get pods -lapp=ingress-validation-cve-fixer`.
Так же, необходимо поочередно перезапустить поды Ingress-nginx контроллера в пространстве имен d8-ingress-nginx.

После перезапуска можно проверить наличие уязвимых подов командой: `kubectl -n d8-ingress-nginx get pods -lapp=controller -o json | jq -r '.items[] | select(.spec.containers[].args[]? == "--validating-webhook=:8443") | .metadata.name'`.
