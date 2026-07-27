/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

const (
	gatewayAPICRDComplianceRetryInterval = time.Minute
	gatewayAPIBundleVersionAnnotation    = "gateway.networking.k8s.io/bundle-version"
)

//go:embed crds_gateway_api/*.yaml
var gatewayAPICRDManifests embed.FS

type bundledGatewayAPICRD struct {
	manifestPath               string
	crd                        *apiextensionsv1.CustomResourceDefinition
	minimumBundleVersion       string
	minimumServedVersion       string
	requiredExactServedVersion string
}

// requiredGatewayAPIEndpoints lists API endpoints used directly by this
// controller. Unlike the CRDs installed for the wider Gateway API ecosystem,
// these must serve the exact version against which the controller is compiled.
var requiredGatewayAPIEndpoints = map[string]string{
	"gateways." + gatewayv1.GroupName: gatewayv1.GroupVersion.Version,
}

func WaitForGatewayAPICRDCompliance(ctx context.Context) error {
	bundledCRDs, err := loadBundledGatewayAPICRDs()
	if err != nil {
		return err
	}
	klog.V(4).InfoS("Loaded bundled Gateway API CRDs for preflight", "count", len(bundledCRDs))

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("get kubeconfig for Gateway API CRD preflight: %w", err)
	}

	clientset, err := apiextensionsclientset.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create apiextensions client for Gateway API CRD preflight: %w", err)
	}

	for {
		klog.V(4).InfoS("Checking Gateway API CRD compliance", "count", len(bundledCRDs))
		retryMessage, err := ensureGatewayAPICRDComplianceOnce(ctx, clientset, bundledCRDs)
		if err == nil && retryMessage == "" {
			klog.V(1).InfoS(
				"Gateway API CRD preflight completed successfully",
				"count", len(bundledCRDs),
			)
			return nil
		}

		if retryMessage != "" {
			klog.Warning(retryMessage)
		}
		if err != nil {
			klog.Warningf(
				"Gateway API CRD preflight failed, retrying in %s: %v",
				gatewayAPICRDComplianceRetryInterval,
				err,
			)
		}

		timer := time.NewTimer(gatewayAPICRDComplianceRetryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func ensureGatewayAPICRDComplianceOnce(ctx context.Context, clientset apiextensionsclientset.Interface, bundledCRDs []bundledGatewayAPICRD) (string, error) {
	clusterCRDs := make(map[string]*apiextensionsv1.CustomResourceDefinition, len(bundledCRDs))
	for _, bundledCRD := range bundledCRDs {
		klog.V(4).InfoS(
			"Checking Gateway API CRD",
			"name", bundledCRD.crd.Name,
			"minimumBundleVersion", bundledCRD.minimumBundleVersion,
			"minimumServedVersion", bundledCRD.minimumServedVersion,
		)
		clusterCRD, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Get(
			ctx,
			bundledCRD.crd.Name,
			metav1.GetOptions{},
		)
		if apierrors.IsNotFound(err) {
			klog.V(1).InfoS(
				"Gateway API CRD is missing from cluster and will be created",
				"name", bundledCRD.crd.Name,
				"minimumBundleVersion", bundledCRD.minimumBundleVersion,
				"minimumServedVersion", bundledCRD.minimumServedVersion,
			)
			continue
		}
		if err != nil {
			return "", fmt.Errorf("get cluster CRD %q: %w", bundledCRD.crd.Name, err)
		}

		clusterCRDs[bundledCRD.crd.Name] = clusterCRD
	}

	toCreate, mismatches := evaluateGatewayAPICRDState(bundledCRDs, clusterCRDs)
	if len(mismatches) > 0 {
		return fmt.Sprintf(
			"incompatible Gateway API CRDs detected, controller startup is blocked and will retry in %s: %s",
			gatewayAPICRDComplianceRetryInterval,
			strings.Join(mismatches, "; "),
		), nil
	}

	for _, bundledCRD := range toCreate {
		klog.V(4).InfoS(
			"Creating bundled Gateway API CRD",
			"name", bundledCRD.crd.Name,
			"minimumBundleVersion", bundledCRD.minimumBundleVersion,
		)

		_, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Create(
			ctx,
			bundledCRD.crd.DeepCopy(),
			metav1.CreateOptions{},
		)
		if err == nil {
			klog.V(1).InfoS(
				"Created bundled Gateway API CRD",
				"name", bundledCRD.crd.Name,
				"minimumBundleVersion", bundledCRD.minimumBundleVersion,
			)

			continue
		}
		if apierrors.IsAlreadyExists(err) {
			// Another actor created the CRD after the read above. Do not assume it
			// is compatible; make the next pass fetch and validate it.
			return fmt.Sprintf(
				"Gateway API CRD %q was created concurrently, controller startup is blocked until it is validated on the next retry in %s",
				bundledCRD.crd.Name,
				gatewayAPICRDComplianceRetryInterval,
			), nil
		}

		return "", fmt.Errorf("create bundled Gateway API CRD %q: %w", bundledCRD.crd.Name, err)
	}

	return "", nil
}

func evaluateGatewayAPICRDState(bundledCRDs []bundledGatewayAPICRD, clusterCRDs map[string]*apiextensionsv1.CustomResourceDefinition) ([]bundledGatewayAPICRD, []string) {
	var (
		toCreate   []bundledGatewayAPICRD
		mismatches []string
	)

	for _, bundledCRD := range bundledCRDs {
		clusterCRD, exists := clusterCRDs[bundledCRD.crd.Name]
		if !exists {
			toCreate = append(toCreate, bundledCRD)
			continue
		}

		clusterBundleVersion := clusterCRD.Annotations[gatewayAPIBundleVersionAnnotation]
		if bundledCRD.requiredExactServedVersion != "" && !servesVersion(clusterCRD, bundledCRD.requiredExactServedVersion) {
			mismatches = append(mismatches, fmt.Sprintf(
				"%s (required API version %s is not served)",
				clusterCRD.Name,
				bundledCRD.requiredExactServedVersion,
			))
			continue
		}
		if !servesVersionOrHigher(clusterCRD, bundledCRD.minimumServedVersion) {
			mismatches = append(mismatches, fmt.Sprintf(
				"%s (minimum API version %s or higher is not served)",
				clusterCRD.Name,
				bundledCRD.minimumServedVersion,
			))
			continue
		}

		if semver.IsValid(clusterBundleVersion) {
			if semver.Compare(clusterBundleVersion, bundledCRD.minimumBundleVersion) < 0 {
				klog.V(4).InfoS(
					"Gateway API CRD bundle version is older than bundled manifest",
					"name", clusterCRD.Name,
					"clusterBundleVersion", clusterBundleVersion,
					"minimumBundleVersion", bundledCRD.minimumBundleVersion,
				)
				mismatches = append(mismatches, fmt.Sprintf(
					"%s (cluster bundle=%s, minimum bundle=%s)",
					clusterCRD.Name,
					clusterBundleVersion,
					bundledCRD.minimumBundleVersion,
				))
				continue
			}

			klog.V(4).InfoS(
				"Gateway API CRD bundle version is compatible",
				"name", clusterCRD.Name,
				"clusterBundleVersion", clusterBundleVersion,
				"minimumBundleVersion", bundledCRD.minimumBundleVersion,
			)
			continue
		}

		// The bundle-version annotation is metadata added by upstream release
		// bundles, not part of the CRD's API contract. User- or vendor-supplied
		// compatible CRDs may omit it, so accept the CRD based on the served API
		// version validated above.
		klog.Warningf(
			"Gateway API CRD %q has missing or invalid %q annotation %q; accepting it because API version %q or higher is served",
			clusterCRD.Name,
			gatewayAPIBundleVersionAnnotation,
			clusterBundleVersion,
			bundledCRD.minimumServedVersion,
		)
	}

	return toCreate, mismatches
}

func loadBundledGatewayAPICRDs() ([]bundledGatewayAPICRD, error) {
	manifestPaths, err := fs.Glob(gatewayAPICRDManifests, "crds_gateway_api/*.yaml")
	if err != nil {
		return nil, fmt.Errorf("list bundled Gateway API CRD manifests: %w", err)
	}
	slices.Sort(manifestPaths)

	bundledCRDs := make([]bundledGatewayAPICRD, 0, len(manifestPaths))
	for _, manifestPath := range manifestPaths {
		rawManifest, err := gatewayAPICRDManifests.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read bundled Gateway API CRD manifest %q: %w", manifestPath, err)
		}

		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.Unmarshal(rawManifest, crd); err != nil {
			return nil, fmt.Errorf("decode bundled Gateway API CRD manifest %q: %w", manifestPath, err)
		}

		bundleVersion, err := bundleVersionForCRD(crd)
		if err != nil {
			return nil, fmt.Errorf("determine bundle version for bundled Gateway API CRD manifest %q: %w", manifestPath, err)
		}
		minimumServedVersion, err := minimumServedVersionForCRD(crd)
		if err != nil {
			return nil, fmt.Errorf("determine minimum served version for bundled Gateway API CRD manifest %q: %w", manifestPath, err)
		}

		bundledCRDs = append(bundledCRDs, bundledGatewayAPICRD{
			manifestPath:               manifestPath,
			crd:                        crd,
			minimumBundleVersion:       bundleVersion,
			minimumServedVersion:       minimumServedVersion,
			requiredExactServedVersion: requiredGatewayAPIEndpoints[crd.Name],
		})
	}

	if len(bundledCRDs) == 0 {
		return nil, fmt.Errorf("no bundled Gateway API CRD manifests found")
	}

	return bundledCRDs, nil
}

func bundleVersionForCRD(crd *apiextensionsv1.CustomResourceDefinition) (string, error) {
	bundleVersion := crd.Annotations[gatewayAPIBundleVersionAnnotation]
	if bundleVersion == "" {
		return "", fmt.Errorf("CRD %q does not have the %q annotation", crd.Name, gatewayAPIBundleVersionAnnotation)
	}
	if !semver.IsValid(bundleVersion) {
		return "", fmt.Errorf("CRD %q has invalid bundle version %q in the %q annotation", crd.Name, bundleVersion, gatewayAPIBundleVersionAnnotation)
	}

	return bundleVersion, nil
}

// minimumServedVersionForCRD returns the minimum API version a compatible
// cluster CRD must serve. The storage version is used because it is unique per
// CRD and makes the choice deterministic and independent of the ordering of
// spec.versions. The bundled manifests always mark their storage version as
// served; this is verified here so an invalid manifest fails fast at load time.
func minimumServedVersionForCRD(crd *apiextensionsv1.CustomResourceDefinition) (string, error) {
	storageVersion, err := storageVersionForCRD(crd)
	if err != nil {
		return "", err
	}
	if !servesVersion(crd, storageVersion) {
		return "", fmt.Errorf("CRD %q does not serve its storage version %q", crd.Name, storageVersion)
	}

	return storageVersion, nil
}

func servesVersion(crd *apiextensionsv1.CustomResourceDefinition, requiredVersion string) bool {
	for _, version := range crd.Spec.Versions {
		if version.Name == requiredVersion && version.Served {
			return true
		}
	}

	return false
}

// servesVersionOrHigher reports whether the CRD serves an API version whose
// Kubernetes version priority is equal to or higher than minimumVersion. This
// is intentionally a compatibility heuristic, not a schema compatibility
// guarantee: newer or more stable API versions are accepted without requiring
// an exact version match.
func servesVersionOrHigher(crd *apiextensionsv1.CustomResourceDefinition, minimumVersion string) bool {
	for _, version := range crd.Spec.Versions {
		if version.Served && k8sversion.CompareKubeAwareVersionStrings(version.Name, minimumVersion) >= 0 {
			return true
		}
	}

	return false
}

func storageVersionForCRD(crd *apiextensionsv1.CustomResourceDefinition) (string, error) {
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			return version.Name, nil
		}
	}

	return "", fmt.Errorf("CRD %q does not declare a storage version", crd.Name)
}
