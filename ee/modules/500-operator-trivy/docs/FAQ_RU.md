---
title: "Модуль operator-trivy: FAQ"
description: Как в модуле operator-trivy Deckhouse посмотреть ресурсы, которые не прошли CIS compliance-проверки.
---
{% raw %}

## Просмотр ресурсов, которые не прошли CIS compliance-проверки

```bash
d8 k get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq '.status.detailReport.results | map(select(.checks | map(.success) | all | not))'
```

## Просмотр ресурсов, которые не прошли конкретную CIS compliance-проверку

По `id`:

```bash
check_id="5.7.3"
d8 k get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq --arg check_id "$check_id" '.status.detailReport.results | map(select(.id == $check_id))'
```

По описанию:

```bash
check_desc="Apply Security Context to Your Pods and Containers"
d8 k get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq --arg check_desc "$check_desc" '.status.detailReport.results | map(select(.description == $check_desc))'
```

{% endraw %}

## Ручной перезапуск сканирования ресурса

Модуль выполняет повторное сканирование ресурсов каждые 24 часа согласно следующему алгоритму:

1. В пространстве имён c каждым просканированным ресурсом создаётся объект `VulnerabilityReport`.
1. В этом объекте присутствует аннотация `trivy-operator.aquasecurity.github.io/report-ttl`, которая указывает время жизни отчёта (по умолчанию - `24h`).
1. По истечении этого времени объект удаляется, что вызывает повторное сканирование ресурса.

Принудительно запустить повторное сканирование ресурса можно одним из двух способов:

- Перезапишите аннотацию `trivy-operator.aquasecurity.github.io/report-ttl`, указав короткое время жизни отчёта.
- Удалите объект `VulnerabilityReport` из пространства имён, где находится просканированный ресурс.

Пример команды для перезаписи аннотации `trivy-operator.aquasecurity.github.io/report-ttl`:

```bash
d8 k annotate VulnerabilityReport -n <namespace> <reportName>  trivy-operator.aquasecurity.github.io/report-ttl=1s --overwrite
```

## Кто имеет доступ к результатам сканирования

Доступ к результатам сканирования (в том числе возможность просматривать [ресурсы с результатами](cr.html)) предоставляется пользователям, обладающим следующими [ролями доступа](../user-authz/#экспериментальная-ролевая-модель):

- `d8:manage:networking:viewer` или выше;
- `d8:manage:permission:module:operator-trivy:view`.
  
## Как ограничить список сканируемых ресурсов в пространстве имён

В текущей версии функциональности ограничения перечня ресурсов для сканирования в пространстве имён не предусмотрено.  
Оператор сканирует **все ресурсы**, находящиеся в пространстве имён, помеченном меткой `security-scanning.deckhouse.io/enabled=""`.

## Как просмотреть отчёт по своему приложению

Для просмотра результатов сканирования вашего приложения воспользуйтесь Grafana-дашбордом `Security / Trivy Image Vulnerability Overview`.  
Вы можете отфильтровать результаты по нужному пространству имён и ресурсу.

Также вы можете напрямую просматривать [ресурсы](cr.html) с результатами сканирования, которые создаются для каждого сканируемого объекта.  
Подробности о структуре их имён и местоположении доступны в [документации](cr.html).
