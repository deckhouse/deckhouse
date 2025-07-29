---
title: "Модуль operator-trivy"
description: Периодическое сканирование на уязвимости в кластере Deckhouse Kubernetes Platform.
---

Модуль позволяет запускать регулярную проверку пользовательских образов в runtime на известные CVE, включая уязвимости Astra Linux, Redos и ALT Linux. Базируется на проекте [Trivy](https://github.com/aquasecurity/trivy). Для сканирования используются [публичные базы](https://github.com/aquasecurity/trivy-db/tree/main/pkg/vulnsrc) уязвимостей, обогащаемые базами Astra Linux, ALT Linux и РЕД ОС.

Также модуль производит анализ соответствия кластера kubernetes требованиями [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes/).

Модуль каждые 24 часа выполняет сканирование в пространствах имён, которые содержат метку `security-scanning.deckhouse.io/enabled=""`.
Если в кластере отсутствуют пространства имён с указанной меткой, сканируется пространство имён `default`.

Как только в кластере обнаруживается пространство имён с меткой `security-scanning.deckhouse.io/enabled=""`, сканирование пространства имён `default` прекращается.
Чтобы снова включить сканирование для пространства имён `default`, необходимо установить у него метку командой:

```shell
kubectl label namespace default security-scanning.deckhouse.io/enabled=""
```

С результаты сканирования можно ознакомиться в:
Grafana Dashboard:
  -  `Security/Trivy Image Vulnerability Overview` - сводка по найденным уязвимостям в ресурсах кластера
  -  `Security/CIS Kubernetes Benchmark` - информация о соответствии кластером требованиям CIS Kubernetes Benchmark
В ресурсах кластера:
  - Отчеты о безопасности кластера:
    - [`ClusterComplianceReport`](trivy-cr.html#clustercompliancereport)
    - [`RbacAssessmentReport`](trivy-cr.html#rbacassessmentreport)
  - Отчеты о безопасности ресурсов кластера:
    - [`VulnerabilityReport`](trivy-cr.html#vulnerabilityreport)
    - [`SbomReport`](trivy-cr.html#sbomreport)
    - [`ConfigAuditReport`](trivy-cr.html#configauditreport)
    - [`ExposedSecretReport`](trivy-cr.html#exposedsecretreport)
