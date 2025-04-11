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

## How to manually restart resource scanning, how to understand when a resource will be rescanned?

The module rescans resources every 24 hours, according to the following algorithm:

A `VulnerabilityReport` object is created in the namespace with each scanned resource.  
This object contains the annotation `trivy-operator.aquasecurity.github.io/report-ttl`, which specifies the report lifetime (default is `24h`).  
After this time, the operator deletes the object, which triggers a rescan of the resource.  

To force a rescan of the resource, you need to overwrite the annotation `trivy-operator.aquasecurity.github.io/report-ttl`, specifying a short period of time.  
It is also possible to delete the `VulnerabilityReport` object.

Example of annotation command:
```bash
kubectl annotate VulnerabilityReport -n <namespace> <reportName> trivy-operator.aquasecurity.github.io/report-ttl=1s --overwrite
```

