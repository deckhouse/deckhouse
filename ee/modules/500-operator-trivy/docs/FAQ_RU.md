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

## Как вручную перезапустить сканирование ресурса и когда происходит повторное сканирование?

Модуль выполняет повторное сканирование ресурсов каждые 24 часа согласно следующему алгоритму:

1. В пространстве имён c каждым просканированным ресурсом создаётся объект `VulnerabilityReport`.
1. В этом объекте присутствует аннотация `trivy-operator.aquasecurity.github.io/report-ttl`, которая указывает время жизни отчёта (по умолчанию - `24h`).
1. По истечении этого времени объект удаляется, что вызывает повторное сканирование ресурса.

Принудительно запустить повторное сканирование ресурса можно одним из двух способов:

- Перезапишите аннотацию `trivy-operator.aquasecurity.github.io/report-ttl`, указав короткое время жизни отчёта.
- Удалите объект `VulnerabilityReport` из пространства имён, где находится просканированный ресурс.

Пример команды для перезаписи аннотации `trivy-operator.aquasecurity.github.io/report-ttl`:

```bash
kubectl annotate VulnerabilityReport -n <namespace> <reportName>  trivy-operator.aquasecurity.github.io/report-ttl=1s --overwrite
```
