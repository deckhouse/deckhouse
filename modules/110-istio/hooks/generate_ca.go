/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const defaultCASecretNamespace = "d8-istio"

// Value paths for the resolved ("internal") CA material persisted by this hook
// and consumed by the cacerts Secret and webhook caBundle templates.
const (
	internalCACertPath   = "istio.internal.ca.cert"
	internalCAKeyPath    = "istio.internal.ca.key"
	internalCAChainPath  = "istio.internal.ca.chain"
	internalCARootPath   = "istio.internal.ca.root"
	internalCASourcePath = "istio.internal.ca.source"
)

// Source markers recorded in `istio.internal.ca.source`.
const (
	caSourceInline     = "inline"
	caSourceCacerts    = "cacerts"
	caSourceSelfSigned = "selfSigned"
	// caSourceSecretRefPrefix is followed by "<namespace>/<name>"
	// of the referenced Secret.
	caSourceSecretRefPrefix = "secretRef:"
)

// secretRefSource builds the `istio.internal.ca.source` marker for a
// referenced Secret.
func secretRefSource(namespace, name string) string {
	return caSourceSecretRefPrefix + namespace + "/" + name
}

// isExternalSource reports whether a source marker denotes operator-supplied
// CA material (inline `istio.ca.*` or `ca.secretRef`) as opposed to
// module-owned material (`selfSigned` or an out-of-band `cacerts` Secret).
func isExternalSource(source string) bool {
	return source == caSourceInline || strings.HasPrefix(source, caSourceSecretRefPrefix)
}

// Value paths for the user-supplied inline CA and secretRef configuration.
const (
	caCertPath               = "istio.ca.cert"
	caKeyPath                = "istio.ca.key"
	caChainPath              = "istio.ca.chain"
	caRootPath               = "istio.ca.root"
	caSecretRefNamePath      = "istio.ca.secretRef.name"
	caSecretRefNamespacePath = "istio.ca.secretRef.namespace"
)

// Data keys recognized inside a referenced Secret:
// cert-manager `kubernetes.io/tls` layout and native Istio `cacerts` layout.
const (
	tlsCertKey   = "tls.crt"
	tlsKeyKey    = "tls.key"
	tlsCACertKey = "ca.crt"

	cacertsCertKey  = "ca-cert.pem"
	cacertsKeyKey   = "ca-key.pem"
	cacertsChainKey = "cert-chain.pem"
	cacertsRootKey  = "root-cert.pem"
)

// caSourceAnnotation mirrors `istio.internal.ca.source` onto the rendered
// `d8-istio/cacerts` Secret (see templates/control-plane/secrets.yaml). Since
// `istio.internal.ca.*` is volatile (lost on a Deckhouse restart), this
// annotation lets the secretRef last-good fallback recognize the durable
// Secret across restarts. Safe because istiod treats cacerts as a read-only
// plugged CA and only ever writes the `istio-generated` data key (which the
// module never emits), so a metadata annotation cannot conflict.
const caSourceAnnotation = "istio.deckhouse.io/ca-source"

// caSnapshot is the `secret_ca` snapshot payload: the CA material read from
// the `d8-istio/cacerts` Secret plus its provenance annotation (empty if the
// Secret was created out-of-band).
type caSnapshot struct {
	CA     lib.IstioCA `json:"ca"`
	Source string      `json:"source"`
}

const caMetricsGroup = "generate_ca"

// invalidCAMetric fires (value 1) whenever the hook publishes CA material to
// istiod that failed validation. This only ever happens on the log-only paths
// (a live/out-of-band `cacerts` Secret, the internal-reuse fallback, or a
// secretRef last-good fallback) where hard-blocking would regress an
// already-working mesh; the config-sourced paths hard-block instead and never
// reach this metric. The `source` label carries the provenance marker so the
// alert can point at the offending material.
const invalidCAMetric = "d8_istio_ca_material_invalid"

// secretRefUnresolvedMetric fires (value 1) whenever a configured
// `ca.secretRef` cannot be resolved on a run but the hook keeps serving the
// last-good CA instead of hard-blocking (transient source failure after a
// successful first resolution). It surfaces a silently-degraded rotation that
// would otherwise only appear in logs. The `source` label is the current
// secretRef marker.
const secretRefUnresolvedMetric = "d8_istio_ca_secretref_unresolved" // gitleaks:allow

// setInvalidCAMetric records that invalid CA material is being published as-is.
func setInvalidCAMetric(input *go_hook.HookInput, source string) {
	input.MetricsCollector.Set(invalidCAMetric, 1, map[string]string{"source": source}, metrics.WithGroup(caMetricsGroup))
}

// setSecretRefUnresolvedMetric records that a configured secretRef could not be
// re-resolved and the last-good CA is being kept.
func setSecretRefUnresolvedMetric(input *go_hook.HookInput, source string) {
	input.MetricsCollector.Set(secretRefUnresolvedMetric, 1, map[string]string{"source": source}, metrics.WithGroup(caMetricsGroup))
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	// When `ca.secretRef` is used, the referenced Secret is fetched with a
	// direct Get (not watched), so this schedule bounds how long a rotated or
	// newly-created source Secret can take to propagate to
	// `istio.internal.ca.*` (and, in turn, to the `d8-istio` Secret consumed
	// by istiod).
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "generate_ca",
			Crontab: "*/5 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret_ca",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyIstioCAFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cacerts"},
			},
			NamespaceSelector: lib.NsSelector(),
		},
	},
}, dependency.WithExternalDependencies(generateCA))

func applyIstioCAFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert selfsigned ca secret to secret: %v", err)
	}

	return caSnapshot{
		CA: lib.IstioCA{
			Cert:  string(secret.Data[cacertsCertKey]),
			Key:   string(secret.Data[cacertsKeyKey]),
			Chain: string(secret.Data[cacertsChainKey]),
			Root:  string(secret.Data[cacertsRootKey]),
		},
		Source: secret.Annotations[caSourceAnnotation],
	}, nil
}

func generateCA(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	var istioCA lib.IstioCA
	// caSource is persisted to `istio.internal.ca.source`;
	// every branch below must set it.
	var caSource string

	// Clear the CA-health metrics up front; each is re-set below only if its
	// condition still holds on this run, so a resolved problem stops alerting.
	input.MetricsCollector.Expire(caMetricsGroup)

	switch {
	case input.Values.Exists(caSecretRefNamePath):
		name := input.Values.Get(caSecretRefNamePath).String()
		namespace := input.Values.Get(caSecretRefNamespacePath).String()
		if namespace == "" {
			namespace = defaultCASecretNamespace
		}
		caSource = secretRefSource(namespace, name)

		ca, err := resolveCAFromSecretRef(ctx, dc, name, namespace)
		if err != nil {
			// If *this same* secretRef resolved successfully before, keep that
			// last-published CA rather than hard-blocking on a transient
			// source problem (briefly deleted/rotated Secret, API blip): it
			// reuses the exact material istiod already runs with, so a working
			// mesh is not regressed and no *different* CA is ever adopted. A
			// first resolution (or a different secretRef) has no matching
			// last-published CA and so hard-blocks.
			//
			// The reused material is validated at log level only, never
			// hard-blocked — the same policy the module-owned paths (live
			// `cacerts` snapshot and internal-reuse) use in the `default` case
			// below (see validateIstioCA). This is deliberately the *same*
			// last-published material istiod is already running with; only the
			// presence of `ca.secretRef` config distinguishes this path from
			// those. Hard-blocking here (while log-only there) would make an
			// already- working mesh's survival of a transient source failure
			// depend on whether config is still present, escalating a
			// transient/edge-case validation failure into a module-wide render
			// failure. The real safety gate is the wantSource marker match
			// inside lastGoodSecretRefCA, which ensures an *unrelated* CA is
			// never adopted; validateIstioCA is only a quality check.
			if prev, store := lastGoodSecretRefCA(input, caSource); prev != nil {
				setSecretRefUnresolvedMetric(input, caSource)
				checkCA := *prev
				normalizeIstioCA(&checkCA)
				if verr := validateIstioCA(checkCA); verr != nil {
					setInvalidCAMetric(input, caSource)
					input.Logger.Warn(
						"ca.secretRef re-resolution failed; keeping the last-published CA whose material is now invalid; publishing it as-is, but istiod may reject it",
						"error", err,
						"validationError", verr,
						"source", caSource,
						"store", store,
					)
				} else {
					input.Logger.Warn(
						"ca.secretRef re-resolution failed; keeping the last-good CA",
						"error", err,
						"source", caSource,
						"store", store,
					)
				}
				istioCA = *prev
				break
			}
			return err
		}

		istioCA = ca
	case input.Values.Exists(caCertPath):
		caSource = caSourceInline
		istioCA.Cert = input.Values.Get(caCertPath).String()
		istioCA.Key = input.Values.Get(caKeyPath).String()
		istioCA.Chain = input.Values.Get(caChainPath).String()
		istioCA.Root = input.Values.Get(caRootPath).String()
		// Inline CA is user-supplied config, so it is hard-blocked on invalid
		// material like the secretRef path (see validateIstioCA for the
		// hard-block vs. log-only policy).
		//
		// IMPLICIT self-anchoring guard (inline path): when no `root` is
		// supplied it defaults to `cert`, so the signing cert would be
		// published as its own root-cert.pem / webhook caBundle.  That
		// inference is only sound for a genuine self-signed root; a
		// non-self-signed intermediate has an unknown true issuer. This must
		// be checked HERE, before finalizeAndValidate normalizes an empty Root
		// to Cert (after which the implicit vs. explicit `root == cert` cases
		// are indistinguishable inside validateIstioCA, which deliberately
		// accepts an EXPLICIT `root` == `cert` even for a non-self-signed
		// intermediate). The mappers enforce the same rule before their own
		// defaulting (see mapCertManagerTLSSecretToCA / mapCacertsSecretToCA).
		if strings.TrimSpace(istioCA.Root) == "" {
			if err := verifyCertIsSelfSigned(istioCA.Cert); err != nil {
				return fmt.Errorf("the inline CA in 'istio.ca.*' is not valid; if 'cert' is an intermediate certificate that is not self-signed, add 'root' with the root trust anchor and no 'root' trust anchor is otherwise assumed: %w", err)
			}
		}
		if err := finalizeAndValidate(&istioCA); err != nil {
			return fmt.Errorf("the inline CA in 'istio.ca.*' is not valid: %w", err)
		}
	default:
		certs := input.Snapshots.Get("secret_ca")

		internalCA := currentInternalCA(input)

		switch {
		case len(certs) == 1:
			var snap caSnapshot
			if err := certs[0].UnmarshalTo(&snap); err != nil {
				return fmt.Errorf("cannot convert certificate to certificate authority: failed to unmarshal 'secret_ca' snapshot: %w", err)
			}
			istioCA = snap.CA
			// A live cacerts Secret is validated at log level only, never
			// hard-blocked (see validateIstioCA): hard-blocking it would
			// regress a working mesh on the schedule.  Validate a copy so the
			// log-only policy is preserved; the published material is
			// normalized once below.
			checkCA := snap.CA
			if err := finalizeAndValidate(&checkCA); err != nil {
				setInvalidCAMetric(input, caSourceCacerts)
				input.Logger.Warn("existing d8-istio/cacerts Secret has invalid CA material; publishing it as-is (only defaulting empty chain/root to the signing cert), but istiod may reject it", "error", err)
			}
			// Reaching the `default` branch means no `ca.*` config is present,
			// so a cacerts Secret found here is now module-owned material (the
			// operator either never configured `ca.*`, or removed it and is
			// intentionally keeping the last-published CA). Re-stamp the
			// provenance to `cacerts` regardless of any external
			// (`inline`/`secretRef`) marker the Secret still carries from when
			// it was resolved from config. This keeps the internal-reuse
			// fallback (the next case) active for this material, so a
			// subsequent transiently-empty snapshot cannot rotate this live CA
			// to a fresh self-signed one (which would break mTLS mesh-wide).
			// The external marker only has a consumer while `ca.secretRef` is
			// still configured (the first switch case reads it back via
			// lastGoodSecretRefCA); once the config is gone the marker is
			// dead, so dropping it is safe.
			caSource = caSourceCacerts
		case internalCA != nil:
			internalCASource := input.Values.Get(internalCASourcePath).String()

			// Reaching the `default` branch means no `ca.*` config is present.
			// Any external (`inline`/`secretRef`) marker still sitting in
			// `istio.internal.ca.source` is now dead: its only consumer is the
			// `ca.secretRef` switch case (via lastGoodSecretRefCA), which is
			// not reached once the config is gone. Demote it to the
			// module-owned `cacerts` marker so this reuse branch treats the
			// last-published material as module-owned.
			//
			// This closes a rotation race: right after `ca.*` is removed, a
			// scheduled run can fire before the transitional beforeHelm run
			// re-stamps the durable cacerts Secret. On that run the marker is
			// still external, and if the cacerts snapshot is *also*
			// transiently empty (informer hiccup / brief delete-recreate) this
			// reuse branch used to be skipped, generating a fresh self-signed
			// CA and rotating the live mesh root — breaking mTLS mesh-wide.
			// Demoting the marker here makes the reuse branch fire regardless
			// of run ordering, so a transiently-empty snapshot never rotates a
			// live CA. The intended revert-to-self-signed path (remove `ca.*`
			// AND delete the cacerts Secret, then a fresh process start wipes
			// the volatile internal values) still reaches the generate case
			// because there is then neither a snapshot nor a persisted
			// internal CA.
			if isExternalSource(internalCASource) {
				internalCASource = caSourceCacerts
			}

			// No cacerts Secret in the snapshot, but a module-owned CA
			// (selfSigned, an out-of-band cacerts Secret, or a last-published
			// external CA whose now-dead marker was demoted to `cacerts`
			// above) was persisted to `istio.internal.ca.*` on a previous run.
			// Reuse it (preserving its source marker) instead of generating a
			// fresh one: on the schedule, a transiently empty snapshot must
			// never rotate a live CA (which would break mTLS mesh-wide).
			//
			// The reused material is validated at log level only, never
			// hard-blocked — the same policy the live `cacerts` snapshot uses
			// in the `len(certs) == 1` case above (see validateIstioCA).  This
			// is deliberately the *same* module-owned material as that path (a
			// transiently-empty snapshot is the only difference), so the two
			// must react identically: hard-blocking here would let an informer
			// hiccup escalate a prior warning into a module-wide render
			// failure, and would regress a mesh istiod may already be running
			// with.
			caSource = internalCASource
			istioCA = *internalCA
			// Validate a copy so the log-only policy is preserved; the
			// published material is normalized once below (see the trailing
			// finalizeAndValidate/normalize step).
			checkCA := istioCA
			if err := finalizeAndValidate(&checkCA); err != nil {
				setInvalidCAMetric(input, internalCASource)
				input.Logger.Warn("reusing the previously persisted CA from 'istio.internal.ca.*' whose material is invalid; publishing it as-is, but istiod may reject it", "source", internalCASource, "error", err)
			}
		default:
			selfSignedCA, err := certificate.GenerateCA(
				input.Logger,
				"d8-istio",
				certificate.WithGroups("d8-istio"),
				certificate.WithKeyRequest(&csr.KeyRequest{
					A: "rsa",
					S: 2048,
				}),
			)
			if err != nil {
				return err
			}
			caSource = caSourceSelfSigned
			istioCA.Cert = selfSignedCA.Cert
			istioCA.Key = selfSignedCA.Key
		}
	}

	// Ensure Chain/Root are always populated (defaulting to Cert): istiod
	// requires a non-empty root-cert.pem in a plugged CA and publishes it as
	// the webhook caBundle. This is the sole normalization for the branches
	// that publish their raw material: self-signed generation (sets only
	// Cert/Key) and the log-only snapshot/internal-reuse branches (which
	// validate a copy but assign the un-normalized original to istioCA).
	// Idempotent for the inline/secretRef paths, which already ran
	// finalizeAndValidate on istioCA itself. (Validation is per-branch; see
	// validateIstioCA for which paths hard-block vs. only log.)
	normalizeIstioCA(&istioCA)

	input.Values.Set(internalCACertPath, istioCA.Cert)
	input.Values.Set(internalCAKeyPath, istioCA.Key)
	input.Values.Set(internalCAChainPath, istioCA.Chain)
	input.Values.Set(internalCARootPath, istioCA.Root)
	input.Values.Set(internalCASourcePath, caSource)

	return nil
}

