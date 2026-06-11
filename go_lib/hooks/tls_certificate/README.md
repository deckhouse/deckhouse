# `tls_certificate` — internal TLS hook

`RegisterInternalTLSHook` produces a CA + leaf TLS certificate pair for an
in-cluster service (typically an admission/conversion webhook server). The
generated material is stored in the values tree (`FullValuesPathPrefix`) and
materialised by Helm into a `kubernetes.io/tls` Secret named `TLSSecretName`
inside `Namespace`.

The hook also re-issues the pair on its own when:

- the CA is missing or close to expiry;
- the leaf certificate has drifted from the configured CN / SAN set;
- the leaf was issued with `Subject == Issuer`
  (legacy depth-0 self-signed collision, rejected by strict validators);
- the leaf has no `ExtendedKeyUsage` extension
  (legacy "requestheader-client" pseudo-usage that cfssl silently drops).

See `internal_tls.go` for the implementation details and the rationale.

## Pod restart contract

When the hook re-issues a CA + leaf pair, Helm rewrites the underlying
`Secret`. Two things happen at the same moment:

1. The Pod template's projected/secret volume content is updated on disk by
   the kubelet (eventually — atomic-write symlinks).
2. The `ValidatingWebhookConfiguration` /
   `MutatingWebhookConfiguration` / `APIService` CABundle is refreshed
   with the **new** CA.

The webhook **server** inside the Pod, however, keeps the **old** TLS
cert + key in memory because most servers load them once at startup. The
client (kube-apiserver) presents the new CA when validating the TLS
handshake, the server still offers the old certificate → handshake fails
with `x509: certificate signed by unknown authority`, the affected
webhook (and every consumer of it) goes red until the Pod is restarted.

For every `RegisterInternalTLSHook` registration, **each Pod template
that mounts the generated Secret as a volume MUST carry a `checksum/*`
annotation in `spec.template.metadata.annotations` whose value depends on
the Secret content.** This forces a rolling restart of the Pod whenever
the Secret changes, which is the only deterministic way to get the
in-memory cert reloaded.

Reference (cert-manager webhook):

```yaml
spec:
  template:
    metadata:
      annotations:
        checksum/certificate: {{ include (print .Template.BasePath "/webhook/secret-tls.yaml") . | sha256sum }}
```

The `include` path must resolve to the Helm template that renders the
`Secret`. Mentioning the secret name in the annotation value (e.g. via a
literal substring or a `printf`) is also accepted; what matters is that
the resulting annotation value changes whenever the Secret content
changes.

For workloads composed via `helm_lib_*` helpers (such as
`helm_lib_capi_controller_manager_manifests`), pass the annotation through
`additionalPodAnnotations` in the helper's input dict.

### Alternative: hot-reload on disk

