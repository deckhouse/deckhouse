---
title: "Модуль operator-trivy"
description: operator-trivy — модуль Deckhouse для периодического сканирования на уязвимости в кластере Kubernetes. 
---

Модуль позволяет запускать периодическое сканирование на уязвимости. Базируется на проекте [Trivy](https://github.com/aquasecurity/trivy).

Сканирование осуществляется каждые 24 часа в пространствах имён с лейблом `security-scanning.deckhouse.io/enabled=""`.

Если не найдено пространств имён с лейблом `security-scanning.deckhouse.io/enabled=""` производится сканирование пространства имён `default`. После обнаружения какого-либо пространства имён с лейблом `security-scanning.deckhouse.io/enabled=""`, сканирование для пространства имён `default` будет отключено и чтобы он сканировался, потребуется также установить лейбл `kubectl label namespace default security-scanning.deckhouse.io/enabled=""`
