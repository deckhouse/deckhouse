## Hooks

## ingress-nginx

### update_ingress_address.go

Fetches the provider-managed `Service` for each controller, copies the current load balancer address into the corresponding `IngressNginxController` status, and mirrors the address list to the reload helper ingresses.

