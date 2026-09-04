# Patches

## 001-istio-gomod_gosum.patch

Fix Istio CVE vulnerabilities

## 001-kiali-gomod_gosum.patch

Fix Kiali CVE vulnerabilities

## 002-istio-multicluster-regenerate-token-timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 002-kiali-logout.patch

Enable Logout in Kiali for header auth (DexAuthenticator). The tab that clicks Logout calls `/logout?rd=<app-origin>/` once; other tabs receive a `localStorage` event and only dispatch `sessionExpired` locally (no second sign_out, no reload) to avoid oauth2-proxy CSRF races.

## 003-istio-implement-sidecar-to-waypoint-routing.patch

Add Istio Pilot support for routing eligible Service-addressed traffic from sidecar workloads through service waypoints. For HBONE-enabled sidecars, CDS, LDS, RDS, and EDS generation honors Service waypoint bindings, uses cluster-local waypoint endpoints, and triggers the required xDS updates when bindings or waypoint workloads change. Direct pod-IP, headless-Service, and workload-waypoint traffic is outside this interoperability path. Native ambient multicluster lookups preserve each cluster's waypoint binding. Controlled by `ENABLE_SIDECAR_WAYPOINT_ROUTING`, which is enabled by default.
