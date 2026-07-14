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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

const gatewayAPICRDComplianceRetryInterval = time.Minute

//go:embed crds_gateway_api/*.yaml
var gatewayAPICRDManifests embed.FS

type bundledGatewayAPICRD struct {
	manifestPath   string
	crd            *apiextensionsv1.CustomResourceDefinition
	storageVersion string
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
			"expectedStorageVersion", bundledCRD.storageVersion,
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
				"expectedStorageVersion", bundledCRD.storageVersion,
			)
			continue
		}
		if err != nil {
			return "", fmt.Errorf("get cluster CRD %q: %w", bundledCRD.crd.Name, err)
		}

		clusterCRDs[bundledCRD.crd.Name] = clusterCRD
	}

	toCreate, mismatches, err := evaluateGatewayAPICRDState(bundledCRDs, clusterCRDs)
	if err != nil {
		return "", err
	}
	if len(mismatches) > 0 {
		return fmt.Sprintf(
			"Gateway API CRD storage version mismatch detected, controller startup is blocked and will retry in %s: %s",
			gatewayAPICRDComplianceRetryInterval,
			strings.Join(mismatches, "; "),
		), nil
	}

	for _, crd := range toCreate {
		storageVersion, err := storageVersionForCRD(crd)
		if err != nil {
			return "", fmt.Errorf("determine storage version for bundled Gateway API CRD %q before create: %w", crd.Name, err)
		}

		klog.V(4).InfoS(
			"Creating bundled Gateway API CRD",
			"name", crd.Name,
			"storageVersion", storageVersion,
		)

		_, err = clientset.ApiextensionsV1().CustomResourceDefinitions().Create(
			ctx,
			crd,
			metav1.CreateOptions{},
		)
		if err == nil {
			klog.V(1).InfoS(
				"Created bundled Gateway API CRD",
				"name", crd.Name,
				"storageVersion", storageVersion,
			)

			continue
		}
		if apierrors.IsAlreadyExists(err) {
			klog.V(4).InfoS(
				"Gateway API CRD already exists while creating, continuing",
				"name", crd.Name,
			)

			continue
		}

		return "", fmt.Errorf("create bundled Gateway API CRD %q: %w", crd.Name, err)
	}

	return "", nil
}

func evaluateGatewayAPICRDState(bundledCRDs []bundledGatewayAPICRD, clusterCRDs map[string]*apiextensionsv1.CustomResourceDefinition) ([]*apiextensionsv1.CustomResourceDefinition, []string, error) {
	var (
		toCreate   []*apiextensionsv1.CustomResourceDefinition
		mismatches []string
	)

	for _, bundledCRD := range bundledCRDs {
		clusterCRD, exists := clusterCRDs[bundledCRD.crd.Name]
		if !exists {
			toCreate = append(toCreate, bundledCRD.crd.DeepCopy())
			continue
		}

		clusterStorageVersion, err := storageVersionForCRD(clusterCRD)
		if err != nil {
			return nil, nil, fmt.Errorf("determine storage version for cluster CRD %q: %w", clusterCRD.Name, err)
		}
		if clusterStorageVersion != bundledCRD.storageVersion {
			klog.V(4).InfoS(
				"Gateway API CRD storage version mismatch detected",
				"name", clusterCRD.Name,
				"clusterStorageVersion", clusterStorageVersion,
				"expectedStorageVersion", bundledCRD.storageVersion,
			)
			mismatches = append(mismatches, fmt.Sprintf(
				"%s (cluster=%s, bundled=%s)",
				clusterCRD.Name,
				clusterStorageVersion,
				bundledCRD.storageVersion,
			))
			continue
		}

		klog.V(4).InfoS(
			"Gateway API CRD storage version matches bundled manifest",
			"name", clusterCRD.Name,
			"storageVersion", clusterStorageVersion,
		)
	}

	return toCreate, mismatches, nil
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

		storageVersion, err := storageVersionForCRD(crd)
		if err != nil {
			return nil, fmt.Errorf("determine storage version for bundled Gateway API CRD manifest %q: %w", manifestPath, err)
		}

		bundledCRDs = append(bundledCRDs, bundledGatewayAPICRD{
			manifestPath:   manifestPath,
			crd:            crd,
			storageVersion: storageVersion,
		})
	}

	if len(bundledCRDs) == 0 {
		return nil, fmt.Errorf("no bundled Gateway API CRD manifests found")
	}

	return bundledCRDs, nil
}

func storageVersionForCRD(crd *apiextensionsv1.CustomResourceDefinition) (string, error) {
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			return version.Name, nil
		}
	}

	return "", fmt.Errorf("CRD %q does not declare a storage version", crd.Name)
}