A Pod restart is the brute-force option. If the webhook server is built
with a watcher that reloads the cert/key from disk (e.g.
`k8s.io/apiserver/pkg/server/dynamiccertificates` or
`sigs.k8s.io/controller-runtime`'s `webhook.Server`), the checksum
annotation is no longer strictly required because the kubelet's atomic
secret update is picked up live. New webhooks SHOULD prefer this path;
the validation test below recognises an explicit opt-out marker for
those (see `tls_certificate_pod_consumers_test.go`).

## Validation

The contract is enforced by
`testing/hooks/validation/tls_certificate_pod_consumers_test.go`, which is
part of `make validate`. The test:

1. Scans every Go hook file under each edition's `modulesDir/*/hooks/`
   for `tls_certificate.RegisterInternalTLSHook(...)` calls and extracts
   the `(Namespace, TLSSecretName)` pair (resolving same-package and
   cross-package string constants).
2. For each pair, locates Pod template manifests (`Deployment`,
   `StatefulSet`, `DaemonSet`) inside the same module's `templates/` tree
   that mount the TLS Secret as a volume.
3. Asserts that each such Pod template carries at least one `checksum/*`
   annotation whose value either:
   - contains the literal secret name, or
   - `include`s a template file that declares a `kind: Secret` with
     `name: <TLSSecretName>`.

A failure prints the offending workload template and the rationale; the
fix is almost always a one-line annotation addition.

## TLS profiles

Once the Pod restart contract is in place, the next decision is what
**TLS handshake parameters** the in-Pod server offers. Deckhouse uses
four profiles, named A–D after the type of client the server faces.

### TL;DR

- **Category A** — admission webhooks, conversion webhooks, extension
  API servers. Client is deterministic and in-cluster
  (kube-apiserver). Pin `MinVersion: tls.VersionTLS13` and do **not**
  configure `CipherSuites`: Go fixes the suite list to three safe AEAD
  ciphers for TLS 1.3
  (`TLS_AES_128_GCM_SHA256`, `TLS_AES_256_GCM_SHA384`,
  `TLS_CHACHA20_POLY1305_SHA256`).
- **Category B** — `kube-rbac-proxy` sidecars, metrics endpoints, and
  any HTTPS surface whose clients are not strictly under our control
  (Prometheus is fine, but third-party scrapers may exist). Keep
  TLS 1.2 as a compatibility floor and limit `CipherSuites` to ECDHE +
  AEAD only — six entries in total. The forbidden classes are
  `TLS_RSA_WITH_*` (no PFS), `*_CBC_*` (Lucky13), `*_SHA` without
  `SHA256/SHA384` (SHA1), and the GOST suites
  (`TLS_GOSTR341112_*`, `KUZNYECHIK`, `MAGMA`) that have no
  implementation in upstream Go.
- **Category C** — `kube-apiserver` itself. Already configured by
  `candi/control-plane/kube-apiserver.yaml.tpl`; do not touch without
  an architecture decision.
- **Category D** — services exposed through ingress (`ingress-nginx`,
  `dex` via ingress, …). Out of scope for this document; follow
  Mozilla Intermediate / Modern as agreed with the security team.

### Helper

For convenience the same package exposes a tiny helper:

```go
import tlscert "github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"

cfg := &tls.Config{ /* … */ }
tlscert.ApplyServerCategoryA(cfg) // MinVersion = TLS 1.3, CipherSuites = nil
// or
tlscert.ApplyServerCategoryB(cfg) // MinVersion = TLS 1.2, CipherSuites = ECDHE + AEAD only
```

For controller-runtime managers use the `func(*tls.Config)` variant via
`tlscert.ServerOptionCategoryA()` / `ServerOptionCategoryB()`. Most
images that host their own webhook server live in a separate go.mod
module and inline a four-line literal instead of importing the helper;
see e.g. `modules/002-deckhouse/images/webhook-handler/operator/cmd/main.go`.

### Helm/werf side

For components that use the Kubernetes component-base CLI flags
(`--tls-cipher-suites`, `--tls-min-version`), pass the same allow-list:

- Category A:

  ```yaml
  - --tls-min-version=VersionTLS13
  # --tls-cipher-suites: do not set, the list is fixed for TLS 1.3.
  ```

- Category B:

  ```yaml
  - --tls-min-version=VersionTLS12
  - --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
  ```

### PR reviewer checklist

When a diff touches `--tls-cipher-suites`, `tlsCipherSuites:`,
`tls.Config{ CipherSuites: … }` or `MinVersion:`, verify:

- [ ] Category of the component is identified (A / B / C / D).
- [ ] For Category A — `MinVersion` is `tls.VersionTLS13` and
      `CipherSuites` is empty; why **not** TLS 1.3-only if it isn't?
- [ ] No `TLS_RSA_WITH_*` (RSA key exchange, no PFS).
- [ ] No `*_CBC_*` and no `*_SHA` without `SHA256/SHA384`.
- [ ] No `TLS_GOSTR341112_*` / `KUZNYECHIK` / `MAGMA`.
- [ ] If `--tls-min-version=VersionTLS12` — does the PR description
      justify why TLS 1.3 is not used?

### Validation tests

Two `make validate` tests guard this section:

- `TestValidationTLSCipherSuitesPolicy`
  (`testing/hooks/validation/tls_cipher_suites_policy_test.go`) walks
  every Helm template, werf manifest and kubelet template and fails on
  forbidden cipher names.
- `TestValidationCategoryAWebhookServersUseTLS13` (same file) verifies
  that every deckhouse-owned admission / conversion webhook Go server
  pins `MinVersion` to `tls.VersionTLS13`.

Both checks live under the `validation` build tag and are part of the
`make validate` target.

### Why no GOST suites

The Kubernetes component-base flag parser accepts the names
`TLS_GOSTR341112_256_WITH_KUZNYECHIK_MGM_L` and `_MGM_S`, but the
**implementation** of those suites does not exist in upstream Go. All
deckhouse images are built with stock upstream Go, so writing those
names into `--tls-cipher-suites` has zero runtime effect — the
handshake silently never selects them. The only outcome is a false
sense of GOST support during security audits. Real GOST-TLS, when ever
required, is delivered out-of-band: a GOST-aware proxy (`stunnel`,
OpenSSL+GOST-engine) in front of the component, or a Go fork with the
GOST patches, neither of which is configured here.
