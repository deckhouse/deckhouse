---
title: Kubernetes conformance test results
permalink: en/reference/kubernetes-conformance.html
description: "CNCF Kubernetes conformance e2e test results for Kubernetes versions supported by Deckhouse Kubernetes Platform"
search: kubernetes conformance, cncf conformance, sonobuoy, e2e tests, junit
---

Deckhouse Kubernetes Platform is tested against the CNCF Kubernetes conformance suite. The tests are run with Sonobuoy in `certified-conformance` mode for each Kubernetes minor version listed below.

## Test results

{% assign conformance_results = site.data.kubernetes_conformance.results %}
{% if conformance_results.size > 0 %}
{% for result in conformance_results %}
- Kubernetes **{{ result.version }}** — [XML report]({{ site.canonical_url_prefix_documentation }}{{ result.xml_path }})
{% endfor %}
{% else %}
No conformance test results are available.
{% endif %}

{% assign conformance_readme = site.data.kubernetes_conformance.readme %}
{% if conformance_readme != empty %}
{{ conformance_readme | markdownify }}
{% else %}

## Running the tests

The conformance test instructions are currently unavailable.
{% endif %}
