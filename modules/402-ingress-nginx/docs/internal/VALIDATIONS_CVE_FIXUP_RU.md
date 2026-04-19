---
title: "Исправление уязвимости CVE-2025-1974 в Ingress-nginx контроллере"
---

Фикс отключает ValidationWebhook в подах Ingress-nginx контроллера с помощью MutatingWebhookConfiguration.

Для применения фикса выполните следующую команду на хосте с доступом к кластеру Kubernetes с правами пользователя `ClusterAdmin`:

```bash
curl -sL https://raw.githubusercontent.com/deckhouse/deckhouse/refs/heads/main/modules/402-ingress-nginx/docs/internal/validations_cve_fixup.sh | bash
```

После развёртывания фикса необходимо убедиться, что поды `d8-ingress-validation-cve-fixer` запущены:

```bash
d8 k -n d8-system get pods -lapp=ingress-validation-cve-fixer
```

Также необходимо с помощью команды

```bash
d8 k  edit ingressnginxcontrollers.deckhouse.io
```

выставить параметр `spec.validationEnabled` в значение `false` и поочередно перезапустить поды Ingress-nginx контроллера в пространстве имен `d8-ingress-nginx`.

После перезапуска можно проверить наличие уязвимых подов командой:

```bash
d8 k -n d8-ingress-nginx get pods -lapp=controller -o json | jq -r '.items[] | select(.spec.containers[].args[]? == "--validating-webhook=:8443") | .metadata.name'
```