// currentInternalCA returns the CA currently persisted in
// `istio.internal.ca.*`, or nil if none has been resolved yet (empty signing
// cert or key). It lets a scheduled run reuse the last-good CA when a fresh
// resolution fails, rather than regressing a working mesh.
func currentInternalCA(input *go_hook.HookInput) *lib.IstioCA {
	cert := input.Values.Get(internalCACertPath).String()
	key := input.Values.Get(internalCAKeyPath).String()
	if cert == "" || key == "" {
		return nil
	}
	return &lib.IstioCA{
		Cert:  cert,
		Key:   key,
		Chain: input.Values.Get(internalCAChainPath).String(),
		Root:  input.Values.Get(internalCARootPath).String(),
	}
}

// lastGoodSecretRefCA returns the last-published CA for the current secretRef
// (wantSource, e.g.  "secretRef:<ns>/<name>"), or nil if there is none. It
// consults two stores, each gated on a provenance marker equal to wantSource
// so an unrelated CA is never reused:
//  1. `istio.internal.ca.source` volatile module value.
//  2. `d8-istio/cacerts` Secret via the `secret_ca` snapshot (marker:
//     the `istio.deckhouse.io/ca-source` annotation) — the durable store
//     that survives a restart, which wipes store (1).
//
// The wantSource marker match is the safety gate: it ensures only *this*
// secretRef's own last-published material is ever reused, never an unrelated
// CA. The returned material is NOT filtered by validateIstioCA on purpose —
// that would make an already-working mesh's survival of a transient source
// failure depend on whether the material still validates, escalating a
// transient validation failure into a module-wide hard block. The caller
// validates the returned CA at log level only, matching the module-owned reuse
// paths (see validateIstioCA). Callers must normalizeIstioCA the result before
// use.
func lastGoodSecretRefCA(input *go_hook.HookInput, wantSource string) (*lib.IstioCA, string) {
	if prev := currentInternalCA(input); prev != nil &&
		input.Values.Get(internalCASourcePath).String() == wantSource {
		return prev, "istio.internal.ca.*"
	}

	for _, raw := range input.Snapshots.Get("secret_ca") {
		var snap caSnapshot
		if err := raw.UnmarshalTo(&snap); err != nil {
			continue
		}
		if snap.Source != wantSource {
			continue
		}
		ca := snap.CA
		return &ca, "d8-istio/cacerts Secret"
	}

	return nil, ""
}

