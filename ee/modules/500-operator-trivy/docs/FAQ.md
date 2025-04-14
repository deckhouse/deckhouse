---
title: "The operator-trivy module: FAQ"
description: How to view resources that have not passed CIS compliance checks in the operator-trivy Deckhouse module.
---
{% raw %}

## How to view all resources that have not passed CIS compliance checks?

```bash
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson | 
  jq '.status.detailReport.results | map(select(.checks | map(.success) | all | not))'
```

## How to view resources that have not passed a specific CIS compliance check?

By check `id`:

```bash
check_id="5.7.3"
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson | 
  jq --arg check_id "$check_id" '.status.detailReport.results | map(select(.id == $check_id))'
```

By check description:

```bash
check_desc="Apply Security Context to Your Pods and Containers"
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq --arg check_desc "$check_desc" '.status.detailReport.results | map(select(.description == $check_desc))'
```

{% endraw %}

## How to manually restart resource scanning and when is a resource rescanned?

The module rescans resources every 24 hours according to the following algorithm:

1. A `VulnerabilityReport` object is created in the namespace with each scanned resource.  
1. This object contains the annotation `trivy-operator.aquasecurity.github.io/report-ttl`, which specifies the report lifetime (the default is `24h`).  
1. After the lifetime expires, the object is deleted, which triggers a rescan of the resource.  

You can force a resource rescan in one of the following ways:

- Overwrite the annotation `trivy-operator.aquasecurity.github.io/report-ttl`, specifying a short report lifetime.  
- Delete the `VulnerabilityReport` object from the namespace where the scanned resource is located.

Example command for overwriting the annotation `trivy-operator.aquasecurity.github.io/report-ttl`:

```bash
kubectl annotate VulnerabilityReport -n <namespace> <reportName> trivy-operator.aquasecurity.github.io/report-ttl=1s --overwrite
```
