---
title: Проверка соответствия CIS Kubernetes Benchmark
description: Проверка, которую Deckhouse Platform автоматически выполняет в каждом кластере по рекомендациям CIS Kubernetes Benchmark. Результаты доступны в виде отчётов и на дашборде Security / CIS Kubernetes Benchmark.
lang: ru
---

Deckhouse Platform соответствует рекомендациям CIS Kubernetes Benchmark как на уровне отдельных компонентов, так и для платформы в целом. Доступ к отчётам, как и к результатам [сканирования на уязвимости](glossary.html#vulnerability-scanning), выдаётся через [ролевую модель](glossary.html#rbac) модуля [`operator-trivy`](/modules/operator-trivy/).

Подробнее в разделе [«Настройка сканирования контейнерных образов на уязвимости»](user/security/scanning.html).