// resolveCAFromSecretRef fetches the referenced Secret directly (no watch) and
// maps it into an IstioCA. It returns an error when the Secret is missing or
// malformed, so istiod is never reconfigured with a wrong CA.
func resolveCAFromSecretRef(ctx context.Context, dc dependency.Container, name, namespace string) (lib.IstioCA, error) {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return lib.IstioCA{}, err
	}

	secret, err := k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case k8serrors.IsNotFound(err):
		return lib.IstioCA{}, fmt.Errorf("ca.secretRef points to Secret %q in namespace %q, but it was not found", name, namespace)
	case err != nil:
		return lib.IstioCA{}, fmt.Errorf("cannot get referenced Secret %q in namespace %q: %w", name, namespace, err)
	}

	return mapReferencedSecretToCA(secret)
}

// mapReferencedSecretToCA converts a referenced Secret into an IstioCA,
// auto-detecting whether it is in cert-manager `tls` format or native Istio
// `cacerts` format (both are validated).
func mapReferencedSecretToCA(secret *v1.Secret) (lib.IstioCA, error) {
	switch {
	case len(secret.Data[tlsCertKey]) > 0:
		return mapCertManagerTLSSecretToCA(secret)
	case len(secret.Data[cacertsCertKey]) > 0:
		return mapCacertsSecretToCA(secret)
	default:
		return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q does not contain a recognized CA format (expected cert-manager 'tls.crt'/'tls.key' or Istio 'ca-cert.pem'/'ca-key.pem')", secret.Name, secret.Namespace)
	}
}

