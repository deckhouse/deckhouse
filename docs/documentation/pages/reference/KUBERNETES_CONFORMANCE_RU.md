---
title: Результаты Kubernetes conformance-тестов
permalink: ru/reference/kubernetes-conformance.html
description: "Результаты e2e-тестов CNCF Kubernetes conformance для версий Kubernetes, поддерживаемых Deckhouse Kubernetes Platform"
lang: ru
search: kubernetes conformance, cncf conformance, sonobuoy, e2e-тесты, junit, тесты на совместимость
---

Deckhouse Kubernetes Platform тестируется на соответствие требованиям CNCF Kubernetes conformance. Для каждой указанной ниже минорной версии Kubernetes тесты запускаются с помощью Sonobuoy в режиме `certified-conformance`.

## Результаты тестирования

{% assign conformance_results = site.data.kubernetes_conformance.results %}
{% if conformance_results.size > 0 %}
{% for result in conformance_results %}
- Kubernetes **{{ result.version }}** — [отчёт JUnit в формате XML]({{ site.canonical_url_prefix_documentation }}{{ result.xml_path }})
{% endfor %}
{% else %}
Результаты conformance-тестов пока недоступны.
{% endif %}

## Запуск тестов

{% alert level="info" %}
Перевод инструкции пока недоступен. Ознакомьтесь с [инструкцией на английском языке]({{ site.canonical_url_prefix_documentation }}/en/reference/kubernetes-conformance.html#cncf-kubernetes-conformance-sonobuoy).
{% endalert %}
