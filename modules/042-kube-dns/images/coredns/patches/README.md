# Patches

## support-alpha-tolerate-unready-endpoints-annotation.patch

CoreDNS [relies](https://github.com/coredns/coredns/blob/5534625c75a0ae1f84e1f715eaddb257de4166eb/plugin/kubernetes/object/endpoint.go#L182-L184) on Ready status of an endpoint in an EndpointSlice. Make sure that coredns respects the deprecated `service.alpha.kubernetes.io/tolerate-unready-endpoints` annotation.