// mapCertManagerTLSSecretToCA maps a cert-manager `kubernetes.io/tls` Secret
// into an IstioCA.  The mapping follows cert-manager conventions (the `ca.crt`
// anchor and leaf-first `tls.crt` chain are cert-manager's, not part of the
// `kubernetes.io/tls` type): cert-manager puts the issued chain (leaf-first,
// root omitted) in `tls.crt` and the trust anchor in `ca.crt`. Since Istio's
// `ca-cert.pem` must be the single signing CA cert, we split `tls.crt` into
// its leaf (the signing CA) and the remaining intermediates.
func mapCertManagerTLSSecretToCA(secret *v1.Secret) (lib.IstioCA, error) {
	tlsCert := string(secret.Data[tlsCertKey])
	tlsKey := string(secret.Data[tlsKeyKey])
	caCert := string(secret.Data[tlsCACertKey])

	if tlsKey == "" {
		return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q is a cert-manager 'tls' secret but is missing the 'tls.key' key", secret.Name, secret.Namespace)
	}

	signingCert, intermediates, err := splitLeafAndIntermediates(tlsCert)
	if err != nil {
		return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q has a malformed 'tls.crt': %w", secret.Name, secret.Namespace, err)
	}

	ca := lib.IstioCA{
		Cert: signingCert,
		Key:  tlsKey,
	}

	switch {
	case caCert == "" && intermediates == "":
		// A single cert with no explicit root and no intermediates. The
		// signing cert would default to being its own root (IMPLICIT
		// self-anchoring). That is only sound for a genuine self-signed root,
		// so require it here — before defaulting, because validateIstioCA can
		// no longer tell an implicit default from an explicit `root == cert`
		// (it accepts the latter regardless of self-signage).
		if err := verifyCertIsSelfSigned(signingCert); err != nil {
			return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q has a 'tls.crt' that is not self-signed and no 'ca.crt' trust anchor; add 'ca.crt' with the root certificate so the module can publish the correct root-cert.pem: %w", secret.Name, secret.Namespace, err)
		}
		ca.Root = signingCert
		ca.Chain = joinPEM(signingCert, intermediates)
	case caCert == "":
		// `tls.crt` carries a chain (leaf + intermediates) but no `ca.crt`
		// trust anchor.  cert-manager deliberately omits the root from
		// `tls.crt`, so the true root is unknowable here. Refuse rather than
		// publish an intermediate as `root-cert.pem` (the webhook caBundle /
		// workload trust root), which would silently break trust.
		return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q has an intermediate 'tls.crt' chain but no 'ca.crt' trust anchor; add 'ca.crt' with the root certificate so the module can publish the correct root-cert.pem", secret.Name, secret.Namespace)
	case certsEqual(signingCert, caCert):
		// `ca.crt` is the same certificate as the signing cert (EXPLICIT
		// self-anchoring): typically the cert-manager `isCA: true`
		// selfsigned-issuer case, but also honored for a non-self-signed
		// intermediate the operator has deliberately chosen to anchor at
		// (validateIstioCA accepts `root == cert` regardless of self-signage).
		// Dedupe so the chain is not the same cert twice.
		ca.Root = caCert
		ca.Chain = joinPEM(signingCert, intermediates)
	default:
		// Intermediate CA: root is the separate `ca.crt`; chain is signing
		// cert + intermediates + root. Including the root in cert-chain.pem
		// matches upstream Istio's own cacerts tooling (tools/certs:
		// `cert-chain.pem = ca-cert.pem + root-cert.pem`) and istiod accepts
		// it.
		ca.Root = caCert
		ca.Chain = joinPEM(signingCert, intermediates, caCert)
	}

	// finalizeAndValidate is the authoritative gate for the remaining checks:
	// it normalizes (a safety net should a future switch branch forget to
	// populate Chain/Root) and verifies the signing cert/key and the anchoring
	// structure when root differs from cert. The implicit no-root self-signed
	// rule is handled above, before defaulting; an explicit `ca.crt` ==
	// `tls.crt` intermediate is accepted as an explicit self-anchor.
	if err := finalizeAndValidate(&ca); err != nil {
		return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q is not a valid CA: %w", secret.Name, secret.Namespace, err)
	}

	return ca, nil
}

// mapCacertsSecretToCA maps a native Istio `cacerts` Secret into an IstioCA.
func mapCacertsSecretToCA(secret *v1.Secret) (lib.IstioCA, error) {
	istioKey := string(secret.Data[cacertsKeyKey])
	if istioKey == "" {
		return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q is a 'cacerts' secret but is missing the 'ca-key.pem' key", secret.Name, secret.Namespace)
	}

	ca := lib.IstioCA{
		Cert:  string(secret.Data[cacertsCertKey]),
		Key:   istioKey,
		Chain: string(secret.Data[cacertsChainKey]),
		Root:  string(secret.Data[cacertsRootKey]),
	}

	// A cacerts Secret may omit root-cert.pem, in which case root would
	// default to the signing cert (IMPLICIT self-anchoring). Require
	// self-signed here — before defaulting — because after normalization
	// validateIstioCA can no longer distinguish this implicit default from an
	// explicit root-cert.pem == ca-cert.pem (which it accepts regardless of
	// self-signage). The message guides the operator to add 'root-cert.pem'.
	if strings.TrimSpace(ca.Root) == "" {
		if err := verifyCertIsSelfSigned(ca.Cert); err != nil {
			return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q is a 'cacerts' secret whose 'ca-cert.pem' is not self-signed and is missing 'root-cert.pem'; add 'root-cert.pem' with the root certificate so the module can publish the correct trust anchor: %w", secret.Name, secret.Namespace, err)
		}
	}

	// finalizeAndValidate normalizes (cacerts Secrets may omit
	// cert-chain.pem/root-cert.pem, and validateIstioCA needs a populated root
	// to perform its checks) and then validates.
	if err := finalizeAndValidate(&ca); err != nil {
		return lib.IstioCA{}, fmt.Errorf("referenced Secret %q in namespace %q is not a valid CA: %w", secret.Name, secret.Namespace, err)
	}

	return ca, nil
}

// normalizeIstioCA fills in the optional Chain/Root fields, defaulting each to
// the signing Cert when empty. This keeps the resolved material
// self-consistent across every resolution path (inline values, cacerts
// snapshot, referenced Secret, self-signed generation) so that
// istio.internal.ca.* — and the cacerts Secret and webhook caBundle rendered
// from it — never carry an empty root or chain.
func normalizeIstioCA(ca *lib.IstioCA) {
	if strings.TrimSpace(ca.Chain) == "" {
		ca.Chain = ca.Cert
	}
	if strings.TrimSpace(ca.Root) == "" {
		ca.Root = ca.Cert
	}
}

