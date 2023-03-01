---
title: "Модуль operator-trivy"
---

Модуль позволяет запускать периодическое сканирование на уязвимости. Базируется на проекте [Trivy](https://github.com/aquasecurity/trivy).

Сканирование осуществляется каждые 24 часа в пространствах имен с лейблом `security-scanning.deckhouse.io/enabled`.
