---
title: "The operator-trivy module: FAQ"
description: How to view resources that have not passed CIS compliance checks in the operator-trivy Deckhouse module.
---
{% raw %}

## Viewing resources that failed CIS compliance checks

```bash
d8 k get clustercompliancereports.aquasecurity.github.io cis -ojson | 
  jq '.status.detailReport.results | map(select(.checks | map(.success) | all | not))'
```

## Viewing resources that have not passed a specific CIS compliance check

By check `id`:

```bash
check_id="5.7.3"
d8 k get clustercompliancereports.aquasecurity.github.io cis -ojson | 
  jq --arg check_id "$check_id" '.status.detailReport.results | map(select(.id == $check_id))'
```

By check description:

```bash
check_desc="Apply Security Context to Your Pods and Containers"
d8 k get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq --arg check_desc "$check_desc" '.status.detailReport.results | map(select(.description == $check_desc))'
```

{% endraw %}

## Manual rescan of a resource

The module rescans resources every 24 hours according to the following algorithm:

1. A `VulnerabilityReport` object is created in the namespace with each scanned resource.  
1. This object contains the annotation `trivy-operator.aquasecurity.github.io/report-ttl`, which specifies the report lifetime (the default is `24h`).  
1. After the lifetime expires, the object is deleted, which triggers a rescan of the resource.  

You can force a resource rescan in one of the following ways:

- Overwrite the annotation `trivy-operator.aquasecurity.github.io/report-ttl`, specifying a short report lifetime.  
- Delete the `VulnerabilityReport` object from the namespace where the scanned resource is located.

Example command for overwriting the annotation `trivy-operator.aquasecurity.github.io/report-ttl`:

```bash
d8 k annotate VulnerabilityReport -n <namespace> <reportName> trivy-operator.aquasecurity.github.io/report-ttl=1s --overwrite
```

## Who has access to scan results

Access to scan results (including the ability to view [resources with results](trivy-cr.html)) is granted to users with the following [access roles](../user-authz/#experimental-access-model):

- `d8:manage:networking:viewer` or higher;
- `d8:manage:permission:module:operator-trivy:view`.

## How to limit the list of resources scanned in a namespace

The current version does not support limiting the list of scanned resources within a namespace.  
The operator scans **all resources** located in any namespace labeled with `security-scanning.deckhouse.io/enabled=""`.

## How to view the scan report for your application

To view the scan results of your application, use the Grafana dashboard `Security / Trivy Image Vulnerability Overview`.  
You can filter the results by the desired namespace and resource.

You can also directly view the [resources](cr.html) that contain scan results created for each scanned object.  
Details about naming structure and resource location are available in the [documentation](cr.html).