// validateIstioCA verifies that the resolved CA material is usable: the
// signing cert is a single parseable CA certificate that is permitted to sign
// certificates and whose key matches, its trust anchor is sound (either the
// signing cert is self-signed and acts as its own root, or a distinct root
// actually anchors it through the supplied chain). A root that does not anchor
// the signing cert — or a signing cert wrongly published as its own
// non-self-signed root — would otherwise be published as the webhook caBundle
// and silently break the mesh.
//
// The self-signed / root-anchoring check here is the single canonical guard
// for every resolution path: the inline, cert-manager `tls` and native
// `cacerts` mappers all normalize Root (defaulting it to Cert when absent) and
// then funnel through this function, so the rule is enforced in one place
// rather than duplicated per branch.
//
// Relationship to upstream istiod validation (as of istio 1.25.2,
// security/pkg/pki/util/keycertbundle.go `Verify`, reached for a plugged
// `cacerts` Secret via ca.NewVerifiedKeyCertBundleFromPem):
//
// istiod loads root-cert.pem into an x509 roots pool and cert-chain.pem into
// an intermediates pool, then calls cert.Verify — a set-based path check — and
// separately asserts the signing cert has basicConstraints CA:TRUE. It
// deliberately does NOT check keyCertSign key usage, does NOT require the root
// to be self-signed, and (being pools) is indifferent to duplicate or
// unrelated certs and to chain ordering. It DOES honor certificate validity
// periods (expiry).
//
// This function intentionally diverges on three points, because istiod is not
// the only consumer of the resolved material — `root` is also published as the
// webhook caBundle, verified by the Kubernetes API server, and `cert` mints
// workload certs verified by Envoy mesh-wide:
//
//   - keyCertSign is enforced (only when a KeyUsage extension is present, per
//     RFC 5280 §4.2.1.3), because a CA that declares it cannot sign certs would
//     mint workload certs that conformant verifiers reject at the mTLS
//     handshake — istiod merely defers that failure to runtime, whereas here we
//     fail closed at config time with an actionable message.
//   - a self-anchoring cert must be self-signed ONLY when `root` was not
//     supplied: the module would otherwise be guessing that an intermediate is
//     its own root and publishing an anchor whose true issuer is unknown. That
//     implicit-only guard is enforced by the callers before they default an
//     empty Root to Cert (the inline path in generateCA and the no-root
//     branches of the mappers), because normalization erases the
//     implicit/explicit distinction. When the operator explicitly sets `root`
//     == `cert`, that stated intent is honored even for a non-self-signed
//     intermediate, matching istiod's indifference to self-signage.
//   - validity periods are deliberately IGNORED (see verifyRootAnchorsCert): a
//     scheduled re-resolution must not hard-block an already-working mesh
//     purely because the clock advanced; expiry is istiod's concern at
//     runtime. This is the one axis where we are LESS strict than istiod, and
//     it is a conscious trade-off.
//
// Canonical policy for how callers react to a validation failure (referenced
// here from every resolution branch, rather than repeated at each):
//   - config-sourced paths (inline `istio.ca.*`, `ca.secretRef`) hard-block,
//     so malformed user input never reaches istiod.
//   - module-owned material is only logged, never hard-blocked: it is material
//     already in the cluster (or generated in-process) and hard-blocking it
//     would regress a working mesh on the periodic schedule. This covers both
//     the live `cacerts` snapshot and the internal-reuse fallback, which
//     handle the *same* material and therefore react identically.
//   - the self-signed generation path produces trusted in-process material and
//     is not validated.
func validateIstioCA(ca lib.IstioCA) error {
	blocks, err := strictPEMCertBlocks("signing certificate", ca.Cert)
	if err != nil {
		return fmt.Errorf("the signing certificate is not valid: %w", err)
	}
	if len(blocks) != 1 {
		return fmt.Errorf("the signing certificate must be a single certificate, not a chain")
	}

	cert, err := x509.ParseCertificate(blocks[0].Bytes)
	if err != nil {
		return fmt.Errorf("cannot parse the signing certificate: %w", err)
	}
	if !cert.IsCA {
		return fmt.Errorf("the signing certificate is not a CA certificate (basicConstraints CA:TRUE is required)")
	}
	// A CA that cannot sign certificates cannot mint workload certs; a chain
	// built on it would be rejected by conformant verifiers, breaking mTLS
	// after rollout. Enforce the keyCertSign key usage when a KeyUsage is
	// present at all. (KeyUsage == 0 means the extension is absent, in which
	// case RFC 5280 places no key-usage restriction, so we do not reject it.)
	// See the "Relationship to upstream istiod" note above for why this is
	// stricter than istiod.
	if cert.KeyUsage != 0 && cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return fmt.Errorf("the signing certificate is not allowed to sign certificates (keyUsage keyCertSign is required)")
	}

	if _, err := tls.X509KeyPair([]byte(ca.Cert), []byte(ca.Key)); err != nil {
		return fmt.Errorf("the signing certificate and key do not form a valid key pair: %w", err)
	}

	if _, err := strictPEMCertBlocks("certificate chain", ca.Chain); err != nil {
		return fmt.Errorf("the certificate chain is not valid: %w", err)
	}

	// The root, when present, is published verbatim as root-cert.pem and the
	// webhook caBundle, so it must contain only PEM CERTIFICATE blocks.
	// Validate it strictly here — before the certsEqual shortcut below — so
	// that trailing garbage or a stray private-key block cannot ride along
	// into the trust anchor. certsEqual/verifyCertIsSelfSigned only inspect
	// the first PEM block, so without this a Root of "<signing cert> +
	// <private key or junk>" would slip through the self-signed branch and be
	// published as a malformed trust anchor. verifyRootAnchorsCert (the
	// intermediate branch) re-runs this check on its own but harmlessly and
	// consistently.
	if strings.TrimSpace(ca.Root) != "" {
		if _, err := strictPEMCertBlocks("root certificate", ca.Root); err != nil {
			return fmt.Errorf("the root certificate is not valid: %w", err)
		}
	}

	if strings.TrimSpace(ca.Root) == "" || certsEqual(ca.Root, ca.Cert) {
		// Self-anchoring: the signing cert is (or is explicitly set as) its
		// own root, so it will be published as root-cert.pem / the webhook
		// caBundle. There is nothing left to verify structurally here — the
		// anchor IS the cert. We deliberately do NOT require self-signage:
		// anchoring at a non-self-signed intermediate is a valid (if unusual)
		// PKI choice that upstream istiod's x509.Verify and the K8s API server
		// both accept as-is.
		//
		// The one case that must still be rejected — IMPLICITLY defaulting an
		// absent `root` to a non-self-signed `cert`, where the module would be
		// *guessing* the anchor — is caught by the callers BEFORE they
		// normalize an empty Root to Cert (see the inline path in generateCA
		// and the no-root branches of mapCertManagerTLSSecretToCA /
		// mapCacertsSecretToCA). By the time material reaches this function it
		// has been normalized, so the empty-Root arm here is only a defensive
		// fallback for any future un-normalized caller and is treated the same
		// as an explicit self-anchor.
		return nil
	}

	// The root is distinct from the signing cert (an intermediate CA): verify
	// that the root actually anchors the signing cert through the provided
	// chain. Otherwise a valid but unrelated root would pass and be published
	// as the webhook trust anchor, breaking the mesh.
	if err := verifyRootAnchorsCert(cert, ca.Root, ca.Chain); err != nil {
		return err
	}

	return nil
}

