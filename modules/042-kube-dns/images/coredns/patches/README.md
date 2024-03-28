# Patches

## support-alpha-tolerate-unready-endpoints-annotation.patch

CoreDNS [relies](https://github.com/coredns/coredns/blob/5534625c75a0ae1f84e1f715eaddb257de4166eb/plugin/kubernetes/object/endpoint.go#L182-L184) on Ready status of an endpoint in an EndpointSlice. Make sure that coredns respects the deprecated `service.alpha.kubernetes.io/tolerate-unready-endpoints` annotation.

Upstream [PR](https://github.com/coredns/coredns/pull/5491).

## Go mod

To create this patch run commands:

```shell
go get google.golang.org/grpc@v1.57.1
go mod tidy
git diff
```
