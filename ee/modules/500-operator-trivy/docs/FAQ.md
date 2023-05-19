---
title: "The operator-trivy module: FAQ"
---
{% raw %}

## How to view all resources that have not passed CIS compliance checks?

```bash
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson | jq '.status.detailReport.results | map(select(.checks | map(.success) | all | not))'
```

## How to view resources that have not passed specific CIS compliance checks

By check `id`:

```bash
check_id="5.7.3"
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson | jq --arg check_id "$check_id" '.status.detailReport.results | map(select(.id == $check_id))'
```

By check description:

```bash
check_desc="Apply Security Context to Your Pods and Containers"
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson | jq --arg check_desc "$check_desc" '.status.detailReport.results | map(select(.description == $check_desc))'
```

{% endraw %}