// finalizeAndValidate normalizes the CA (defaulting empty Chain/Root to Cert)
// and then validates it. Every resolution path funnels through this single
// helper so that validation always runs on the same fully-populated material
// that will ultimately be published, and the normalize/validate ordering is
// never left to per-branch comment discipline. It mutates ca in place so the
// caller keeps the normalized result.
func finalizeAndValidate(ca *lib.IstioCA) error {
	normalizeIstioCA(ca)
	return validateIstioCA(*ca)
}

// verifyCertIsSelfSigned checks that certPEM is a self-signed certificate: it
// must both name itself as its own issuer (Subject DN == Issuer DN) AND carry
// a signature that verifies against its own public key. It is used to decide
// whether a single certificate with no `ca.crt` trust anchor may be published
// as its own root, so that a non-self-signed intermediate is never mistaken
// for a trust anchor. Like the other validation steps it deliberately ignores
// validity periods (expiry is istiod's concern).
//
// The Subject==Issuer check is required in addition to the signature check:
// CheckSignatureFrom only proves the cert was signed by *this* key, not that
// the cert names itself as its issuer. A cert self-signed with its own key but
// bearing a foreign Issuer DN would pass the signature check alone, yet it is
// not a real self-signed root — publishing it as root-cert.pem/caBundle would
// anchor mesh trust at a certificate whose stated issuer is never actually
// present.
func verifyCertIsSelfSigned(certPEM string) error {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("the certificate is not a valid PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("cannot parse the certificate: %w", err)
	}
	if !bytes.Equal(cert.RawSubject, cert.RawIssuer) {
		return fmt.Errorf("the certificate is not self-signed: its issuer does not match its subject")
	}
	if err := cert.CheckSignatureFrom(cert); err != nil {
		return fmt.Errorf("the certificate is not self-signed: %w", err)
	}
	return nil
}

// joinPEM concatenates non-empty PEM parts, ensuring exactly one newline
// between them.
func joinPEM(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, strings.TrimRight(p, "\r\n"))
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	return strings.Join(nonEmpty, "\n") + "\n"
}

// containsCert reports whether certs contains c
// (by parsed-certificate equality).
func containsCert(certs []*x509.Certificate, c *x509.Certificate) bool {
	for _, other := range certs {
		if other.Equal(c) {
			return true
		}
	}
	return false
}

// certsEqual reports whether two PEM strings decode to the same first X.509
// certificate, comparing the parsed certificates rather than the raw PEM bytes
// so that differences in whitespace, line wrapping, CRLF endings or
// surrounding comments do not cause a mismatch.  It returns false if either
// input cannot be parsed as a certificate.
func certsEqual(aPEM, bPEM string) bool {
	aBlocks := pemCertBlocks([]byte(aPEM))
	bBlocks := pemCertBlocks([]byte(bPEM))
	if len(aBlocks) == 0 || len(bBlocks) == 0 {
		return false
	}
	a, err := x509.ParseCertificate(aBlocks[0].Bytes)
	if err != nil {
		return false
	}
	b, err := x509.ParseCertificate(bBlocks[0].Bytes)
	if err != nil {
		return false
	}
	return a.Equal(b)
}

// pemCertBlocks returns the CERTIFICATE PEM blocks decoded from b.
func pemCertBlocks(b []byte) []*pem.Block {
	var blocks []*pem.Block
	for {
		block, rest := pem.Decode(b)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			blocks = append(blocks, block)
		}
		b = rest
	}
	return blocks
}

