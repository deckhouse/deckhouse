---
title: "Модуль operator-trivy"
description: operator-trivy — модуль Deckhouse для периодического сканирования на уязвимости в кластере Kubernetes. 
---

Модуль позволяет запускать периодическое сканирование на уязвимости. Базируется на проекте [Trivy](https://github.com/aquasecurity/trivy).

Модуль каждые 24 часа выполняет сканирование в пространствах имён, которые содержат метку `security-scanning.deckhouse.io/enabled=""`.
Если в кластере отсутствуют пространства имён с указанной меткой, сканируется пространство имён `default`.

Как только в кластере обнаруживается пространство имён с меткой `security-scanning.deckhouse.io/enabled=""`, сканирование пространства имён `default` прекращается.

Чтобы снова включить сканирование для пространства имён `default`, необходимо установить метку командой:

```shell
kubectl label namespace default security-scanning.deckhouse.io/enabled=""
```
