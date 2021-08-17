== Motivativation

`/etc/hosts` file in Pods is being rendered by kubelet, but unfortunately it doesn't support clusterDomainAliases. It leads to poorly rendered `/etc/hosts` for Pods with `spec.subdomain`. It contains fqdn with clusterDomain but doesn't contain fqdns with clusterDomainAliases. Some services (like RabbitMQ) need to resolve Pod's fqdn before it become Ready and `/etc/hosts` is the only way to do this.