// strictPEMCertBlocks decodes all PEM blocks from pemText and requires the
// input to contain only valid CERTIFICATE PEM blocks (apart from whitespace
// between blocks). It returns an error for garbage, non-certificate PEM
// blocks, empty input, or a certificate block whose DER cannot be parsed. Use
// it for config-sourced material before publishing that material to cacerts.
func strictPEMCertBlocks(fieldName, pemText string) ([]*pem.Block, error) {
	b := []byte(pemText)
	var blocks []*pem.Block
	for {
		b = bytes.TrimSpace(b)
		if len(b) == 0 {
			break
		}
		block, rest := pem.Decode(b)
		if block == nil {
			return nil, fmt.Errorf("%s contains data that is not a PEM block", fieldName)
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("%s contains a PEM block of type %q, expected CERTIFICATE", fieldName, block.Type)
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return nil, fmt.Errorf("%s contains an unparsable certificate: %w", fieldName, err)
		}
		blocks = append(blocks, block)
		b = rest
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("%s has no PEM CERTIFICATE block", fieldName)
	}
	return blocks, nil
}

// splitLeafAndIntermediates decodes a PEM certificate chain and returns the
// first (leaf) certificate re-encoded on its own, plus the remaining
// certificates re-encoded as a chain. It errors if no certificate can be
// decoded.
func splitLeafAndIntermediates(chainPEM string) (string, string, error) {
	blocks, err := strictPEMCertBlocks("certificate chain", chainPEM)
	if err != nil {
		return "", "", err
	}

	leaf := string(pem.EncodeToMemory(blocks[0]))

	var sb strings.Builder
	for _, block := range blocks[1:] {
		sb.Write(pem.EncodeToMemory(block))
	}

	return leaf, sb.String(), nil
}

// maxChainDepth bounds the recursion in chainReachesRoot to guard against
// pathological or adversarial inputs (e.g. mutually cross-signed intermediates
// forming a cycle). Real CA chains are only a handful of levels deep, so this
// is comfortably above any legitimate chain.
const maxChainDepth = 10

// chainReachesRoot reports whether cert can reach one of roots via
// signature-verified hops through intermediates. Each hop requires both an
// issuer-DN match (cert.RawIssuer == candidate.RawSubject) and a valid
// signature (CheckSignatureFrom), so validity periods are never consulted.
//
// depth bounds the path length (see maxChainDepth), while visited marks the
// intermediates already expanded on the current search so each is expanded at
// most once.
func chainReachesRoot(cert *x509.Certificate, intermediates, roots []*x509.Certificate, depth int, visited map[*x509.Certificate]bool) bool {
	if depth > maxChainDepth {
		return false
	}
	for _, root := range roots {
		if bytes.Equal(cert.RawIssuer, root.RawSubject) && cert.CheckSignatureFrom(root) == nil {
			return true
		}
	}
	for _, mid := range intermediates {
		if visited[mid] {
			continue
		}
		if bytes.Equal(cert.RawIssuer, mid.RawSubject) && cert.CheckSignatureFrom(mid) == nil {
			visited[mid] = true
			if chainReachesRoot(mid, intermediates, roots, depth+1, visited) {
				return true
			}
		}
	}
	return false
}

// verifyRootAnchorsCert checks that rootPEM is the trust anchor of the signing
// cert, using any intermediate certificates found in chainPEM to bridge the
// gap. It asserts the trust *structure* (root -> [intermediates] -> signing
// cert) only, deliberately ignoring validity periods: a plugged CA whose chain
// is merely expired (or not yet valid) is not structurally "wrong", and
// coupling expiry into this check would let a scheduled re-resolution
// hard-block an otherwise- working mesh purely because of the clock. Expiry is
// istiod's concern at runtime; the other validation steps
// (parse/IsCA/key-pair) likewise ignore it.
//
// It walks the chain by signature (CheckSignatureFrom) rather than going
// through x509.Verify.  x509.Verify applies a single CurrentTime uniformly to
// every cert in the chain, so it cannot verify a chain whose validity windows
// do not overlap at all (e.g. an already-expired signing cert combined with a
// freshly-issued root): no single instant is inside every cert's window.  A
// pairwise signature walk has no such coupling and verifies the trust
// structure regardless of each cert's validity window, matching the intent
// above. CheckSignatureFrom also enforces that each issuer is itself a CA
// whose key usage permits certificate signing, so the walk asserts a
// well-formed CA chain, not merely a sequence of matching signatures.
func verifyRootAnchorsCert(cert *x509.Certificate, rootPEM, chainPEM string) error {
	rootBlocks, err := strictPEMCertBlocks("root certificate", rootPEM)
	if err != nil {
		return fmt.Errorf("the root certificate is not a valid PEM certificate: %w", err)
	}
	roots := make([]*x509.Certificate, 0, len(rootBlocks))
	for _, block := range rootBlocks {
		rootCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("cannot parse the root certificate: %w", err)
		}
		roots = append(roots, rootCert)
	}

	chainBlocks, err := strictPEMCertBlocks("certificate chain", chainPEM)
	if err != nil {
		return fmt.Errorf("the certificate chain is not valid: %w", err)
	}

	// Collect the intermediates from the chain: everything that is neither the
	// signing cert nor one of the roots, so an intermediate -> root path can
	// be built.
	var intermediates []*x509.Certificate
	for _, block := range chainBlocks {
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("cannot parse certificate in the certificate chain: %w", err)
		}
		if c.Equal(cert) || containsCert(roots, c) {
			continue
		}
		intermediates = append(intermediates, c)
	}

	if chainReachesRoot(cert, intermediates, roots, 0, make(map[*x509.Certificate]bool)) {
		return nil
	}
	return fmt.Errorf("the root certificate does not anchor the signing certificate")
}
