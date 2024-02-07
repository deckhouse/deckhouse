---
title: "Модуль operator-trivy: FAQ"
description: Как в модуле operator-trivy Deckhouse посмотреть ресурсы, которые не прошли CIS compliance-проверки.
---
{% raw %}

## Как посмотреть все ресурсы, которые не прошли CIS compliance-проверки?

```bash
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq '.status.detailReport.results | map(select(.checks | map(.success) | all | not))'
```

## Как посмотреть ресурсы, которые не прошли конкретную CIS compliance-проверку?

По `id`:

```bash
check_id="5.7.3"
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq --arg check_id "$check_id" '.status.detailReport.results | map(select(.id == $check_id))'
```

По описанию:

```bash
check_desc="Apply Security Context to Your Pods and Containers"
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq --arg check_desc "$check_desc" '.status.detailReport.results | map(select(.description == $check_desc))'
```

{% endraw %}
