# End-to-end tests of the cert-manager module

This directory holds the [Chainsaw](https://kyverno.github.io/chainsaw/) suites of the `cert-manager` module.
The e2e pipeline runs them on a fresh cluster under the runner module name `cert-manager`.

## Coverage

The functional suite in `tests/` checks the behaviour that users rely on.
Every case runs in the suite namespace `e2e-cm`, and object names carry the `e2e-cm-` prefix:

| Case | Priority | What is asserted |
|------|----------|------------------|
| `selfsigned-ready` | P0 | A Certificate issued by the ClusterIssuer `selfsigned` is Ready within 60 seconds. `tls.crt` has the requested DNS and IP SANs and is signed by `cluster-selfsigned-ca`. `ca.crt` equals `tls.crt` of `d8-cert-manager/selfsigned-ca-key-pair`. |
| `ca-issuer-chain` | P0 | `selfsigned-no-trust` issues a CA certificate, a namespaced Issuer `e2e-cm-ca` uses it and reaches `KeyPairVerified`, and the leaf it issues passes `openssl verify`. |
| `webhook-rejects-invalid` | P0 | `webhook.cert-manager.io` rejects a Certificate with `renewBefore` longer than `duration`, with `renewBefore` below 5 minutes, without any subject, with a malformed IP SAN, or with an unknown `issuerRef.kind`. It also rejects an Issuer without a type. The CRD schema rejects a Certificate without `issuerRef`. A valid Certificate and Issuer pass the same server-side dry run as a control. |
| `owner-ref` | P0 | With `spec.certificateOwnerRef: true` (patch `001-certificate_owner_ref`), the Secret is owned by the Certificate and is garbage-collected with it. With `false`, the Secret has no owner reference and is kept. |
| `renewal` | P1 | A Certificate with `duration: 1h` and `renewBefore: 59m` is renewed by the controller: `status.revision` grows and the serial number changes. A Certificate whose Secret is deleted is reissued. |
| `ingress-shim` | P1 | An Ingress annotated with `cert-manager.io/cluster-issuer: selfsigned` gets a Certificate named after its TLS `secretName`, owned by the Ingress and Ready. Deleting the Ingress removes the Certificate. |
| `cainjector` | P1 | A ValidatingWebhookConfiguration with `cert-manager.io/inject-ca-from` makes the hook `discover_cainjector_usage` enable cainjector. The `cainjector` Deployment becomes available and `caBundle` equals `ca.crt` of the Certificate Secret. |
| `metrics` | P2 | The controller metrics behind kube-rbac-proxy (`:9404/metrics`) expose `certmanager_certificate_ready_status` (condition `True`) and `certmanager_certificate_expiration_timestamp_seconds` for a test Certificate. |

The destructive suite in `tests-destructive/` checks that the module survives a restart:

| Case | What is asserted |
|------|------------------|
| `rollout-restart-under-load` | The `cert-manager`, `webhook` and `cainjector` Deployments are restarted one after another while a Certificate is created every 2 seconds and older ones are deleted. No `kubectl` call fails, so there are no webhook, x509 or connection errors, and every remaining Certificate becomes Ready. |

## Prerequisites

The suites expect the following environment:

- a cluster with Deckhouse and `cluster-admin` access through `KUBECONFIG`;
- `chainsaw` v0.2.15, `task`, `kubectl`, `jq`, `curl`, `openssl` and `base64` on `PATH`;
- network access from the host to pod IPs, which the `metrics` case needs.

Every test starts with the step template `tests/common/steps/ensure-cert-manager.yaml`.
The template calls `testing/e2e/lib/module-enabled.sh ensure cert-manager`.
When the module is not Ready, the helper enables it through its ModuleConfig with a merge patch.
It then waits for the Module to be Ready and for the Deckhouse `main` queue to drain.
The original `spec.enabled` value is kept in `/tmp/e2e-cm-state`.
The Taskfile restores it with `module-enabled.sh restore-all` after the suite, even when the suite fails.

The `cainjector` case and the destructive case leave cainjector enabled only while their
ValidatingWebhookConfiguration exists. Once it is deleted, the hook disables cainjector again,
unless something else in the cluster needs it.

## Suite namespace

The Taskfile creates the namespace `e2e-cm` before Chainsaw and passes it with `--namespace`.
All tests of a suite run in it, and a `defer` deletes it afterwards with `testing/e2e/lib/delete-namespace.sh`.
The namespace helpers are shared by the module suites and live in `testing/e2e/lib`.
The Taskfile finds them through `E2E_LIB_DIR`: the CI bundle sets it, in a checkout it defaults to that directory.
Chainsaw ephemeral namespaces are not used: multitenancy-manager adopts every new user namespace into a Project
and then refuses to delete the namespace, so the automatic cleanup of Chainsaw would fail.
The helper deletes the Project of that name when it exists and the namespace otherwise,
so the suites also work with multitenancy-manager disabled.

## Running the suites

Run the tasks from this directory on a master node:

```shell
task run                # functional suite, report cert-manager.xml
task run:destructive    # destructive suite, report cert-manager-destructive.xml
task run:cpu-starvation # CPU starvation scenarios from cpu-starvation/, on demand only
task lint               # lint the tests, the configuration and the scripts
```

The reports are written to `$E2E_REPORTS_DIR`, or to `./reports` when the variable is not set.

## Cases that are not automated

The following planned cases are not automated:

- **ACME HTTP-01 issuance.** It needs an ACME server inside the cluster, such as Pebble.
  The only workload image allowed in the e2e environment is `agnhost`, and Deckhouse ships no ACME server image.
  The ClusterIssuers `letsencrypt` and `letsencrypt-staging` need public DNS and inbound HTTP from the internet,
  which the e2e cluster does not have.
- **ACME DNS-01 issuance** through Cloudflare, Route53, DigitalOcean, Cloud DNS or Yandex Cloud DNS.
  It needs real DNS provider credentials.
- **Firing of the alerts** `CertmanagerCertificateExpiredSoon`, `CertmanagerCertificateExpired`,
  `CertmanagerCertificateOrderErrors` and `CertmanagerCertificateChallengePending`.
  Each of them has a `for` duration of 10 minutes to 1 hour, and the order and challenge alerts need ACME traffic.
  The `metrics` case checks the series these alerts are built on instead.
- **Gateway API and ListenerSet integration.** It depends on the Gateway API CRDs, which the cluster may not have.

## Assumptions

The cases rely on the following behaviour of the cluster:

- An Ingress with the IngressClass `e2e-cm-no-controller` is admitted. No controller serves this class,
  and ingress-shim handles an annotated Ingress regardless of its class.
- Test namespaces are not labelled `cert-manager.io/disable-validation`,
  so `webhook.cert-manager.io` validates them.
