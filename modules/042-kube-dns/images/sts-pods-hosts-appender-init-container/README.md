== Motivativation

/etc/hosts file in pods is being rendered by kubelet, but unfortunately, it doesn't support clusterDomainAliases. It leads to poorly rendered /etc/hosts for pods with `spec.subdomain`. It contains fqdn with clusterDomain but doesn't contain fqdns with clusterDomainAliases. Some services (like rabbitmq) need to resolve pod's fqdn before it becomes Ready and /etc/hosts is the only way to do this.
