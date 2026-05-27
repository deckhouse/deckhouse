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
