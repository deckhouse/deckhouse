## Patches

### Move healthz endpoint to the telemetry server
By default healthz endpoint is served by the same server as for the metrics we are interested in. Thus, switching metrics' host to 127.0.0.1 (through rbac proxy) makes healthz endpoint unavailable for http_get liveness probes. This patch moves healthz from the metrics server to the telemetry server (serves internal metrics).
