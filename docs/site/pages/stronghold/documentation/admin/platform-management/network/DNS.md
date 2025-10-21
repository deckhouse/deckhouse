---
title: "DNS management"
permalink: en/stronghold/documentation/admin/platform-management/network/dns.html
---

To install CoreDNS components for DNS management, use the kube-dns module.

{% alert level="info" %}
kube-dns deletes resources previously installed via kubeadm, including Deployment, ConfigMap, and RBAC for CoreDNS.
{% endalert %}

To enable kube-dns with default settings, use the following ModuleConfig resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  enabled: true
EOF
```

For detailed description of kube-dns settings, refer to the [corresponding article](/modules/kube-dns/configuration.html).

## DNS configuration example

Example of the kube-dns module configuration using the ModuleConfig resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  enabled: true
  settings:
    # A list of IP addresses for recursive DNS servers that CoreDNS uses to resolve external domains
    # By default, it uses the list from /etc/resolv.conf
    upstreamNameservers:
      - 8.8.8.8
      - 8.8.4.4
    # A static list of hosts formatted similarly to /etc/hosts
    hosts:
      - domain: one.example.com
        ip: 192.168.0.1
      - domain: two.another.example.com
        ip: 10.10.0.128
    # A list of additional zones managed by CoreDNS
    stubZones:
      - zone: consul.local
        upstreamNameservers:
          - 10.150.0.1
    # A list of alternative cluster domains resolved along with global.discovery.clusterDomain
    clusterDomainAliases:
      - foo.bar
      - baz.qux
EOF
```

For detailed description of kube-dns settings, refer to the [corresponding article](/modules/kube-dns/configuration.html).

## Replace a cluster domain

To replace a cluster domain with minimal downtime, follow these steps:

1. Edit the settings of the control-plane manager module responsible for Deckhouse configuration.

    Make changes using the following template:

    ```yaml
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      name: control-plane-manager
    spec:
      version: 1
      enabled: true
      settings:
        apiserver:
          # A list of SANs certificate options for generating the API server certificate
          certSANs:
           - kubernetes.default.svc.<old clusterDomain>
           - kubernetes.default.svc.<new clusterDomain>
          serviceAccount:
            # A list of API audiences to include when creating ServiceAccount tokens
            additionalAPIAudiences:
            - https://kubernetes.default.svc.<old clusterDomain>
            - https://kubernetes.default.svc.<new clusterDomain>
            # A list of additional ServiceAccount API token issuers to add as they are created
            additionalAPIIssuers:
            - https://kubernetes.default.svc.<old clusterDomain>
            - https://kubernetes.default.svc.<new clusterDomain>
    ```

1. Add a list of alternative cluster domains to the kube-dns module configuration:

    ```yaml
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      name: kube-dns
    spec:
      version: 1
      enabled: true
      settings:
        clusterDomainAliases:
          - <old clusterDomain>
          - <new clusterDomain>
    ```

1. Wait for `kube-apiserver` to restart.
1. Replace `clusterDomain` with a new domain in `dhctl config edit cluster-configuration`.
