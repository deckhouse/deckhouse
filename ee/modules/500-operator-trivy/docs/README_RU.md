---
title: "Модуль operator-trivy"
description: Периодическое сканирование на уязвимости в кластере Deckhouse Platform Certified Security Edition.
---

Модуль позволяет запускать регулярную проверку пользовательских образов в runtime на известные CVE, включая уязвимости Astra Linux, Redos и ALT Linux. Базируется на проекте Trivy уязвимостей, обогащаемые базами Astra Linux, ALT Linux и РЕД ОС.

Также модуль производит анализ соответствия кластера kubernetes требованиями CIS Kubernetes Benchmark.

Модуль выполняет сканирование в пространствах имён, которые содержат метку `security-scanning.deckhouse.io/enabled=""`.
Если в кластере отсутствуют пространства имён с указанной меткой, сканируется пространство имён `default`.

Как только в кластере обнаруживается пространство имён с меткой `security-scanning.deckhouse.io/enabled=""`, сканирование пространства имён `default` прекращается.
Чтобы снова включить сканирование для пространства имён `default`, необходимо установить у него метку командой:

```shell
d8 k label namespace default security-scanning.deckhouse.io/enabled=""
```

## Условия запуска сканирования

Сканирование запускается:

- автоматически каждые 24 часа,
- при запуске компонентов с новыми образами контейнеров в пространствах имен, для которых включено сканирование (в частности, при появлении новых объектов).

## Механизм сканирования уязвимостей

Сканер уязвимостей является центральным компонентом Trivy: он выявляет известные уязвимости на основе специализированных баз данных. Модуль `operator-trivy` сканирует пакеты и компоненты операционной системы (исполняемые файлы, библиотеки) и зависимости приложений (библиотеки, модули и прочие зависимости).

Оператор автоматически запускает сканирование при обнаружении новых или изменённых рабочих нагрузок в неймспейсах, для которых включено сканирование.

### Пакеты операционной системы

Сканер автоматически определяет дистрибутив ОС и применяет соответствующие данные из баз уязвимостей, предоставляемых DKP.

Используются следующие источники данных:

- Банк данных угроз безопасности информации ФСТЭК России (БДУ ФСТЭК);
- AlmaLinux Errata;
- Alpine SecDB;
- ALTRepo Errata OVAL;
- Amazon Linux Security Advisories;
- Arch Linux Security Tracker;
- Debian Security Tracker;
- GitHub Security Advisory Database;
- National Vulnerability Database (NVD);
- Oracle OVAL;
- Photon Security Advisory;
- Red Hat OVAL;
- RED SOFT OVAL;
- Rocky Linux UpdateInfo;
- SUSE Security CVRF;
- Ubuntu CVE Tracker;
- Wolfi SecDB.



## Где просматривать результаты сканирования

В Grafana:

- `Security/Trivy Image Vulnerability Overview` — сводный обзор уязвимостей в образах и ресурсах кластера.
- `Security/CIS Kubernetes Benchmark` — результаты проверки соответствия кластера требованиям CIS Kubernetes Benchmark.

В ресурсах кластера:

- Отчеты о безопасности кластера:
  - [`ClusterComplianceReport`](cr.html#clustercompliancereport)
  - [`RbacAssessmentReport`](cr.html#rbacassessmentreport)
- Отчеты о безопасности ресурсов кластера:
  - [`VulnerabilityReport`](cr.html#vulnerabilityreport) — уязвимости в образах контейнеров;
  - [`SbomReport`](cr.html#sbomreport) — состав ПО в образах (SBOM);
  - [`ConfigAuditReport`](cr.html#configauditreport) — ошибки конфигурации Kubernetes-объектов;
  - [`ExposedSecretReport`](cr.html#exposedsecretreport) — утечки секретов в контейнерах.
