 # How to test ingress controller build
 
Apply resources:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  annotationValidationEnabled: false
  chaosMonkey: false
  config:
    brotli-level: "6"
    brotli-types: text/xml image/svg+xml application/x-font-ttf image/vnd.microsoft.icon
      application/x-font-opentype application/json font/eot application/vnd.ms-fontobject
      application/javascript font/otf application/xml application/xhtml+xml text/javascript
      application/x-javascript text/plain application/x-font-truetype application/xml+rss
      image/x-icon font/opentype text/css image/x-win-bitmap
    enable-brotli: "true"
  controllerVersion: "1.9"
  disableHTTP2: false
  hsts: false
  ingressClass: nginx
  inlet: HostWithFailover
  maxReplicas: 1
  minReplicas: 1
  underscoresInHeaders: false
  validationEnabled: false
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/server-snippet: |
      location @toprod {
          proxy_pass "https://google.com";
          proxy_set_header Host "google.com";
          proxy_intercept_errors off;
          add_header x-source-s3 "prod" always;
      }
  name: nginx
  namespace: default
spec:
  rules:
    - host: nginx.test.com
      http:
        paths:
          - backend:
              service:
                name: nginx
                port:
                  number: 80
            path: /
            pathType: Exact
```

And check ingress-controller logs for errors.
