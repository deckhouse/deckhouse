---
title: "Модуль operator-trivy"
description: operator-trivy — модуль Deckhouse для периодического сканирования на уязвимости в кластере Kubernetes. 
---

Модуль позволяет запускать периодическое сканирование на уязвимости. Базируется на проекте [Trivy](https://github.com/aquasecurity/trivy).

Сканирование осуществляется каждые 24 часа в пространствах имен с лейблом `security-scanning.deckhouse.io/enabled=""`.

Пространство имён `default` сканируется независимо от наличия лейбла `security-scanning.deckhouse.io/enabled=""`.
