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

## Как вручную перезапустить сканирование ресурса, как понять когда ресурс будет просканирован повторно?

Модуль каждые 24 часа выполняет повторное сканирование ресурсов, согласно следующему алгоритму:

В namespace c каждый просканированным ресурсом создается объект `VulnerabilityReport`.  
В данном объекте присутствует аннотация `trivy-operator.aquasecurity.github.io/report-ttl`, которая указывает время жизни отчета (стандартно - `24h`). 
По истечении этого времени оператор удаляет объект, что вызывает повторное сканирование ресурса.

Таким образом, для принудительного повторного сканирования ресурса необходимо перезаписать аннотацию `trivy-operator.aquasecurity.github.io/report-ttl`, указав малый промежуток времени.  
Допустимо также полностью удалить объект `VulnerabilityReport`.

Пример команды указания аннотации:
```bash
kubectl annotate VulnerabilityReport -n <namespace> <reportName>  trivy-operator.aquasecurity.github.io/report-ttl=1s --overwrite
```